package backend

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"PezMax-Agent/internal/config"
	"PezMax-Agent/internal/domain"
)

type JavaClient struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

func NewJavaClient(cfg config.Config) *JavaClient {
	if cfg.BackendBaseURL == "" {
		log.Printf("backend adapter disabled: PEZMAX_BACKEND_BASE_URL is empty")
	} else {
		log.Printf("backend adapter enabled: baseURL=%s", cfg.BackendBaseURL)
	}
	return &JavaClient{
		baseURL: cfg.BackendBaseURL,
		token:   cfg.BackendToken,
		httpClient: &http.Client{
			Timeout: cfg.BackendTimeout,
		},
	}
}

func (c *JavaClient) ListFiles(ctx context.Context, req domain.FileSearchRequest) ([]domain.FileItem, error) {
	if c.baseURL == "" {
		log.Printf("skip backend file list: PEZMAX_BACKEND_BASE_URL is empty")
		return nil, nil
	}
	if req.PageNum <= 0 {
		req.PageNum = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 200
	}

	values := url.Values{}
	if req.School != "" {
		values.Set("fileSchool", req.School)
	}
	if req.Subject != "" {
		values.Set("fileSubject", req.Subject)
	}
	if req.Year > 0 {
		values.Set("fileYear", strconv.Itoa(req.Year))
	}
	if req.Type > 0 {
		values.Set("fileType", strconv.Itoa(req.Type))
	}
	values.Set("pageNum", strconv.Itoa(req.PageNum))
	values.Set("pageSize", strconv.Itoa(req.PageSize))

	body, err := c.get(ctx, "/datum/file/list", values)
	if err != nil {
		return nil, err
	}
	items, err := decodeFileItems(body)
	if err != nil {
		return nil, err
	}
	log.Printf("backend file list decoded rows=%d", len(items))
	return items, nil
}

func (c *JavaClient) SearchFiles(ctx context.Context, req domain.FileSearchRequest) ([]domain.FileItem, error) {
	if c.baseURL == "" {
		log.Printf("skip backend file search: PEZMAX_BACKEND_BASE_URL is empty")
		return nil, nil
	}

	values := url.Values{}
	keyword := firstNonEmpty(req.Keyword, req.Query)
	if keyword != "" {
		values.Set("keyword", keyword)
	}
	if req.School != "" {
		values.Set("fileSchool", req.School)
	}
	if req.Subject != "" {
		values.Set("fileSubject", req.Subject)
	}
	if req.Year > 0 {
		values.Set("fileYear", strconv.Itoa(req.Year))
	}
	if req.Type > 0 {
		values.Set("fileType", strconv.Itoa(req.Type))
	}
	if req.PageNum > 0 {
		values.Set("pageNum", strconv.Itoa(req.PageNum))
	}
	if req.PageSize > 0 {
		values.Set("pageSize", strconv.Itoa(req.PageSize))
	}

	path := "/datum/file/search"
	if req.School != "" || req.Subject != "" || req.Year > 0 || req.Type > 0 {
		path = "/datum/file/list"
	}

	body, err := c.get(ctx, path, values)
	if err != nil {
		return nil, err
	}
	items, err := decodeFileItems(body)
	if err != nil {
		return nil, err
	}
	log.Printf("backend file search decoded rows=%d", len(items))

	if len(items) == 0 && path == "/datum/file/list" && keyword != "" {
		fallbackValues := url.Values{}
		fallbackValues.Set("keyword", keyword)
		log.Printf("backend file search fallback: structured list returned 0 rows, trying /datum/file/search")
		body, err = c.get(ctx, "/datum/file/search", fallbackValues)
		if err != nil {
			return nil, err
		}
		items, err = decodeFileItems(body)
		if err != nil {
			return nil, err
		}
		log.Printf("backend file search fallback decoded rows=%d", len(items))
	}
	return items, nil
}

