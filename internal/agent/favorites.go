package agent

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"PezMax-Agent/internal/domain"
)

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
