package agent

import (
	"context"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"

	"PezMax-Agent/internal/domain"
)

func (s *Service) GenerateStudyPlan(ctx context.Context, req domain.StudyPlanRequest) (domain.StudyPlanResponse, error) {
	req = normalizeStudyPlanRequest(req)
	files, err := s.SearchFiles(ctx, domain.FileSearchRequest{
		Query:    firstNonEmpty(req.Goal, req.Subject),
		Keyword:  firstNonEmpty(req.Subject, req.Goal),
		School:   req.School,
		Subject:  req.Subject,
		Year:     req.Year,
		PageNum:  1,
		PageSize: 20,
	})
	if err != nil {
		return domain.StudyPlanResponse{}, err
	}

	plan := buildStudyPlanDays(req, files.Items)
	return domain.StudyPlanResponse{
		Intent:           "study_plan",
		Goal:             req.Goal,
		Subject:          req.Subject,
		Days:             req.Days,
		HoursPerDay:      req.HoursPerDay,
		Plan:             plan,
		RecommendedFiles: files.Results,
		Suggestions:      buildStudyPlanSuggestions(req, files.Items),
		Summary:          summarizeStudyPlan(req, len(files.Items)),
	}, nil
}

func normalizeStudyPlanRequest(req domain.StudyPlanRequest) domain.StudyPlanRequest {
	req.Goal = strings.TrimSpace(req.Goal)
	if req.Goal == "" {
		req.Goal = strings.TrimSpace(req.Subject)
	}
	if req.Subject == "" {
		req.Subject = extractSubject(req.Goal)
	}
	req.Subject = normalizeSubject(req.Subject)
	if req.Year == 0 {
		req.Year = extractYear(req.Goal)
	}
	if req.Days <= 0 {
		req.Days = extractDays(req.Goal)
	}
	if req.Days <= 0 {
		req.Days = 14
	}
	if req.Days < 1 {
		req.Days = 1
	}
	if req.Days > 60 {
		req.Days = 60
	}
	if req.HoursPerDay <= 0 {
		req.HoursPerDay = extractHoursPerDay(req.Goal)
	}
	if req.HoursPerDay <= 0 {
		req.HoursPerDay = 2
	}
	if req.HoursPerDay < 0.5 {
		req.HoursPerDay = 0.5
	}
	if req.HoursPerDay > 12 {
		req.HoursPerDay = 12
	}
	return req
}

func buildStudyPlanDays(req domain.StudyPlanRequest, files []domain.FileItem) []domain.StudyPlanDay {
	plan := make([]domain.StudyPlanDay, 0, req.Days)
	minutes := int(math.Round(req.HoursPerDay * 60))
	if minutes < 30 {
		minutes = 30
	}

	for day := 1; day <= req.Days; day++ {
		phase, focus := studyPlanPhase(day, req.Days)
		tasks := buildStudyTasks(phase, focus, minutes)
		plan = append(plan, domain.StudyPlanDay{
			Day:              day,
			Title:            fmt.Sprintf("第 %d 天：%s", day, phase),
			Focus:            focus,
			Tasks:            tasks,
			RecommendedFiles: pickStudyFiles(files, day, 2),
		})
	}
	return plan
}

func studyPlanPhase(day, total int) (string, string) {
	ratio := float64(day) / float64(total)
	switch {
	case ratio <= 0.35:
		return "基础梳理", "回顾核心概念，补齐公式、定义和常见题型"
	case ratio <= 0.7:
		return "专题训练", "围绕高频章节做分题型练习，记录错题原因"
	case ratio <= 0.9:
		return "真题模拟", "按考试节奏完成整套试卷或综合题"
	default:
		return "查漏补缺", "复盘错题与薄弱点，整理考前速记清单"
	}
}

func buildStudyTasks(phase, focus string, totalMinutes int) []domain.StudyTask {
	warmup := maxInt(15, totalMinutes/6)
	main := maxInt(30, totalMinutes*3/5)
	review := totalMinutes - warmup - main
	if review < 15 {
		review = 15
		main = totalMinutes - warmup - review
	}

	return []domain.StudyTask{
		{
			Type:    "review",
			Title:   "知识回顾",
			Detail:  focus,
			Minutes: warmup,
		},
		{
			Type:    "practice",
			Title:   phase,
			Detail:  "完成对应章节或试卷练习，优先处理不会做和易错题。",
			Minutes: main,
		},
		{
			Type:    "summary",
			Title:   "错题复盘",
			Detail:  "记录错误原因、正确解法和下次复习提醒。",
			Minutes: review,
		},
	}
}

