package handler

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/lenovo/mesh/ipv6diag-server/internal/store"
)

type ReportsHandler struct {
	Store *store.ReportStore
}

func (h *ReportsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Route: /reports and /reports/{id}
	path := strings.TrimPrefix(r.URL.Path, "/reports")
	path = strings.TrimPrefix(path, "/")

	switch {
	case path == "" && r.Method == http.MethodPost:
		h.handleUpload(w, r)
	case path == "" && r.Method == http.MethodGet:
		h.handleList(w, r)
	case path != "" && r.Method == http.MethodGet:
		h.handleGet(w, r, path)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *ReportsHandler) handleUpload(w http.ResponseWriter, r *http.Request) {
	var req store.UploadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if req.SessionID == "" {
		writeError(w, http.StatusBadRequest, "missing session_id")
		return
	}
	if err := h.Store.Upsert(&req); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save report")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok", "id": req.SessionID})
}

func (h *ReportsHandler) handleList(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	filter := store.ListFilter{
		Device: q.Get("device"),
	}
	if v := q.Get("from"); v != "" {
		t, err := time.Parse("2006-01-02", v)
		if err == nil {
			filter.From = &t
		}
	}
	if v := q.Get("to"); v != "" {
		t, err := time.Parse("2006-01-02", v)
		if err == nil {
			filter.To = &t
		}
	}

	reports, total, err := h.Store.List(filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "query failed")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"reports": reports, "total": total})
}

func (h *ReportsHandler) handleGet(w http.ResponseWriter, _ *http.Request, id string) {
	detail, err := h.Store.GetByID(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "query failed")
		return
	}
	if detail == nil {
		writeError(w, http.StatusNotFound, "report not found")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(detail)
}
