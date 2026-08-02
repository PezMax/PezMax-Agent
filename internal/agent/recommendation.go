package agent

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"PezMax-Agent/internal/domain"
)

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

func summarizeRecommendations(results []domain.FileSearchResult) string {
	if len(results) == 0 {
		return "No related files found yet."
	}
	return fmt.Sprintf("Found %d related files.", len(results))
}
