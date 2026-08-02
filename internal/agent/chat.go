package agent

import (
	"context"
	"strings"

	"PezMax-Agent/internal/domain"
)

func (s *Service) Chat(ctx context.Context, req domain.ChatRequest) (domain.ChatResponse, error) {
	if looksLikeMockExam(req.Message) {
		exam, err := s.GenerateMockExam(ctx, chatMockExamRequest(req.UserID, req.Message))
		if err != nil {
			return domain.ChatResponse{}, err
		}
		return domain.ChatResponse{
			Intent: "mock_exam",
			Answer: exam.Summary,
			Data:   exam,
		}, nil
	}

	if looksLikeStudyPlan(req.Message) {
		plan, err := s.GenerateStudyPlan(ctx, chatStudyPlanRequest(req.UserID, req.Message))
		if err != nil {
			return domain.ChatResponse{}, err
		}
		return domain.ChatResponse{
			Intent: "study_plan",
			Answer: plan.Summary,
			Data:   plan,
		}, nil
	}

	if looksLikeRecommendation(req.Message) {
		recommend, err := s.RecommendFiles(ctx, chatRecommendRequest(req.Message))
		if err != nil {
			return domain.ChatResponse{}, err
		}
		return domain.ChatResponse{
			Intent: "file_recommend",
			Answer: recommend.Summary,
			Data:   recommend,
		}, nil
	}

	if looksLikeFavoriteOrganize(req.Message) {
		if req.UserID <= 0 {
			return domain.ChatResponse{
				Intent: "favorites_organize",
				Answer: "需要先获取当前用户信息，才能整理你的收藏。",
			}, nil
		}
		favorites, err := s.OrganizeFavorites(ctx, domain.FavoriteOrganizeRequest{
			UserID:   req.UserID,
			GroupBy:  chatFavoriteGroupBy(req.Message),
			PageNum:  1,
			PageSize: 100,
		})
		if err != nil {
			return domain.ChatResponse{}, err
		}
		return domain.ChatResponse{
			Intent: "favorites_organize",
			Answer: favorites.Summary,
			Data:   favorites,
		}, nil
	}

	if looksLikeReportSummary(req.Message) {
		reports, err := s.SummarizeReports(ctx, domain.ReportSummarizeRequest{
			PageNum:  1,
			PageSize: 20,
		})
		if err != nil {
			return domain.ChatResponse{}, err
		}
		return domain.ChatResponse{
			Intent: "reports_summarize",
			Answer: reports.Summary,
			Data:   reports,
		}, nil
	}

	if looksLikeOpsInsights(req.Message) {
		ops, err := s.OpsInsights(ctx, domain.OpsInsightRequest{
			PageNum:              1,
			PageSize:             200,
			Subject:              extractSubject(req.Message),
			Year:                 extractYear(req.Message),
			IncludeNotifications: strings.Contains(req.Message, "通知") || strings.Contains(req.Message, "触达"),
		})
		if err != nil {
			return domain.ChatResponse{}, err
		}
		return domain.ChatResponse{
			Intent: "ops_insights",
			Answer: ops.Summary,
			Data:   ops,
		}, nil
	}

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
			Answer: "智能体服务已启动，但当前未配置 DASHSCOPE_API_KEY。" + chatFallbackAnswer(),
		}, nil
	}

	msg, err := s.generate(ctx, chatSystemPrompt, req.Message)
	if err != nil {
		return domain.ChatResponse{}, err
	}
	return domain.ChatResponse{Intent: "general", Answer: msg.Content}, nil
}

func looksLikeRecommendation(text string) bool {
	return containsAny(text, "推荐", "相似", "类似", "相关资料", "复习方向")
}

func looksLikeFavoriteOrganize(text string) bool {
	return containsAny(text, "收藏", "整理", "归类", "分类") && containsAny(text, "收藏", "我的")
}

func looksLikeReportSummary(text string) bool {
	return containsAny(text, "举报", "投诉", "违规", "审核线索", "审核意见")
}

func looksLikeOpsInsights(text string) bool {
	return containsAny(text, "运营", "热门", "低质量", "排行榜", "趋势", "触达", "通知", "数据概览")
}

func chatRecommendRequest(text string) domain.FileRecommendRequest {
	return domain.FileRecommendRequest{
		Keyword: text,
		Subject: extractSubject(text),
		Year:    extractYear(text),
		Type:    extractFileType(text),
		Limit:   8,
	}
}

func chatFavoriteGroupBy(text string) string {
	switch {
	case containsAny(text, "年份", "年"):
		return "year"
	case containsAny(text, "类型", "分类"):
		return "type"
	case containsAny(text, "学校"):
		return "school"
	default:
		return "subject"
	}
}

func containsAny(text string, keywords ...string) bool {
	for _, keyword := range keywords {
		if strings.Contains(text, keyword) {
			return true
		}
	}
	return false
}

func chatFallbackAnswer() string {
	return "我可以帮你找资料、推荐资料、生成学习计划、根据真题出模拟题、整理收藏、分析举报和查看运营洞察。"
}
