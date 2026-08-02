package agent

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"PezMax-Agent/internal/domain"
)

func (s *Service) OpsInsights(ctx context.Context, req domain.OpsInsightRequest) (domain.OpsInsightResponse, error) {
	if req.PageNum <= 0 {
		req.PageNum = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 200
	}
	if req.PageSize > 500 {
		req.PageSize = 500
	}

	filters := domain.FileSearchRequest{
		School:   strings.TrimSpace(req.School),
		Subject:  strings.TrimSpace(req.Subject),
		Year:     req.Year,
		PageNum:  req.PageNum,
		PageSize: req.PageSize,
	}
	files, err := s.backend.ListFiles(ctx, filters)
	if err != nil {
		return domain.OpsInsightResponse{}, err
	}
	downloads, err := s.backend.ListDownloads(ctx, req.PageNum, req.PageSize)
	if err != nil {
		return domain.OpsInsightResponse{}, err
	}
	reports, err := s.backend.ListReports(ctx, domain.ReportQuery{PageNum: 1, PageSize: 200})
	if err != nil {
		return domain.OpsInsightResponse{}, err
	}
	ranks, err := s.backend.ListUploadRanks(ctx)
	if err != nil {
		return domain.OpsInsightResponse{}, err
	}

	var notifications []domain.NotificationItem
	if req.IncludeNotifications {
		notifications, err = s.backend.ListNotifications(ctx, 1, 50)
		if err != nil {
			return domain.OpsInsightResponse{}, err
		}
	}

	fileByID := mapFilesByID(files)
	downloadCounts := countDownloadsByFile(downloads)
	reportCounts := countReportsByFile(reports)
	hotFiles := buildHotFileInsights(files, downloadCounts, reportCounts, 10)
	lowQuality := buildLowQualityInsights(files, reportCounts, 10)
	reportPressure := buildReportPressureInsights(fileByID, reportCounts, 10)
	rankTrends := buildUploaderRankInsights(ranks, 10)
	overview := buildOpsOverview(files, downloads, reports, notifications, lowQuality)
	notificationReach := buildNotificationReachSuggestions(hotFiles, lowQuality, reportPressure, notifications)

	return domain.OpsInsightResponse{
		Intent:            "ops_insights",
		Filters:           filters,
		Overview:          overview,
		HotFiles:          hotFiles,
		LowQualityFiles:   lowQuality,
		ReportPressure:    reportPressure,
		RankTrends:        rankTrends,
		NotificationReach: notificationReach,
		Suggestions:       buildOpsSuggestions(overview, hotFiles, lowQuality, reportPressure, rankTrends),
		Summary:           summarizeOpsInsights(overview, hotFiles, lowQuality, reportPressure),
	}, nil
}

func mapFilesByID(files []domain.FileItem) map[int64]domain.FileItem {
	out := make(map[int64]domain.FileItem, len(files))
	for _, file := range files {
		if file.FileID > 0 {
			out[file.FileID] = file
		}
	}
	return out
}

func countDownloadsByFile(downloads []domain.DownloadItem) map[int64]int {
	counts := map[int64]int{}
	for _, item := range downloads {
		if item.FileID > 0 {
			counts[item.FileID]++
		}
	}
	return counts
}

func countReportsByFile(reports []domain.ReportItem) map[int64]int {
	counts := map[int64]int{}
	for _, item := range reports {
		if item.FileID > 0 {
			counts[item.FileID]++
		}
	}
	return counts
}

func buildHotFileInsights(files []domain.FileItem, downloadCounts map[int64]int, reportCounts map[int64]int, limit int) []domain.HotFileInsight {
	insights := make([]domain.HotFileInsight, 0, len(files))
	for _, file := range files {
		downloads := downloadCounts[file.FileID]
		reports := reportCounts[file.FileID]
		score := downloads*10 - reports*3
		if file.Status == 1 {
			score += 5
		}
		if score <= 0 && downloads == 0 {
			continue
		}
		reasons := []string{}
		if downloads > 0 {
			reasons = append(reasons, fmt.Sprintf("downloaded %d times in sampled records", downloads))
		}
		if file.Subject != "" {
			reasons = append(reasons, fmt.Sprintf("belongs to active subject %s", file.Subject))
		}
		if reports > 0 {
			reasons = append(reasons, fmt.Sprintf("%d reports reduce confidence", reports))
		}
		insights = append(insights, domain.HotFileInsight{
			File:          file,
			DownloadCount: downloads,
			ReportCount:   reports,
			HotScore:      score,
			Reasons:       reasons,
		})
	}
	sort.SliceStable(insights, func(i, j int) bool {
		if insights[i].HotScore == insights[j].HotScore {
			return insights[i].DownloadCount > insights[j].DownloadCount
		}
		return insights[i].HotScore > insights[j].HotScore
	})
	if limit > 0 && len(insights) > limit {
		insights = insights[:limit]
	}
	return insights
}