func pickStudyFiles(files []domain.FileItem, day, limit int) []domain.FileItem {
	if len(files) == 0 || limit <= 0 {
		return nil
	}
	picked := make([]domain.FileItem, 0, limit)
	start := (day - 1) % len(files)
	for offset := 0; offset < len(files) && len(picked) < limit; offset++ {
		picked = append(picked, files[(start+offset)%len(files)])
	}
	return picked
}

func buildStudyPlanSuggestions(req domain.StudyPlanRequest, files []domain.FileItem) []string {
	suggestions := []string{
		"每天结束后把错题按知识点归档，下一天先复盘再做新题。",
		"临近考试前保留至少 2 天做整卷模拟和错题回看。",
	}
	if len(files) == 0 {
		suggestions = append(suggestions, "当前匹配资料较少，可以放宽年份或只按科目搜索。")
	}
	if req.HoursPerDay < 1.5 {
		suggestions = append(suggestions, "每日时间较短，建议优先做高频题型和近年真题。")
	}
	return suggestions
}

func summarizeStudyPlan(req domain.StudyPlanRequest, fileCount int) string {
	subject := req.Subject
	if subject == "" {
		subject = "当前目标"
	}
	return fmt.Sprintf("已为%s生成 %d 天学习计划，每天约 %.1f 小时，并匹配到 %d 份可参考资料。", subject, req.Days, req.HoursPerDay, fileCount)
}

func looksLikeStudyPlan(text string) bool {
	return containsAny(text, "学习计划", "复习计划", "备考计划", "学习安排", "复习安排", "规划", "备考")
}

func chatStudyPlanRequest(userID int64, text string) domain.StudyPlanRequest {
	return domain.StudyPlanRequest{
		UserID:      userID,
		Goal:        text,
		Subject:     extractSubject(text),
		Days:        extractDays(text),
		HoursPerDay: extractHoursPerDay(text),
		Year:        extractYear(text),
	}
}

func extractDays(text string) int {
	re := regexp.MustCompile(`(\d+)\s*(个星期|星期|周|个月|月|天|日)`)
	match := re.FindStringSubmatch(text)
	if len(match) >= 3 {
		value, _ := strconv.Atoi(match[1])
		switch {
		case strings.Contains(match[2], "月"):
			return value * 30
		case strings.Contains(match[2], "周") || strings.Contains(match[2], "星期"):
			return value * 7
		default:
			return value
		}
	}

	if strings.Contains(text, "半个月") {
		return 15
	}

	chineseRe := regexp.MustCompile(`([一二两三四五六七八九十]+)\s*(个星期|星期|周|个月|月|天|日)`)
	chineseMatch := chineseRe.FindStringSubmatch(text)
	if len(chineseMatch) >= 3 {
		value := chineseNumberToInt(chineseMatch[1])
		if value <= 0 {
			return 0
		}
		switch {
		case strings.Contains(chineseMatch[2], "月"):
			return value * 30
		case strings.Contains(chineseMatch[2], "周") || strings.Contains(chineseMatch[2], "星期"):
			return value * 7
		default:
			return value
		}
	}
	return 0
}

func chineseNumberToInt(text string) int {
	digits := map[rune]int{
		'一': 1,
		'二': 2,
		'两': 2,
		'三': 3,
		'四': 4,
		'五': 5,
		'六': 6,
		'七': 7,
		'八': 8,
		'九': 9,
	}
	if text == "十" {
		return 10
	}
	if strings.Contains(text, "十") {
		parts := strings.Split(text, "十")
		tens := 1
		if parts[0] != "" {
			for _, r := range parts[0] {
				tens = digits[r]
				break
			}
		}
		ones := 0
		if len(parts) > 1 && parts[1] != "" {
			for _, r := range parts[1] {
				ones = digits[r]
				break
			}
		}
		return tens*10 + ones
	}
	for _, r := range text {
		return digits[r]
	}
	return 0
}

func extractHoursPerDay(text string) float64 {
	re := regexp.MustCompile(`(?:每天|每日)?\s*(\d+(?:\.\d+)?)\s*(小时|h|H)`)
	match := re.FindStringSubmatch(text)
	if len(match) < 2 {
		return 0
	}
	value, _ := strconv.ParseFloat(match[1], 64)
	return value
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
