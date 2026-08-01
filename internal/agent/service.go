package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"PezMax-Agent/internal/backend"
	"PezMax-Agent/internal/domain"
	pezllm "PezMax-Agent/internal/llm"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

type Service struct {
	model   pezllm.ChatModel
	backend backend.Client
}

func NewService(model pezllm.ChatModel, backend backend.Client) *Service {
	return &Service{model: model, backend: backend}
}

func (s *Service) SearchFiles(ctx context.Context, req domain.FileSearchRequest) (domain.FileSearchResponse, error) {
	if req.PageSize <= 0 {
		req.PageSize = 10
	}
	filters := s.extractSearchFilters(ctx, req)
	normalizeSearchFilters(&filters)
	items, err := s.backend.SearchFiles(ctx, filters)
	if err != nil {
		return domain.FileSearchResponse{}, err
	}

	return domain.FileSearchResponse{
		Intent:      "file_search",
		Filters:     filters,
		Items:       items,
		Results:     buildSearchResults(filters, items),
		Suggestions: buildSearchSuggestions(filters, items),
		Summary:     summarizeSearch(filters, items),
	}, nil
}

func (s *Service) RecommendFiles(ctx context.Context, req domain.FileRecommendRequest) (domain.FileRecommendResponse, error) {
	limit := req.Limit
	if limit <= 0 {
		limit = 8
	}
	if limit > 30 {
		limit = 30
	}

	var seed *domain.FileItem
	if req.FileID > 0 {
		file, err := s.backend.GetFile(ctx, req.FileID)
		if err != nil {
			return domain.FileRecommendResponse{}, err
		}
		seed = file
	}

	filters := recommendFilters(req, seed, limit)
	candidates, err := s.collectRecommendationCandidates(ctx, filters, seed, limit)
	if err != nil {
		return domain.FileRecommendResponse{}, err
	}

	results := buildRecommendationResults(filters, candidates, seed, limit)
	return domain.FileRecommendResponse{
		Intent:          "file_recommend",
		SeedFile:        seed,
		Filters:         filters,
		Recommendations: results,
		Suggestions:     buildRecommendationSuggestions(filters, results),
		Summary:         summarizeRecommendations(results),
	}, nil
}

func (s *Service) OrganizeFavorites(ctx context.Context, req domain.FavoriteOrganizeRequest) (domain.FavoriteOrganizeResponse, error) {
	if req.PageNum <= 0 {
		req.PageNum = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 100
	}
	items, err := s.backend.ListFavorites(ctx, req.UserID, req.PageNum, req.PageSize)
	if err != nil {
		return domain.FavoriteOrganizeResponse{}, err
	}

	groupBy := strings.TrimSpace(req.GroupBy)
	if groupBy == "" {
		groupBy = "subject"
	}
	groups := organizeFavoriteGroups(items, groupBy)

	return domain.FavoriteOrganizeResponse{
		Intent:      "favorites_organize",
		UserID:      req.UserID,
		Total:       len(items),
		Groups:      groups,
		Suggestions: buildFavoriteSuggestions(groups, items),
		Summary:     summarizeFavorites(groups, items),
	}, nil
}

func (s *Service) SuggestMetadata(ctx context.Context, req domain.MetadataSuggestRequest) (domain.MetadataSuggestion, error) {
	suggestion := heuristicMetadata(req)
	normalizeMetadataSuggestion(&suggestion)

	schools, _ := s.backend.SuggestSchools(ctx, firstNonEmpty(req.SchoolHint, suggestion.School), 5)
	subjects, _ := s.backend.SuggestSubjects(ctx, firstNonEmpty(req.SubjectHint, suggestion.Subject), 5)
	if suggestion.School == "" && len(schools) > 0 {
		suggestion.School = schools[0]
		suggestion.Reasons = append(suggestion.Reasons, "matched existing school suggestion")
	}
	if suggestion.Subject == "" && len(subjects) > 0 {
		suggestion.Subject = subjects[0]
		suggestion.Reasons = append(suggestion.Reasons, "matched existing subject suggestion")
	}

	if s.model == nil {
		normalizeMetadataSuggestion(&suggestion)
		return suggestion, nil
	}

	prompt := fmt.Sprintf(`请为试题下载平台补全文件元数据，只返回 JSON。
字段：fileName, fileSchool, fileSubject, fileYear, fileType, fileTypeName, remark, confidence, reasons。
fileType: 1=期末, 2=期中, 3=资料, 4=补考, 5=其他学校。
用户输入: %s
规则兜底: %s
已有学校候选: %s
已有科目候选: %s`, mustJSON(req), mustJSON(suggestion), mustJSON(schools), mustJSON(subjects))

	msg, err := s.generate(ctx, metadataSystemPrompt, prompt)
	if err != nil {
		return suggestion, nil
	}
	var enhanced domain.MetadataSuggestion
	if err := decodeJSONObject(msg.Content, &enhanced); err != nil {
		return suggestion, nil
	}
	mergeMetadata(&suggestion, enhanced)
	normalizeMetadataSuggestion(&suggestion)
	return suggestion, nil
}

