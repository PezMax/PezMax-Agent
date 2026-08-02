package agent

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"PezMax-Agent/internal/domain"
)

func (s *Service) GenerateMockExam(ctx context.Context, req domain.MockExamRequest) (domain.MockExamResponse, error) {
	req = normalizeMockExamRequest(req)
	sourceFiles, searchResults, err := s.loadMockExamSources(ctx, req)
	if err != nil {
		return domain.MockExamResponse{}, err
	}
	enrichMockExamRequestFromFiles(&req, sourceFiles)

	webSources, webErr := s.searchMockExamWebSources(ctx, req)
	analysis := analyzeMockExamSources(req, sourceFiles, webSources, webErr)
	if s.model != nil {
		aiExam, err := s.generateMockExamWithLLM(ctx, req, sourceFiles, webSources, analysis)
		if err == nil && len(aiExam.Questions) > 0 {
			aiExam.Intent = "mock_exam"
			aiExam.Subject = req.Subject
			aiExam.School = req.School
			aiExam.Year = req.Year
			aiExam.HasPastPapers = len(sourceFiles) > 0
			aiExam.PaperAnalysis = analysis
			aiExam.SourceFiles = sourceFiles
			aiExam.WebSources = webSources
			aiExam.RecommendedFiles = searchResults
			if aiExam.Summary == "" {
				aiExam.Summary = summarizeMockExam(req, len(sourceFiles), len(webSources))
			}
			return aiExam, nil
		}
	}

	questions := buildFallbackMockQuestions(req, sourceFiles, webSources)
	return domain.MockExamResponse{
		Intent:           "mock_exam",
		Subject:          req.Subject,
		School:           req.School,
		Year:             req.Year,
		HasPastPapers:    len(sourceFiles) > 0,
		PaperAnalysis:    analysis,
		SourceFiles:      sourceFiles,
		WebSources:       webSources,
		Questions:        questions,
		RecommendedFiles: searchResults,
		Suggestions:      buildMockExamSuggestions(req, sourceFiles, webSources, webErr),
		Summary:          summarizeMockExam(req, len(sourceFiles), len(webSources)),
	}, nil
}

func enrichMockExamRequestFromFiles(req *domain.MockExamRequest, files []domain.FileItem) {
	if len(files) == 0 {
		return
	}
	first := files[0]
	if req.Subject == "" {
		req.Subject = normalizeSubject(first.Subject)
	}
	if req.School == "" {
		req.School = first.School
	}
	if req.Year == 0 {
		req.Year = first.Year
	}
}

func normalizeMockExamRequest(req domain.MockExamRequest) domain.MockExamRequest {
	req.Subject = normalizeSubject(strings.TrimSpace(req.Subject))
	req.School = strings.TrimSpace(req.School)
	req.Goal = strings.TrimSpace(req.Goal)
	if req.Subject == "" {
		req.Subject = extractSubject(req.Goal)
	}
	if req.Year == 0 {
		req.Year = extractYear(req.Goal)
	}
	if req.QuestionCount <= 0 {
		req.QuestionCount = 8
	}
	if req.QuestionCount < 3 {
		req.QuestionCount = 3
	}
	if req.QuestionCount > 20 {
		req.QuestionCount = 20
	}
	req.Difficulty = strings.TrimSpace(req.Difficulty)
	if req.Difficulty == "" {
		req.Difficulty = "中等"
	}
	return req
}

func (s *Service) loadMockExamSources(ctx context.Context, req domain.MockExamRequest) ([]domain.FileItem, []domain.FileSearchResult, error) {
	files := []domain.FileItem{}
	seen := map[int64]bool{}
	for _, fileID := range req.FileIDs {
		if fileID <= 0 || seen[fileID] {
			continue
		}
		file, err := s.backend.GetFile(ctx, fileID)
		if err != nil {
			return nil, nil, err
		}
		if file != nil {
			files = append(files, *file)
			seen[file.FileID] = true
		}
	}

	searchReq := domain.FileSearchRequest{
		Query:    firstNonEmpty(req.Goal, req.Subject+" 期末 真题"),
		Keyword:  firstNonEmpty(req.Subject, req.Goal),
		School:   req.School,
		Subject:  req.Subject,
		Year:     req.Year,
		Type:     1,
		PageNum:  1,
		PageSize: maxInt(req.QuestionCount, 10),
	}
	search, err := s.SearchFiles(ctx, searchReq)
	if err != nil {
		return nil, nil, err
	}
	for _, item := range search.Items {
		if item.FileID > 0 && seen[item.FileID] {
			continue
		}
		files = append(files, item)
		if item.FileID > 0 {
			seen[item.FileID] = true
		}
	}
	return files, search.Results, nil
}