func (c *JavaClient) GetFile(ctx context.Context, fileID int64) (*domain.FileItem, error) {
	if c.baseURL == "" {
		log.Printf("skip backend file detail: PEZMAX_BACKEND_BASE_URL is empty")
		return nil, nil
	}
	body, err := c.get(ctx, fmt.Sprintf("/datum/file/%d", fileID), nil)
	if err != nil {
		return nil, err
	}
	item, err := decodeFileItem(body)
	if err == nil && item != nil {
		return item, nil
	}

	values := url.Values{}
	values.Set("fileId", strconv.FormatInt(fileID, 10))
	values.Set("pageNum", "1")
	values.Set("pageSize", "1")
	log.Printf("backend file detail fallback: trying /datum/file/list?fileId=%d", fileID)
	body, listErr := c.get(ctx, "/datum/file/list", values)
	if listErr != nil {
		return nil, err
	}
	items, listErr := decodeFileItems(body)
	if listErr != nil {
		return nil, listErr
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("backend file %d not found", fileID)
	}
	return &items[0], nil
}

func (c *JavaClient) ListFavorites(ctx context.Context, userID int64, pageNum int, pageSize int) ([]domain.FileItem, error) {
	if c.baseURL == "" {
		log.Printf("skip backend favorite list: PEZMAX_BACKEND_BASE_URL is empty")
		return nil, nil
	}
	if pageNum <= 0 {
		pageNum = 1
	}
	if pageSize <= 0 {
		pageSize = 100
	}

	values := url.Values{}
	values.Set("pageNum", strconv.Itoa(pageNum))
	values.Set("pageSize", strconv.Itoa(pageSize))
	body, err := c.get(ctx, fmt.Sprintf("/datum/desktop/favorite/list/%d", userID), values)
	if err != nil {
		return nil, err
	}
	items, err := decodeFileItems(body)
	if err != nil {
		return nil, err
	}
	log.Printf("backend favorite list decoded rows=%d", len(items))
	return c.enrichFiles(ctx, items), nil
}

func (c *JavaClient) ListDownloads(ctx context.Context, pageNum int, pageSize int) ([]domain.DownloadItem, error) {
	if c.baseURL == "" {
		log.Printf("skip backend download list: PEZMAX_BACKEND_BASE_URL is empty")
		return nil, nil
	}
	if pageNum <= 0 {
		pageNum = 1
	}
	if pageSize <= 0 {
		pageSize = 200
	}

	values := url.Values{}
	values.Set("pageNum", strconv.Itoa(pageNum))
	values.Set("pageSize", strconv.Itoa(pageSize))
	body, err := c.get(ctx, "/datum/download/list", values)
	if err != nil {
		return nil, err
	}
	items, err := decodeDownloadItems(body)
	if err != nil {
		return nil, err
	}
	log.Printf("backend download list decoded rows=%d", len(items))
	return items, nil
}

func (c *JavaClient) ListReports(ctx context.Context, req domain.ReportQuery) ([]domain.ReportItem, error) {
	if c.baseURL == "" {
		log.Printf("skip backend report list: PEZMAX_BACKEND_BASE_URL is empty")
		return nil, nil
	}
	if req.PageNum <= 0 {
		req.PageNum = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 50
	}

	values := url.Values{}
	if req.FileID > 0 {
		values.Set("fileId", strconv.FormatInt(req.FileID, 10))
	}
	if req.UserID > 0 {
		values.Set("userId", strconv.FormatInt(req.UserID, 10))
	}
	if req.Result != "" {
		values.Set("result", req.Result)
	}
	values.Set("pageNum", strconv.Itoa(req.PageNum))
	values.Set("pageSize", strconv.Itoa(req.PageSize))

	body, err := c.get(ctx, "/datum/report/list", values)
	if err != nil {
		return nil, err
	}
	reports, err := decodeReportItems(body)
	if err != nil {
		return nil, err
	}
	log.Printf("backend report list decoded rows=%d", len(reports))
	return reports, nil
}

