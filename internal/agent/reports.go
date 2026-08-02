package agent

import (
	"context"
	"fmt"
	"strings"

	"PezMax-Agent/internal/domain"
)

func (s *Service) SummarizeReports(ctx context.Context, req domain.ReportSummarizeRequest) (domain.ReportSummarizeResponse, error) {
	query := domain.ReportQuery{
		ReportID: req.ReportID,
		FileID:   req.FileID,
		UserID:   req.UserID,
		Result:   req.Result,
		PageNum:  req.PageNum,
		PageSize: req.PageSize,
	}
	if query.PageNum <= 0 {
		query.PageNum = 1
	}
	if query.PageSize <= 0 {
		query.PageSize = 20
	}

	reports, err := s.loadReports(ctx, query)
	if err != nil {
		return domain.ReportSummarizeResponse{}, err
	}

	summaries := make([]domain.ReportSummary, 0, len(reports))
	for _, report := range reports {
		summary := s.buildReportSummary(ctx, report)
		summaries = append(summaries, summary)
	}

	return domain.ReportSummarizeResponse{
		Intent:      "reports_summarize",
		Filters:     query,
		Reports:     summaries,
		RiskLevel:   overallReportRisk(summaries),
		Suggestions: buildReportSuggestions(summaries),
		Summary:     summarizeReports(summaries),
	}, nil
}

func (s *Service) loadReports(ctx context.Context, query domain.ReportQuery) ([]domain.ReportItem, error) {
	if query.ReportID > 0 {
		report, err := s.backend.GetReport(ctx, query.ReportID)
		if err != nil {
			return nil, err
		}
		if report == nil {
			return nil, nil
		}
		return []domain.ReportItem{*report}, nil
	}
	return s.backend.ListReports(ctx, query)
}

func (s *Service) buildReportSummary(ctx context.Context, report domain.ReportItem) domain.ReportSummary {
	var file *domain.FileItem
	if report.FileID > 0 {
		detail, err := s.backend.GetFile(ctx, report.FileID)
		if err == nil {
			file = detail
		}
	}

	auditFile := domain.FileItem{}
	if file != nil {
		auditFile = *file
	} else {
		auditFile.FileID = report.FileID
	}
	audit := heuristicAudit(auditFile)
	audit.Reasons = append(audit.Reasons, reportRiskReasons(report)...)
	audit.Reasons = dedupeStrings(audit.Reasons)
	audit.RiskScore += reportRiskScore(report)
	if audit.RiskScore > 100 {
		audit.RiskScore = 100
	}
	audit.RiskLevel = riskLevel(audit.RiskScore)
	audit.SuggestedAction = reportSuggestedAction(audit.RiskScore, report)
	audit.ReviewComment = reportReviewComment(audit.SuggestedAction)

	return domain.ReportSummary{
		Report:       report,
		File:         file,
		Audit:        audit,
		Clues:        buildReportClues(report, file),
		NextActions:  buildReportNextActions(report, file, audit),
		TimelineText: buildReportTimelineText(report),
	}
}

func reportRiskScore(report domain.ReportItem) int {
	score := 0
	if report.Result == "0" || report.Result == "" {
		score += 15
	}
	if strings.TrimSpace(report.Reason) != "" {
		score += 20
	}
	if isSuspiciousText(report.Reason + " " + report.Remark) {
		score += 25
	}
	return score
}

func reportRiskReasons(report domain.ReportItem) []string {
	reasons := []string{}
	if report.Result == "0" || report.Result == "" {
		reasons = append(reasons, "report is pending")
	}
	if strings.TrimSpace(report.Reason) != "" {
		reasons = append(reasons, "user supplied a report reason")
	}
	if isSuspiciousText(report.Reason + " " + report.Remark) {
		reasons = append(reasons, "report text contains suspicious terms")
	}
	return reasons
}

func riskLevel(score int) string {
	switch {
	case score >= 60:
		return "high"
	case score >= 30:
		return "medium"
	default:
		return "low"
	}
}

func reportSuggestedAction(score int, report domain.ReportItem) string {
	if report.Result == "1" || report.Result == "2" {
		return "manual_review"
	}
	if score >= 60 {
		return "manual_review"
	}
	return "manual_review"
}

func reportReviewComment(action string) string {
	switch action {
	case "reject":
		return "Report clues indicate high risk; verify the file and consider rejecting the upload."
	default:
		return "Review the reported file, compare report reason with file metadata, then decide whether the report is valid."
	}
}

func buildReportClues(report domain.ReportItem, file *domain.FileItem) []string {
	clues := []string{
		fmt.Sprintf("report #%d targets file #%d", report.ReportID, report.FileID),
		"report result: " + reportResultLabel(report.Result),
	}
	if report.UserID > 0 {
		clues = append(clues, fmt.Sprintf("reported by user #%d", report.UserID))
	}
	if report.Reason != "" {
		clues = append(clues, "reason: "+report.Reason)
	}
	if report.Remark != "" {
		clues = append(clues, "remark: "+report.Remark)
	}
	if file != nil {
		clues = append(clues, "file name: "+file.Name)
		if file.Subject != "" {
			clues = append(clues, "file subject: "+file.Subject)
		}
		if file.School != "" {
			clues = append(clues, "file school: "+file.School)
		}
	}
	return clues
}

func buildReportNextActions(report domain.ReportItem, file *domain.FileItem, audit domain.AuditSuggestion) []string {
	actions := []string{"open the reported file and verify the report reason"}
	if file == nil {
		actions = append(actions, "file detail was unavailable; verify whether the file still exists")
		return actions
	}
	if file.Status != 1 {
		actions = append(actions, "file is not approved; check whether it should remain hidden")
	}
	if file.Subject == "" || file.School == "" {
		actions = append(actions, "complete missing metadata before closing the report")
	}
	if audit.RiskLevel == "high" {
		actions = append(actions, "prioritize this report in the moderation queue")
	}
	if report.Result == "0" || report.Result == "" {
		actions = append(actions, "after verification, mark report as valid or invalid")
	}
	return dedupeStrings(actions)
}

func buildReportTimelineText(report domain.ReportItem) string {
	parts := []string{}
	if report.CreateTime != "" {
		parts = append(parts, "submitted at "+report.CreateTime)
	}
	if report.UpdateTime != "" {
		parts = append(parts, "updated at "+report.UpdateTime)
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, "; ")
}

func reportResultLabel(result string) string {
	switch result {
	case "1":
		return "valid"
	case "2":
		return "invalid"
	default:
		return "pending"
	}
}

func overallReportRisk(summaries []domain.ReportSummary) string {
	maxScore := 0
	for _, summary := range summaries {
		if summary.Audit.RiskScore > maxScore {
			maxScore = summary.Audit.RiskScore
		}
	}
	return riskLevel(maxScore)
}

func buildReportSuggestions(summaries []domain.ReportSummary) []string {
	if len(summaries) == 0 {
		return []string{"no reports found for the current filters"}
	}
	suggestions := []string{}
	for _, summary := range summaries {
		if summary.Audit.RiskLevel == "high" {
			suggestions = append(suggestions, "handle high-risk reports first")
			break
		}
	}
	suggestions = append(suggestions, "batch reports by fileId to detect repeated complaints")
	return dedupeStrings(suggestions)
}

func summarizeReports(summaries []domain.ReportSummary) string {
	if len(summaries) == 0 {
		return "No reports found."
	}
	pending := 0
	for _, summary := range summaries {
		if summary.Report.Result == "" || summary.Report.Result == "0" {
			pending++
		}
	}
	return fmt.Sprintf("Summarized %d reports; %d are pending.", len(summaries), pending)
}