func (s *Service) SuggestAudit(ctx context.Context, req domain.AuditSuggestRequest) (domain.AuditSuggestion, error) {
	suggestion := heuristicAudit(req.File)
	if s.model == nil {
		return suggestion, nil
	}

	prompt := fmt.Sprintf(`请作为试题下载平台审核辅助，只返回 JSON。
字段：suggestedAction(pass/reject/manual_review), riskLevel(low/medium/high), riskScore(0-100), reasons, reviewComment。
注意：你只能给审核建议，不能代替管理员做最终决定。
文件信息: %s
规则兜底: %s`, mustJSON(req.File), mustJSON(suggestion))

	msg, err := s.generate(ctx, auditSystemPrompt, prompt)
	if err != nil {
		return suggestion, nil
	}
	var enhanced domain.AuditSuggestion
	if err := decodeJSONObject(msg.Content, &enhanced); err != nil {
		return suggestion, nil
	}
	if enhanced.SuggestedAction != "" {
		return enhanced, nil
	}
	return suggestion, nil
}

func (s *Service) Chat(ctx context.Context, req domain.ChatRequest) (domain.ChatResponse, error) {
	if looksLikeSearch(req.Message) {
		search, err := s.SearchFiles(ctx, domain.FileSearchRequest{Query: req.Message})
		if err != nil {
			return domain.ChatResponse{}, err
		}
		return domain.ChatResponse{
			Intent: "file_search",
			Answer: search.Summary,
			Data:   search,
		}, nil
	}

	if s.model == nil {
		return domain.ChatResponse{
			Intent: "general",
			Answer: "智能体服务已启动，但当前未配置 DASHSCOPE_API_KEY。资料搜索、元数据补全和审核建议仍可使用规则兜底能力。",
		}, nil
	}

	msg, err := s.generate(ctx, chatSystemPrompt, req.Message)
	if err != nil {
		return domain.ChatResponse{}, err
	}
	return domain.ChatResponse{Intent: "general", Answer: msg.Content}, nil
}

func (s *Service) extractSearchFilters(ctx context.Context, req domain.FileSearchRequest) domain.FileSearchRequest {
	filters := heuristicSearch(req)
	if s.model == nil || strings.TrimSpace(req.Query) == "" {
		return filters
	}

	prompt := fmt.Sprintf(`从用户检索意图中抽取试题资料搜索条件，只返回 JSON。
字段：query, keyword, school, subject, year, type, pageNum, pageSize。
type: 1=期末, 2=期中, 3=资料, 4=补考, 5=其他学校。
用户输入: %s
规则兜底: %s`, req.Query, mustJSON(filters))

	msg, err := s.generate(ctx, searchSystemPrompt, prompt)
	if err != nil {
		return filters
	}
	var extracted domain.FileSearchRequest
	if err := decodeJSONObject(msg.Content, &extracted); err != nil {
		return filters
	}
	mergeSearch(&filters, extracted)
	return filters
}

func (s *Service) generate(ctx context.Context, systemPrompt, userPrompt string) (*schema.Message, error) {
	return s.model.Generate(ctx, []*schema.Message{
		schema.SystemMessage(systemPrompt),
		schema.UserMessage(userPrompt),
	}, model.WithTemperature(0.2))
}

