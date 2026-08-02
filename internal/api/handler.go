package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"PezMax-Agent/internal/agent"
	"PezMax-Agent/internal/backend"
	"PezMax-Agent/internal/domain"
)

type Handler struct {
	service *agent.Service
}

func NewHandler(service *agent.Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", h.health)
	mux.HandleFunc("POST /api/v1/agent/chat", h.chat)
	mux.HandleFunc("POST /api/v1/agent/files/search", h.searchFiles)
	mux.HandleFunc("POST /api/v1/agent/files/recommend", h.recommendFiles)
	mux.HandleFunc("POST /api/v1/agent/files/metadata/suggest", h.suggestMetadata)
	mux.HandleFunc("POST /api/v1/agent/files/audit/suggest", h.suggestAudit)
	mux.HandleFunc("POST /api/v1/agent/study/plan", h.generateStudyPlan)
	mux.HandleFunc("POST /api/v1/agent/favorites/organize", h.organizeFavorites)
	mux.HandleFunc("POST /api/v1/agent/reports/summarize", h.summarizeReports)
	mux.HandleFunc("POST /api/v1/agent/ops/insights", h.opsInsights)
	return withCORS(mux)
}

func (h *Handler) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"service": "PezMax-Agent",
	})
}

func (h *Handler) chat(w http.ResponseWriter, r *http.Request) {
	var req domain.ChatRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if strings.TrimSpace(req.Message) == "" {
		writeError(w, http.StatusBadRequest, errors.New("message is required"))
		return
	}

	resp, err := h.service.Chat(requestContext(r), req)
	if err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) searchFiles(w http.ResponseWriter, r *http.Request) {
	var req domain.FileSearchRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if strings.TrimSpace(req.Query) == "" && strings.TrimSpace(req.Keyword) == "" {
		writeError(w, http.StatusBadRequest, errors.New("query or keyword is required"))
		return
	}

	resp, err := h.service.SearchFiles(requestContext(r), req)
	if err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) recommendFiles(w http.ResponseWriter, r *http.Request) {
	var req domain.FileRecommendRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if req.FileID <= 0 &&
		strings.TrimSpace(req.Keyword) == "" &&
		strings.TrimSpace(req.School) == "" &&
		strings.TrimSpace(req.Subject) == "" &&
		req.Year <= 0 &&
		req.Type <= 0 {
		writeError(w, http.StatusBadRequest, errors.New("fileId or at least one recommendation filter is required"))
		return
	}

	resp, err := h.service.RecommendFiles(requestContext(r), req)
	if err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) generateStudyPlan(w http.ResponseWriter, r *http.Request) {
	var req domain.StudyPlanRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if strings.TrimSpace(req.Goal) == "" && strings.TrimSpace(req.Subject) == "" {
		writeError(w, http.StatusBadRequest, errors.New("goal or subject is required"))
		return
	}

	resp, err := h.service.GenerateStudyPlan(requestContext(r), req)
	if err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) organizeFavorites(w http.ResponseWriter, r *http.Request) {
	var req domain.FavoriteOrganizeRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if req.UserID <= 0 {
		writeError(w, http.StatusBadRequest, errors.New("userId is required"))
		return
	}
	if req.GroupBy != "" && req.GroupBy != "subject" && req.GroupBy != "type" && req.GroupBy != "year" && req.GroupBy != "school" {
		writeError(w, http.StatusBadRequest, errors.New("groupBy must be subject, type, year, or school"))
		return
	}

	resp, err := h.service.OrganizeFavorites(requestContext(r), req)
	if err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) summarizeReports(w http.ResponseWriter, r *http.Request) {
	var req domain.ReportSummarizeRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if req.ReportID <= 0 && req.FileID <= 0 && req.UserID <= 0 && strings.TrimSpace(req.Result) == "" {
		writeError(w, http.StatusBadRequest, errors.New("reportId, fileId, userId, or result is required"))
		return
	}

	resp, err := h.service.SummarizeReports(requestContext(r), req)
	if err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) opsInsights(w http.ResponseWriter, r *http.Request) {
	var req domain.OpsInsightRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	resp, err := h.service.OpsInsights(requestContext(r), req)
	if err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) suggestMetadata(w http.ResponseWriter, r *http.Request) {
	var req domain.MetadataSuggestRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if strings.TrimSpace(req.FileName) == "" && strings.TrimSpace(req.OriginalName) == "" {
		writeError(w, http.StatusBadRequest, errors.New("fileName or originalName is required"))
		return
	}

	resp, err := h.service.SuggestMetadata(requestContext(r), req)
	if err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) suggestAudit(w http.ResponseWriter, r *http.Request) {
	var req domain.AuditSuggestRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	resp, err := h.service.SuggestAudit(requestContext(r), req)
	if err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func requestContext(r *http.Request) context.Context {
	return backend.WithAuthToken(r.Context(), r.Header.Get("Authorization"))
}

func readJSON(r *http.Request, target interface{}) error {
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(target)
}

func writeJSON(w http.ResponseWriter, status int, value interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]interface{}{
		"error": err.Error(),
		"code":  status,
	})
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
