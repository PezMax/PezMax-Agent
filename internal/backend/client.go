package backend

import (
	"context"

	"PezMax-Agent/internal/domain"
)

type Client interface {
	SearchFiles(ctx context.Context, req domain.FileSearchRequest) ([]domain.FileItem, error)
	GetFile(ctx context.Context, fileID int64) (*domain.FileItem, error)
	ListFavorites(ctx context.Context, userID int64, pageNum int, pageSize int) ([]domain.FileItem, error)
	ListReports(ctx context.Context, req domain.ReportQuery) ([]domain.ReportItem, error)
	GetReport(ctx context.Context, reportID int64) (*domain.ReportItem, error)
	SuggestSchools(ctx context.Context, keyword string, limit int) ([]string, error)
	SuggestSubjects(ctx context.Context, keyword string, limit int) ([]string, error)
}