func buildLowQualityInsights(files []domain.FileItem, reportCounts map[int64]int, limit int) []domain.QualityIssueInsight {
	issues := []domain.QualityIssueInsight{}
	for _, file := range files {
		score := 0
		reasons := []string{}
		if file.Status == 0 {
			score += 25
			reasons = append(reasons, "file is still pending review")
		}
		if file.Status == 2 {
			score += 70
			reasons = append(reasons, "file status indicates rejected or unavailable")
		}
		if file.Status == 3 {
			score += 80
			reasons = append(reasons, "file is marked as reported")
		}
		if reportCounts[file.FileID] > 0 {
			score += reportCounts[file.FileID] * 30
			reasons = append(reasons, fmt.Sprintf("received %d reports", reportCounts[file.FileID]))
		}
		if strings.TrimSpace(file.Name) == "" || strings.TrimSpace(file.School) == "" || strings.TrimSpace(file.Subject) == "" {
			score += 18
			reasons = append(reasons, "missing important metadata")
		}
		if file.Year == 0 {
			score += 8
			reasons = append(reasons, "missing file year")
		}
		if score == 0 {
			continue
		}
		issues = append(issues, domain.QualityIssueInsight{
			File:      file,
			RiskLevel: opsRiskLevel(score),
			Score:     minInt(score, 100),
			Reasons:   reasons,
		})
	}
	sort.SliceStable(issues, func(i, j int) bool {
		return issues[i].Score > issues[j].Score
	})
	if limit > 0 && len(issues) > limit {
		issues = issues[:limit]
	}
	return issues
}

func buildReportPressureInsights(fileByID map[int64]domain.FileItem, reportCounts map[int64]int, limit int) []domain.ReportPressureInsight {
	insights := []domain.ReportPressureInsight{}
	for fileID, count := range reportCounts {
		if count <= 0 {
			continue
		}
		score := count * 35
		reasons := []string{fmt.Sprintf("file has %d report records", count)}
		var filePtr *domain.FileItem
		if file, ok := fileByID[fileID]; ok {
			fileCopy := file
			filePtr = &fileCopy
			if file.Status == 3 {
				score += 30
				reasons = append(reasons, "file status is reported")
			}
		}
		insights = append(insights, domain.ReportPressureInsight{
			FileID:      fileID,
			File:        filePtr,
			ReportCount: count,
			RiskLevel:   opsRiskLevel(score),
			Reasons:     reasons,
		})
	}
	sort.SliceStable(insights, func(i, j int) bool {
		return insights[i].ReportCount > insights[j].ReportCount
	})
	if limit > 0 && len(insights) > limit {
		insights = insights[:limit]
	}
	return insights
}

func buildUploaderRankInsights(ranks []domain.UploaderRankItem, limit int) []domain.UploaderRankInsight {
	insights := make([]domain.UploaderRankInsight, 0, len(ranks))
	sort.SliceStable(ranks, func(i, j int) bool {
		return ranks[i].Count > ranks[j].Count
	})
	for i, user := range ranks {
		if limit > 0 && i >= limit {
			break
		}
		name := firstNonEmpty(user.NickName, user.UserName, fmt.Sprintf("user-%d", user.UserID))
		trend := "stable"
		insight := fmt.Sprintf("%s has contributed %d files.", name, user.Count)
		if i < 3 {
			trend = "leader"
			insight = fmt.Sprintf("%s is a top contributor; consider recognition or targeted retention.", name)
		} else if user.Count <= 1 {
			trend = "new_or_low_activity"
			insight = fmt.Sprintf("%s may need onboarding incentives to upload more.", name)
		}
		insights = append(insights, domain.UploaderRankInsight{
			Rank:    i + 1,
			User:    user,
			Trend:   trend,
			Insight: insight,
		})
	}
	return insights
}

func buildOpsOverview(files []domain.FileItem, downloads []domain.DownloadItem, reports []domain.ReportItem, notifications []domain.NotificationItem, issues []domain.QualityIssueInsight) domain.OpsOverview {
	subjectCounts := map[string]int{}
	schoolCounts := map[string]int{}
	for _, file := range files {
		if file.Subject != "" {
			subjectCounts[file.Subject]++
		}
		if file.School != "" {
			schoolCounts[file.School]++
		}
	}
	highRisk := 0
	for _, issue := range issues {
		if issue.RiskLevel == "high" {
			highRisk++
		}
	}
	return domain.OpsOverview{
		FileCount:         len(files),
		DownloadCount:     len(downloads),
		ReportCount:       len(reports),
		NotificationCount: len(notifications),
		HighRiskCount:     highRisk,
		HotSubject:        maxCountKey(subjectCounts),
		HotSchool:         maxCountKey(schoolCounts),
	}
}