const searchSystemPrompt = "你是 PezMax 试题下载平台的搜索参数抽取助手。你必须输出严格 JSON，不要解释。"
const metadataSystemPrompt = "你是 PezMax 试题下载平台的上传资料元数据补全助手。你必须输出严格 JSON，不要解释。"
const auditSystemPrompt = "你是 PezMax 试题下载平台的审核辅助助手。你必须输出严格 JSON，不要解释。"
const chatSystemPrompt = "你是 PezMax 试题下载平台助手，回答要简洁、准确，优先围绕找资料、上传资料和审核建议。"

func heuristicSearch(req domain.FileSearchRequest) domain.FileSearchRequest {
	out := req
	text := strings.TrimSpace(firstNonEmpty(req.Query, req.Keyword))
	if out.Keyword == "" {
		out.Keyword = text
	}
	if out.Year == 0 {
		out.Year = extractYear(text)
	}
	if out.Type == 0 {
		out.Type = extractFileType(text)
	}
	if out.Subject == "" {
		out.Subject = extractSubject(text)
	}
	if out.PageSize <= 0 {
		out.PageSize = 10
	}
	if out.PageNum <= 0 {
		out.PageNum = 1
	}
	return out
}

func heuristicMetadata(req domain.MetadataSuggestRequest) domain.MetadataSuggestion {
	name := firstNonEmpty(req.FileName, req.OriginalName)
	base := strings.TrimSuffix(filepath.Base(name), filepath.Ext(name))
	text := strings.Join([]string{name, req.SchoolHint, req.SubjectHint, req.RemarkHint}, " ")

	year := extractYear(text)
	fileType := extractFileType(text)
	subject := firstNonEmpty(req.SubjectHint, extractSubject(text))
	school := req.SchoolHint
	reasons := []string{}
	if year > 0 {
		reasons = append(reasons, "extracted year from filename or hint")
	}
	if fileType > 0 {
		reasons = append(reasons, "extracted file type from filename or hint")
	}
	if subject != "" {
		reasons = append(reasons, "extracted subject from filename or hint")
	}

	confidence := 0.45
	if year > 0 {
		confidence += 0.15
	}
	if fileType > 0 {
		confidence += 0.15
	}
	if subject != "" {
		confidence += 0.15
	}
	if school != "" {
		confidence += 0.10
	}
	if confidence > 0.95 {
		confidence = 0.95
	}

	return domain.MetadataSuggestion{
		FileName:   firstNonEmpty(req.FileName, base),
		School:     school,
		Subject:    subject,
		Year:       year,
		Type:       fileType,
		TypeName:   fileTypeName(fileType),
		Remark:     req.RemarkHint,
		Confidence: confidence,
		Reasons:    reasons,
	}
}

func heuristicAudit(file domain.FileItem) domain.AuditSuggestion {
	score := 0
	reasons := []string{}
	if file.Name == "" {
		score += 30
		reasons = append(reasons, "missing file name")
	}
	if file.URL == "" {
		score += 35
		reasons = append(reasons, "missing file URL")
	}
	if file.Subject == "" {
		score += 15
		reasons = append(reasons, "missing subject")
	}
	if file.School == "" {
		score += 10
		reasons = append(reasons, "missing school")
	}
	if file.Year == 0 {
		score += 10
		reasons = append(reasons, "missing year")
	}
	if isSuspiciousText(file.Name + " " + file.Remark) {
		score += 40
		reasons = append(reasons, "contains suspicious promotional or contact text")
	}
	if len(reasons) == 0 {
		reasons = append(reasons, "metadata looks complete")
	}

	action := "pass"
	level := "low"
	comment := "资料信息较完整，建议人工复核后通过。"
	if score >= 60 {
		action = "reject"
		level = "high"
		comment = "资料存在明显风险或关键信息缺失，建议驳回并要求重新上传。"
	} else if score >= 25 {
		action = "manual_review"
		level = "medium"
		comment = "资料存在部分信息缺失或疑点，建议管理员重点复核。"
	}

	return domain.AuditSuggestion{
		SuggestedAction: action,
		RiskLevel:       level,
		RiskScore:       score,
		Reasons:         reasons,
		ReviewComment:   comment,
	}
}