func (c *JavaClient) GetReport(ctx context.Context, reportID int64) (*domain.ReportItem, error) {
	if c.baseURL == "" {
		log.Printf("skip backend report detail: PEZMAX_BACKEND_BASE_URL is empty")
		return nil, nil
	}
	body, err := c.get(ctx, fmt.Sprintf("/datum/report/%d", reportID), nil)
	if err != nil {
		return nil, err
	}
	report, err := decodeReportItem(body)
	if err == nil && report != nil {
		return report, nil
	}

	reports, listErr := c.ListReports(ctx, domain.ReportQuery{
		ReportID: reportID,
		PageNum:  1,
		PageSize: 1,
	})
	if listErr != nil {
		return nil, err
	}
	if len(reports) == 0 {
		return nil, fmt.Errorf("backend report %d not found", reportID)
	}
	return &reports[0], nil
}

func (c *JavaClient) ListNotifications(ctx context.Context, pageNum int, pageSize int) ([]domain.NotificationItem, error) {
	if c.baseURL == "" {
		log.Printf("skip backend notification list: PEZMAX_BACKEND_BASE_URL is empty")
		return nil, nil
	}
	if pageNum <= 0 {
		pageNum = 1
	}
	if pageSize <= 0 {
		pageSize = 50
	}

	values := url.Values{}
	values.Set("pageNum", strconv.Itoa(pageNum))
	values.Set("pageSize", strconv.Itoa(pageSize))
	body, err := c.get(ctx, "/system/notification/list", values)
	if err != nil {
		return nil, err
	}
	items, err := decodeNotificationItems(body)
	if err != nil {
		return nil, err
	}
	log.Printf("backend notification list decoded rows=%d", len(items))
	return items, nil
}

func (c *JavaClient) ListUploadRanks(ctx context.Context) ([]domain.UploaderRankItem, error) {
	if c.baseURL == "" {
		log.Printf("skip backend upload rank: PEZMAX_BACKEND_BASE_URL is empty")
		return nil, nil
	}

	body, err := c.get(ctx, "/datum/user/rank", nil)
	if err != nil {
		return nil, err
	}
	items, err := decodeUploadRanks(body)
	if err != nil {
		return nil, err
	}
	log.Printf("backend upload rank decoded rows=%d", len(items))
	return items, nil
}

func (c *JavaClient) enrichFiles(ctx context.Context, items []domain.FileItem) []domain.FileItem {
	for i := range items {
		if items[i].FileID == 0 || (items[i].School != "" && items[i].Subject != "") {
			continue
		}
		detail, err := c.GetFile(ctx, items[i].FileID)
		if err != nil || detail == nil {
			continue
		}
		if items[i].School == "" {
			items[i].School = detail.School
		}
		if items[i].Subject == "" {
			items[i].Subject = detail.Subject
		}
		if items[i].Year == 0 {
			items[i].Year = detail.Year
		}
		if items[i].Type == 0 {
			items[i].Type = detail.Type
		}
		if items[i].URL == "" {
			items[i].URL = detail.URL
		}
	}
	return items
}