func buildNotificationReachSuggestions(hotFiles []domain.HotFileInsight, issues []domain.QualityIssueInsight, pressure []domain.ReportPressureInsight, notifications []domain.NotificationItem) []domain.NotificationReachSuggestion {
	suggestions := []domain.NotificationReachSuggestion{}
	if len(hotFiles) > 0 {
		top := hotFiles[0]
		suggestions = append(suggestions, domain.NotificationReachSuggestion{
			Type:          "popular_material",
			Title:         "热门资料推荐",
			Audience:      firstNonEmpty(top.File.Subject, top.File.School, "全体用户"),
			Priority:      "medium",
			DraftContent:  fmt.Sprintf("%s 最近下载热度较高，可加入首页推荐或专题合集。", firstNonEmpty(top.File.Name, fmt.Sprintf("文件%d", top.File.FileID))),
			RelatedFileID: top.File.FileID,
			Confidence:    0.78,
		})
	}
	if len(pressure) > 0 && pressure[0].RiskLevel == "high" {
		suggestions = append(suggestions, domain.NotificationReachSuggestion{
			Type:          "risk_notice",
			Title:         "资料审核处理提醒",
			Audience:      "管理员",
			Priority:      "high",
			DraftContent:  fmt.Sprintf("文件 #%d 举报压力较高，建议优先复核并同步处理结果。", pressure[0].FileID),
			RelatedFileID: pressure[0].FileID,
			Confidence:    0.86,
		})
	}
	if len(issues) > 0 && issues[0].RiskLevel != "low" {
		suggestions = append(suggestions, domain.NotificationReachSuggestion{
			Type:          "quality_cleanup",
			Title:         "低质量资料治理",
			Audience:      "内容管理员",
			Priority:      issues[0].RiskLevel,
			DraftContent:  fmt.Sprintf("%s 存在元数据或审核风险，建议进入内容治理队列。", firstNonEmpty(issues[0].File.Name, fmt.Sprintf("文件%d", issues[0].File.FileID))),
			RelatedFileID: issues[0].File.FileID,
			Confidence:    0.74,
		})
	}
	if len(notifications) == 0 {
		suggestions = append(suggestions, domain.NotificationReachSuggestion{
			Type:         "daily_scroll",
			Title:        "日常运营触达",
			Audience:     "全体用户",
			Priority:     "low",
			DraftContent: "本周可发布资料上传激励或热门科目合集，引导用户补充高需求资料。",
			Confidence:   0.62,
		})
	}
	return suggestions
}

func buildOpsSuggestions(overview domain.OpsOverview, hotFiles []domain.HotFileInsight, issues []domain.QualityIssueInsight, pressure []domain.ReportPressureInsight, ranks []domain.UploaderRankInsight) []string {
	suggestions := []string{}
	if len(hotFiles) > 0 {
		suggestions = append(suggestions, "promote hot files into subject collections or ranking modules")
	}
	if overview.HighRiskCount > 0 {
		suggestions = append(suggestions, "prioritize high-risk files in the content moderation queue")
	}
	if len(pressure) > 0 {
		suggestions = append(suggestions, "group reports by fileId to avoid repeated manual review")
	}
	if len(ranks) > 0 {
		suggestions = append(suggestions, "use top uploader ranking for contribution incentives")
	}
	if overview.HotSubject != "" {
		suggestions = append(suggestions, fmt.Sprintf("prepare a focused acquisition plan for %s materials", overview.HotSubject))
	}
	return dedupeStrings(suggestions)
}

func summarizeOpsInsights(overview domain.OpsOverview, hotFiles []domain.HotFileInsight, issues []domain.QualityIssueInsight, pressure []domain.ReportPressureInsight) string {
	return fmt.Sprintf("Analyzed %d files, %d downloads, and %d reports; found %d hot files, %d quality risks, and %d reported-file pressure points.",
		overview.FileCount,
		overview.DownloadCount,
		overview.ReportCount,
		len(hotFiles),
		len(issues),
		len(pressure),
	)
}

func opsRiskLevel(score int) string {
	switch {
	case score >= 70:
		return "high"
	case score >= 35:
		return "medium"
	default:
		return "low"
	}
}

func maxCountKey(counts map[string]int) string {
	bestKey := ""
	bestCount := 0
	for key, count := range counts {
		if count > bestCount {
			bestKey = key
			bestCount = count
		}
	}
	return bestKey
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