func (s *Service) searchMockExamWebSources(ctx context.Context, req domain.MockExamRequest) ([]domain.WebSearchResult, error) {
	if s.webSearch == nil {
		return nil, fmt.Errorf("web search is not configured")
	}
	query := strings.TrimSpace(fmt.Sprintf("%s %s 期末考试 真题 模拟题 题型", req.School, firstNonEmpty(req.Subject, req.Goal)))
	results, err := s.webSearch.Search(ctx, query, 6)
	if err != nil {
		return nil, err
	}
	out := make([]domain.WebSearchResult, 0, len(results))
	for idx, item := range results {
		snippet := item.Snippet
		if idx < 3 {
			if text, err := s.webSearch.FetchText(ctx, item.URL, 1800); err == nil && text != "" {
				snippet = strings.TrimSpace(snippet + "\n正文摘录：" + text)
			}
		}
		out = append(out, domain.WebSearchResult{
			Title:   item.Title,
			URL:     item.URL,
			Snippet: snippet,
			Source:  item.Source,
		})
	}
	return out, nil
}

func analyzeMockExamSources(req domain.MockExamRequest, files []domain.FileItem, webSources []domain.WebSearchResult, webErr error) string {
	subject := firstNonEmpty(req.Subject, "该科目")
	if len(files) == 0 {
		if len(webSources) == 0 {
			if webErr != nil {
				return fmt.Sprintf("平台没有找到%s可参考真题；网络搜索工具不可用：%s。", subject, webErr.Error())
			}
			return fmt.Sprintf("平台没有找到%s可参考真题，网络侧也暂未返回可用题型线索。", subject)
		}
		return fmt.Sprintf("平台没有找到%s可参考真题；模拟题将基于 %d 条网络题型/复习资源线索生成。", subject, len(webSources))
	}
	years := map[int]bool{}
	schools := map[string]bool{}
	for _, file := range files {
		if file.Year > 0 {
			years[file.Year] = true
		}
		if file.School != "" {
			schools[file.School] = true
		}
	}
	return fmt.Sprintf("平台找到 %d 份%s真题/期末资料，覆盖 %d 个年份、%d 所学校；模拟题将参考这些资料名称、科目、年份和题型分布，并结合 %d 条网络资源。", len(files), subject, len(years), len(schools), len(webSources))
}

func (s *Service) generateMockExamWithLLM(ctx context.Context, req domain.MockExamRequest, files []domain.FileItem, webSources []domain.WebSearchResult, analysis string) (domain.MockExamResponse, error) {
	prompt := fmt.Sprintf(`请根据 PezMax 平台真题资料和网络资源生成一套模拟题，只返回严格 JSON，不要解释。

科目：%s
学校：%s
年份：%d
难度：%s
题目数量：%d
用户目标：%s
真题资料分析：%s
平台真题资料：%s
网络题型资源：%s
科目知识主题：%s

要求：
1. 如果平台真题为空，summary 必须明确说明“平台暂未找到可参考真题”，但可以结合网络资源生成练习建议题。
2. 题目必须是原创模拟题，不能照抄真题原题；要体现真题题型、章节和难度。
3. questions 数量必须等于题目数量；每题包含 number,type,topic,difficulty,stem,options,answer,analysis,sourceBasis。
4. 题型混合使用选择题、填空题、计算题、简答题或综合题，按科目自然分布。
5. 答案和解析要可直接用于学生自测。`,
		req.Subject,
		req.School,
		req.Year,
		req.Difficulty,
		req.QuestionCount,
		req.Goal,
		analysis,
		mustJSON(compactStudyFiles(files, 12)),
		mustJSON(compactWebSources(webSources, 6)),
		mustJSON(subjectStudyTopics(req.Subject)),
	)
	msg, err := s.generate(ctx, mockExamSystemPrompt, prompt)
	if err != nil {
		return domain.MockExamResponse{}, err
	}
	var out domain.MockExamResponse
	if err := decodeJSONObject(msg.Content, &out); err != nil {
		return domain.MockExamResponse{}, err
	}
	return out, nil
}