func extractYear(text string) int {
	re := regexp.MustCompile(`(20\d{2})`)
	match := re.FindStringSubmatch(text)
	if len(match) < 2 {
		return 0
	}
	year, _ := strconv.Atoi(match[1])
	if year < 2000 || year > 2100 {
		return 0
	}
	return year
}

func extractFileType(text string) int {
	lower := strings.ToLower(text)
	switch {
	case strings.Contains(lower, "期末"), strings.Contains(lower, "final"):
		return 1
	case strings.Contains(lower, "期中"), strings.Contains(lower, "midterm"):
		return 2
	case strings.Contains(lower, "补考"):
		return 4
	case strings.Contains(lower, "其他学校"):
		return 5
	case strings.Contains(lower, "资料"), strings.Contains(lower, "笔记"), strings.Contains(lower, "复习"):
		return 3
	default:
		return 0
	}
}

func extractSubject(text string) string {
	aliases := []struct {
		alias   string
		subject string
	}{
		{"高等数学", "高等数学"},
		{"高数", "高等数学"},
		{"线性代数", "线性代数"},
		{"线代", "线性代数"},
		{"概率论与数理统计", "概率论与数理统计"},
		{"概率统计", "概率论与数理统计"},
		{"概率论", "概率论与数理统计"},
		{"大学物理", "大学物理"},
		{"大物", "大学物理"},
		{"大学英语", "大学英语"},
		{"英语", "大学英语"},
		{"数据结构", "数据结构"},
		{"计算机网络", "计算机网络"},
		{"操作系统", "操作系统"},
		{"数据库", "数据库"},
		{"离散数学", "离散数学"},
		{"电路", "电路"},
		{"模拟电子技术", "模拟电子技术"},
		{"数字电子技术", "数字电子技术"},
		{"C语言", "C语言"},
		{"Java", "Java"},
		{"Python", "Python"},
	}
	for _, item := range aliases {
		if strings.Contains(strings.ToLower(text), strings.ToLower(item.alias)) {
			return item.subject
		}
	}
	return ""
}

func normalizeSubject(subject string) string {
	if subject == "" {
		return ""
	}
	normalized := extractSubject(subject)
	if normalized != "" {
		return normalized
	}
	return subject
}

func normalizeSearchFilters(filters *domain.FileSearchRequest) {
	filters.Subject = normalizeSubject(filters.Subject)
	if strings.TrimSpace(filters.Keyword) == "高数" {
		filters.Keyword = "高等数学"
	}
	if strings.TrimSpace(filters.Query) == "高数" {
		filters.Query = "高等数学"
	}
}

func normalizeMetadataSuggestion(suggestion *domain.MetadataSuggestion) {
	suggestion.Subject = normalizeSubject(suggestion.Subject)
}

func fileTypeName(value int) string {
	switch value {
	case 1:
		return "期末"
	case 2:
		return "期中"
	case 3:
		return "资料"
	case 4:
		return "补考"
	case 5:
		return "其他学校"
	default:
		return ""
	}
}

func looksLikeSearch(text string) bool {
	keywords := []string{"找", "搜索", "试卷", "资料", "期末", "期中", "补考", "真题", "下载"}
	for _, keyword := range keywords {
		if strings.Contains(text, keyword) {
			return true
		}
	}
	return false
}

func isSuspiciousText(text string) bool {
	lower := strings.ToLower(text)
	keywords := []string{"qq", "微信", "vx", "代考", "广告", "加群", "http://", "https://"}
	for _, keyword := range keywords {
		if strings.Contains(lower, keyword) {
			return true
		}
	}
	return false
}

func summarizeSearch(filters domain.FileSearchRequest, items []domain.FileItem) string {
	if len(items) == 0 {
		if filters.Keyword == "" {
			return "暂时没有找到匹配资料。"
		}
		return fmt.Sprintf("暂时没有找到与“%s”匹配的资料。", filters.Keyword)
	}
	return fmt.Sprintf("找到 %d 条相关资料，已按你的条件返回候选结果。", len(items))
}