func decodeFileItem(body []byte) (*domain.FileItem, error) {
	var item domain.FileItem
	if err := json.Unmarshal(body, &item); err == nil && item.FileID != 0 {
		return &item, nil
	}

	var ajax struct {
		Code int             `json:"code"`
		Msg  string          `json:"msg"`
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(body, &ajax); err != nil {
		return nil, err
	}
	if ajax.Code != 0 && ajax.Code != 200 {
		return nil, fmt.Errorf("backend returned code=%d msg=%s", ajax.Code, ajax.Msg)
	}
	if len(ajax.Data) == 0 || string(ajax.Data) == "null" {
		return nil, fmt.Errorf("backend returned empty file data")
	}
	if err := json.Unmarshal(ajax.Data, &item); err != nil {
		return nil, err
	}
	if item.FileID == 0 {
		return nil, fmt.Errorf("backend returned invalid file data")
	}
	return &item, nil
}

func (c *JavaClient) SuggestSchools(ctx context.Context, keyword string, limit int) ([]string, error) {
	return c.suggest(ctx, "/datum/file/schools", keyword, limit)
}

func (c *JavaClient) SuggestSubjects(ctx context.Context, keyword string, limit int) ([]string, error) {
	return c.suggest(ctx, "/datum/file/subjects", keyword, limit)
}

func (c *JavaClient) suggest(ctx context.Context, path, keyword string, limit int) ([]string, error) {
	if c.baseURL == "" {
		log.Printf("skip backend suggestion %s: PEZMAX_BACKEND_BASE_URL is empty", path)
		return nil, nil
	}
	values := url.Values{}
	if keyword != "" {
		values.Set("keyword", keyword)
	}
	if limit <= 0 {
		limit = 10
	}
	values.Set("limit", strconv.Itoa(limit))

	body, err := c.get(ctx, path, values)
	if err != nil {
		return nil, err
	}

	var ajax struct {
		Data []struct {
			Value string `json:"value"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &ajax); err != nil {
		return nil, err
	}
	valuesOut := make([]string, 0, len(ajax.Data))
	for _, item := range ajax.Data {
		if item.Value != "" {
			valuesOut = append(valuesOut, item.Value)
		}
	}
	return valuesOut, nil
}

func (c *JavaClient) get(ctx context.Context, path string, values url.Values) ([]byte, error) {
	endpoint := c.baseURL + path
	if len(values) > 0 {
		endpoint += "?" + values.Encode()
	}
	log.Printf("backend request: GET %s", endpoint)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}

	token := firstNonEmpty(authTokenFromContext(ctx), c.token)
	if token != "" {
		if strings.HasPrefix(strings.ToLower(token), "bearer ") {
			req.Header.Set("Authorization", token)
		} else {
			req.Header.Set("Authorization", "Bearer "+token)
		}
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		log.Printf("backend response: GET %s status=%d", endpoint, resp.StatusCode)
		return nil, fmt.Errorf("backend %s returned status %d", path, resp.StatusCode)
	}
	log.Printf("backend response: GET %s status=%d", endpoint, resp.StatusCode)

	var raw json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, err
	}
	return raw, nil
}

func decodeFileItems(body []byte) ([]domain.FileItem, error) {
	var direct []domain.FileItem
	if err := json.Unmarshal(body, &direct); err == nil {
		return direct, nil
	}

	var table struct {
		Rows []domain.FileItem `json:"rows"`
	}
	if err := json.Unmarshal(body, &table); err == nil && table.Rows != nil {
		return table.Rows, nil
	}

	var ajax struct {
		Data json.RawMessage   `json:"data"`
		Rows []domain.FileItem `json:"rows"`
	}
	if err := json.Unmarshal(body, &ajax); err != nil {
		return nil, err
	}
	if len(ajax.Rows) > 0 {
		return ajax.Rows, nil
	}
	if len(ajax.Data) == 0 {
		return nil, nil
	}
	if err := json.Unmarshal(ajax.Data, &direct); err == nil {
		return direct, nil
	}
	var dataTable struct {
		Rows []domain.FileItem `json:"rows"`
	}
	if err := json.Unmarshal(ajax.Data, &dataTable); err != nil {
		return nil, err
	}
	return dataTable.Rows, nil
}

func decodeReportItem(body []byte) (*domain.ReportItem, error) {
	var item domain.ReportItem
	if err := json.Unmarshal(body, &item); err == nil && item.ReportID != 0 {
		return &item, nil
	}

	var ajax struct {
		Code int             `json:"code"`
		Msg  string          `json:"msg"`
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(body, &ajax); err != nil {
		return nil, err
	}
	if ajax.Code != 0 && ajax.Code != 200 {
		return nil, fmt.Errorf("backend returned code=%d msg=%s", ajax.Code, ajax.Msg)
	}
	if len(ajax.Data) == 0 || string(ajax.Data) == "null" {
		return nil, fmt.Errorf("backend returned empty report data")
	}
	if err := json.Unmarshal(ajax.Data, &item); err != nil {
		return nil, err
	}
	if item.ReportID == 0 {
		return nil, fmt.Errorf("backend returned invalid report data")
	}
	return &item, nil
}

func decodeReportItems(body []byte) ([]domain.ReportItem, error) {
	var direct []domain.ReportItem
	if err := json.Unmarshal(body, &direct); err == nil {
		return direct, nil
	}

	var table struct {
		Rows []domain.ReportItem `json:"rows"`
	}
	if err := json.Unmarshal(body, &table); err == nil && table.Rows != nil {
		return table.Rows, nil
	}

	var ajax struct {
		Data json.RawMessage     `json:"data"`
		Rows []domain.ReportItem `json:"rows"`
	}
	if err := json.Unmarshal(body, &ajax); err != nil {
		return nil, err
	}
	if len(ajax.Rows) > 0 {
		return ajax.Rows, nil
	}
	if len(ajax.Data) == 0 {
		return nil, nil
	}
	if err := json.Unmarshal(ajax.Data, &direct); err == nil {
		return direct, nil
	}
	var dataTable struct {
		Rows []domain.ReportItem `json:"rows"`
	}
	if err := json.Unmarshal(ajax.Data, &dataTable); err != nil {
		return nil, err
	}
	return dataTable.Rows, nil
}

func decodeDownloadItems(body []byte) ([]domain.DownloadItem, error) {
	var direct []domain.DownloadItem
	if err := json.Unmarshal(body, &direct); err == nil {
		return direct, nil
	}

	var table struct {
		Rows []domain.DownloadItem `json:"rows"`
	}
	if err := json.Unmarshal(body, &table); err == nil && table.Rows != nil {
		return table.Rows, nil
	}

	var ajax struct {
		Data json.RawMessage       `json:"data"`
		Rows []domain.DownloadItem `json:"rows"`
	}
	if err := json.Unmarshal(body, &ajax); err != nil {
		return nil, err
	}
	if len(ajax.Rows) > 0 {
		return ajax.Rows, nil
	}
	if len(ajax.Data) == 0 {
		return nil, nil
	}
	if err := json.Unmarshal(ajax.Data, &direct); err == nil {
		return direct, nil
	}
	var dataTable struct {
		Rows []domain.DownloadItem `json:"rows"`
	}
	if err := json.Unmarshal(ajax.Data, &dataTable); err != nil {
		return nil, err
	}
	return dataTable.Rows, nil
}

func decodeNotificationItems(body []byte) ([]domain.NotificationItem, error) {
	var direct []domain.NotificationItem
	if err := json.Unmarshal(body, &direct); err == nil {
		return direct, nil
	}

	var table struct {
		Rows []domain.NotificationItem `json:"rows"`
	}
	if err := json.Unmarshal(body, &table); err == nil && table.Rows != nil {
		return table.Rows, nil
	}

	var ajax struct {
		Data json.RawMessage           `json:"data"`
		Rows []domain.NotificationItem `json:"rows"`
	}
	if err := json.Unmarshal(body, &ajax); err != nil {
		return nil, err
	}
	if len(ajax.Rows) > 0 {
		return ajax.Rows, nil
	}
	if len(ajax.Data) == 0 {
		return nil, nil
	}
	if err := json.Unmarshal(ajax.Data, &direct); err == nil {
		return direct, nil
	}
	var dataTable struct {
		Rows []domain.NotificationItem `json:"rows"`
	}
	if err := json.Unmarshal(ajax.Data, &dataTable); err != nil {
		return nil, err
	}
	return dataTable.Rows, nil
}

func decodeUploadRanks(body []byte) ([]domain.UploaderRankItem, error) {
	var direct []domain.UploaderRankItem
	if err := json.Unmarshal(body, &direct); err == nil {
		return direct, nil
	}

	var ajax struct {
		Data json.RawMessage           `json:"data"`
		Rows []domain.UploaderRankItem `json:"rows"`
	}
	if err := json.Unmarshal(body, &ajax); err != nil {
		return nil, err
	}
	if len(ajax.Rows) > 0 {
		return ajax.Rows, nil
	}
	if len(ajax.Data) == 0 {
		return nil, nil
	}
	if err := json.Unmarshal(ajax.Data, &direct); err == nil {
		return direct, nil
	}
	var dataTable struct {
		Rows []domain.UploaderRankItem `json:"rows"`
	}
	if err := json.Unmarshal(ajax.Data, &dataTable); err != nil {
		return nil, err
	}
	return dataTable.Rows, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