func buildFallbackMockQuestions(req domain.MockExamRequest, files []domain.FileItem, webSources []domain.WebSearchResult) []domain.MockQuestion {
	topics := subjectStudyTopics(req.Subject)
	types := []string{"选择题", "填空题", "计算题", "简答题", "综合题"}
	questions := make([]domain.MockQuestion, 0, req.QuestionCount)
	for i := 0; i < req.QuestionCount; i++ {
		topic := topics[i%len(topics)]
		qType := types[i%len(types)]
		questions = append(questions, domain.MockQuestion{
			Number:      i + 1,
			Type:        qType,
			Topic:       topic.Name,
			Difficulty:  req.Difficulty,
			Stem:        fallbackQuestionStem(req.Subject, qType, topic),
			Options:     fallbackQuestionOptions(qType, topic),
			Answer:      fallbackQuestionAnswer(qType, topic),
			Analysis:    fmt.Sprintf("本题考查%s。复习时重点关注：%s", topic.Name, topic.Focus),
			SourceBasis: fallbackSourceBasis(files, webSources),
		})
	}
	return questions
}

func fallbackQuestionStem(subject, qType string, topic studyTopic) string {
	switch qType {
	case "选择题":
		return fmt.Sprintf("关于“%s”的常见考试判断，下列说法最符合课程要求的是哪一项？", topic.Name)
	case "填空题":
		return fmt.Sprintf("请写出“%s”中最核心的判定条件、公式或算法步骤。", topic.Name)
	case "计算题":
		return fmt.Sprintf("围绕“%s”设计一道中等难度计算/推导题，并完整写出关键步骤。", topic.Name)
	case "简答题":
		return fmt.Sprintf("简述“%s”的核心思想、适用条件和常见失分点。", topic.Name)
	default:
		return fmt.Sprintf("结合%s历年期末题型，完成一道关于“%s”的综合应用题。", firstNonEmpty(subject, "本课程"), topic.Name)
	}
}

func fallbackQuestionOptions(qType string, topic studyTopic) []string {
	if qType != "选择题" {
		return nil
	}
	return []string{
		"A. 只记结论即可，不需要关注适用条件",
		"B. 先判断适用条件，再选择对应方法并检查边界情况",
		"C. 所有题都可以直接套同一个公式",
		"D. 只要结果正确，过程条件可以省略",
	}
}

func fallbackQuestionAnswer(qType string, topic studyTopic) string {
	if qType == "选择题" {
		return "B"
	}
	return fmt.Sprintf("答案要点：%s；练习要求：%s", topic.Concepts, topic.ExamTarget)
}

func fallbackSourceBasis(files []domain.FileItem, webSources []domain.WebSearchResult) string {
	if len(files) > 0 {
		return fmt.Sprintf("参考平台真题资料：%s", files[0].Name)
	}
	if len(webSources) > 0 {
		return fmt.Sprintf("参考网络资源：%s", webSources[0].Title)
	}
	return "未找到平台真题或网络资源，基于科目知识结构生成。"
}

func buildMockExamSuggestions(req domain.MockExamRequest, files []domain.FileItem, webSources []domain.WebSearchResult, webErr error) []string {
	suggestions := []string{"建议先限时完成模拟题，再对照答案整理错题。"}
	if len(files) == 0 {
		suggestions = append(suggestions, "平台暂无可参考真题，生成结果更适合作为专项练习，不建议直接当作押题。")
	}
	if len(webSources) == 0 && webErr != nil {
		suggestions = append(suggestions, "配置网络搜索工具后，可以结合外部题型资源生成更贴近考试范围的模拟题。")
	}
	return suggestions
}

func summarizeMockExam(req domain.MockExamRequest, fileCount, webCount int) string {
	subject := firstNonEmpty(req.Subject, "该科目")
	if fileCount == 0 {
		return fmt.Sprintf("平台暂未找到%s可参考真题，已结合 %d 条网络资源线索生成 %d 道模拟练习题。", subject, webCount, req.QuestionCount)
	}
	return fmt.Sprintf("已参考 %d 份%s平台真题/资料和 %d 条网络资源，生成 %d 道模拟题。", fileCount, subject, webCount, req.QuestionCount)
}

func looksLikeMockExam(text string) bool {
	return containsAny(text, "模拟题", "出题", "生成题", "模拟卷", "仿真题", "押题")
}

func chatMockExamRequest(userID int64, text string) domain.MockExamRequest {
	return domain.MockExamRequest{
		UserID:        userID,
		Goal:          text,
		Subject:       extractSubject(text),
		Year:          extractYear(text),
		QuestionCount: extractQuestionCount(text),
	}
}

func extractQuestionCount(text string) int {
	re := regexp.MustCompile(`(\d+)\s*(道|题|个题目)`)
	match := re.FindStringSubmatch(text)
	if len(match) < 2 {
		return 0
	}
	value, _ := strconv.Atoi(match[1])
	return value
}