func buildSearchResults(filters domain.FileSearchRequest, items []domain.FileItem) []domain.FileSearchResult {
	results := make([]domain.FileSearchResult, 0, len(items))
	for _, item := range items {
		score, reasons := scoreFileMatch(filters, item)
		results = append(results, domain.FileSearchResult{
			File:    item,
			Score:   score,
			Reasons: reasons,
		})
	}
	return results
}

func recommendFilters(req domain.FileRecommendRequest, seed *domain.FileItem, limit int) domain.FileSearchRequest {
	filters := domain.FileSearchRequest{
		Keyword:  req.Keyword,
		School:   req.School,
		Subject:  req.Subject,
		Year:     req.Year,
		Type:     req.Type,
		PageNum:  1,
		PageSize: limit * 3,
	}
	if seed != nil {
		if filters.Keyword == "" {
			filters.Keyword = seed.Subject
		}
		if filters.School == "" {
			filters.School = seed.School
		}
		if filters.Subject == "" {
			filters.Subject = seed.Subject
		}
		if filters.Year == 0 {
			filters.Year = seed.Year
		}
		if filters.Type == 0 {
			filters.Type = seed.Type
		}
	}
	normalizeSearchFilters(&filters)
	return filters
}

func (s *Service) collectRecommendationCandidates(ctx context.Context, filters domain.FileSearchRequest, seed *domain.FileItem, limit int) ([]domain.FileItem, error) {
	queries := []domain.FileSearchRequest{
		filters,
		{School: filters.School, Subject: filters.Subject, PageNum: 1, PageSize: limit * 3},
		{Subject: filters.Subject, PageNum: 1, PageSize: limit * 3},
		{Keyword: firstNonEmpty(filters.Subject, filters.Keyword), PageNum: 1, PageSize: limit * 3},
	}
	if filters.School != "" {
		queries = append(queries, domain.FileSearchRequest{School: filters.School, PageNum: 1, PageSize: limit * 2})
	}

	seen := map[int64]bool{}
	if seed != nil {
		seen[seed.FileID] = true
	}
	candidates := []domain.FileItem{}
	for _, query := range queries {
		normalizeSearchFilters(&query)
		items, err := s.backend.SearchFiles(ctx, query)
		if err != nil {
			return nil, err
		}
		for _, item := range items {
			if item.FileID == 0 || seen[item.FileID] {
				continue
			}
			seen[item.FileID] = true
			candidates = append(candidates, item)
		}
		if len(candidates) >= limit*3 {
			break
		}
	}
	return candidates, nil
}

func buildRecommendationResults(filters domain.FileSearchRequest, items []domain.FileItem, seed *domain.FileItem, limit int) []domain.FileSearchResult {
	results := make([]domain.FileSearchResult, 0, len(items))
	for _, item := range items {
		score, reasons := scoreFileMatch(filters, item)
		if seed != nil {
			extraScore, extraReasons := scoreSeedSimilarity(*seed, item)
			score += extraScore
			reasons = append(reasons, extraReasons...)
		}
		if score > 100 {
			score = 100
		}
		results = append(results, domain.FileSearchResult{
			File:    item,
			Score:   score,
			Reasons: dedupeStrings(reasons),
		})
	}

	sort.SliceStable(results, func(i, j int) bool {
		if results[i].Score == results[j].Score {
			return results[i].File.FileID > results[j].File.FileID
		}
		return results[i].Score > results[j].Score
	})
	if len(results) > limit {
		results = results[:limit]
	}
	return results
}

func scoreSeedSimilarity(seed, item domain.FileItem) (int, []string) {
	score := 0
	reasons := []string{}
	if seed.Subject != "" && seed.Subject == item.Subject {
		score += 15
		reasons = append(reasons, "same subject as current file")
	}
	if seed.School != "" && seed.School == item.School {
		score += 10
		reasons = append(reasons, "same school as current file")
	}
	if seed.Year > 0 && seed.Year == item.Year {
		score += 8
		reasons = append(reasons, "same year as current file")
	}
	if seed.Type > 0 && seed.Type == item.Type {
		score += 8
		reasons = append(reasons, "same file type as current file")
	}
	if seed.Remark != "" && seed.Remark == item.Remark {
		score += 5
		reasons = append(reasons, "same folder or collection")
	}
	return score, reasons
}

