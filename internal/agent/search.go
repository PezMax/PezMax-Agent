package agent

import (
	"context"
	"fmt"
	"strings"

	"PezMax-Agent/internal/domain"
)

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

func normalizeSearchFilters(filters *domain.FileSearchRequest) {
	filters.Subject = normalizeSubject(filters.Subject)
	if strings.TrimSpace(filters.Keyword) == "高数" {
		filters.Keyword = "高等数学"
	}
	if strings.TrimSpace(filters.Query) == "高数" {
		filters.Query = "高等数学"
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
