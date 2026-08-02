package agent

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"PezMax-Agent/internal/domain"
)

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

func normalizeMetadataSuggestion(suggestion *domain.MetadataSuggestion) {
	suggestion.Subject = normalizeSubject(suggestion.Subject)
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