func scoreFileMatch(filters domain.FileSearchRequest, item domain.FileItem) (int, []string) {
	score := 0
	reasons := []string{}

	if filters.School != "" && strings.EqualFold(filters.School, item.School) {
		score += 20
		reasons = append(reasons, "school matched")
	}
	if filters.Subject != "" && strings.EqualFold(filters.Subject, item.Subject) {
		score += 25
		reasons = append(reasons, "subject matched")
	}
	if filters.Year > 0 && filters.Year == item.Year {
		score += 20
		reasons = append(reasons, "year matched")
	}
	if filters.Type > 0 && filters.Type == item.Type {
		score += 15
		reasons = append(reasons, "file type matched")
	}

	keyword := strings.TrimSpace(firstNonEmpty(filters.Keyword, filters.Query))
	if keyword != "" && containsFold(item.Name+" "+item.Subject+" "+item.School+" "+item.Remark, keyword) {
		score += 20
		reasons = append(reasons, "keyword matched file metadata")
	}
	if item.Status == 1 {
		score += 5
		reasons = append(reasons, "approved file")
	}
	if len(reasons) == 0 {
		reasons = append(reasons, "returned by backend search")
	}
	if score > 100 {
		score = 100
	}
	return score, reasons
}

func buildSearchSuggestions(filters domain.FileSearchRequest, items []domain.FileItem) []string {
	suggestions := []string{}
	if len(items) == 0 {
		if filters.Year > 0 {
			suggestions = append(suggestions, "try searching without year")
		}
		if filters.Type > 0 {
			suggestions = append(suggestions, "try searching without file type")
		}
		if filters.Subject != "" {
			suggestions = append(suggestions, "try searching by subject only")
		}
		if filters.Keyword != "" {
			suggestions = append(suggestions, "try a shorter keyword")
		}
		if len(suggestions) == 0 {
			suggestions = append(suggestions, "try another school, subject, or file keyword")
		}
		return suggestions
	}

	if len(items) < 3 {
		if filters.Year > 0 {
			suggestions = append(suggestions, "few results found; removing the year may show more files")
		}
		if filters.Type > 0 {
			suggestions = append(suggestions, "few results found; removing the file type may show related materials")
		}
	}
	return suggestions
}

func buildRecommendationSuggestions(filters domain.FileSearchRequest, results []domain.FileSearchResult) []string {
	if len(results) > 0 {
		if len(results) < 3 {
			return []string{"few recommendations found; try removing year or file type"}
		}
		return nil
	}
	suggestions := []string{"try recommending by subject only"}
	if filters.Year > 0 {
		suggestions = append(suggestions, "try removing year")
	}
	if filters.Type > 0 {
		suggestions = append(suggestions, "try removing file type")
	}
	if filters.School != "" {
		suggestions = append(suggestions, "try removing school")
	}
	return suggestions
}

func organizeFavoriteGroups(items []domain.FileItem, groupBy string) []domain.FavoriteGroup {
	grouped := map[string][]domain.FileItem{}
	labels := map[string]string{}
	for _, item := range items {
		key, label := favoriteGroupKey(item, groupBy)
		grouped[key] = append(grouped[key], item)
		labels[key] = label
	}

	groups := make([]domain.FavoriteGroup, 0, len(grouped))
	for key, groupItems := range grouped {
		groups = append(groups, domain.FavoriteGroup{
			Key:      key,
			Label:    labels[key],
			Count:    len(groupItems),
			Priority: favoriteGroupPriority(groupItems),
			Items:    groupItems,
		})
	}

	sort.SliceStable(groups, func(i, j int) bool {
		if groups[i].Priority == groups[j].Priority {
			return groups[i].Count > groups[j].Count
		}
		return priorityRank(groups[i].Priority) > priorityRank(groups[j].Priority)
	})
	return groups
}

func favoriteGroupKey(item domain.FileItem, groupBy string) (string, string) {
	switch groupBy {
	case "type":
		label := fileTypeName(item.Type)
		if label == "" {
			label = "Unknown type"
		}
		return fmt.Sprintf("type:%d", item.Type), label
	case "year":
		if item.Year <= 0 {
			return "year:unknown", "Unknown year"
		}
		return fmt.Sprintf("year:%d", item.Year), strconv.Itoa(item.Year)
	case "school":
		label := firstNonEmpty(item.School, "Unknown school")
		return "school:" + label, label
	default:
		label := firstNonEmpty(item.Subject, "Unknown subject")
		return "subject:" + label, label
	}
}

func favoriteGroupPriority(items []domain.FileItem) string {
	for _, item := range items {
		if item.Type == 1 || item.Type == 2 {
			return "high"
		}
	}
	if len(items) >= 5 {
		return "medium"
	}
	return "normal"
}

func priorityRank(priority string) int {
	switch priority {
	case "high":
		return 3
	case "medium":
		return 2
	default:
		return 1
	}
}

func buildFavoriteSuggestions(groups []domain.FavoriteGroup, items []domain.FileItem) []string {
	if len(items) == 0 {
		return []string{"favorite some files first, then organize them by subject or file type"}
	}
	suggestions := []string{}
	for _, group := range groups {
		if group.Priority == "high" {
			suggestions = append(suggestions, fmt.Sprintf("review %s first; it contains exam-oriented files", group.Label))
			break
		}
	}
	if len(groups) > 6 {
		suggestions = append(suggestions, "many small groups found; consider organizing by file type for faster review")
	}
	if len(suggestions) == 0 {
		suggestions = append(suggestions, "start with the largest subject group")
	}
	return suggestions
}

func summarizeFavorites(groups []domain.FavoriteGroup, items []domain.FileItem) string {
	if len(items) == 0 {
		return "No favorite files found."
	}
	return fmt.Sprintf("Organized %d favorite files into %d groups.", len(items), len(groups))
}

func summarizeRecommendations(results []domain.FileSearchResult) string {
	if len(results) == 0 {
		return "No related files found yet."
	}
	return fmt.Sprintf("Found %d related files.", len(results))
}

func containsFold(text, keyword string) bool {
	return strings.Contains(strings.ToLower(text), strings.ToLower(keyword))
}

func dedupeStrings(values []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, value := range values {
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func mergeSearch(base *domain.FileSearchRequest, extra domain.FileSearchRequest) {
	if extra.Keyword != "" {
		base.Keyword = extra.Keyword
	}
	if extra.School != "" {
		base.School = extra.School
	}
	if extra.Subject != "" {
		base.Subject = extra.Subject
	}
	if extra.Year > 0 {
		base.Year = extra.Year
	}
	if extra.Type > 0 {
		base.Type = extra.Type
	}
	if extra.PageNum > 0 {
		base.PageNum = extra.PageNum
	}
	if extra.PageSize > 0 {
		base.PageSize = extra.PageSize
	}
}

func mergeMetadata(base *domain.MetadataSuggestion, extra domain.MetadataSuggestion) {
	if extra.FileName != "" {
		base.FileName = extra.FileName
	}
	if extra.School != "" {
		base.School = extra.School
	}
	if extra.Subject != "" {
		base.Subject = extra.Subject
	}
	if extra.Year > 0 {
		base.Year = extra.Year
	}
	if extra.Type > 0 {
		base.Type = extra.Type
		base.TypeName = fileTypeName(extra.Type)
	}
	if extra.TypeName != "" {
		base.TypeName = extra.TypeName
	}
	if extra.Remark != "" {
		base.Remark = extra.Remark
	}
	if extra.Confidence > 0 {
		base.Confidence = extra.Confidence
	}
	if len(extra.Reasons) > 0 {
		base.Reasons = extra.Reasons
	}
}

func decodeJSONObject(content string, target interface{}) error {
	content = strings.TrimSpace(content)
	start := strings.Index(content, "{")
	end := strings.LastIndex(content, "}")
	if start >= 0 && end >= start {
		content = content[start : end+1]
	}
	return json.Unmarshal([]byte(content), target)
}

func mustJSON(value interface{}) string {
	data, _ := json.Marshal(value)
	return string(data)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
