package handler

import (
	"net/http"

	"github.com/lenovo/mesh/ipv6diag-server/web"
)

type DashboardHandler struct{}

func (h *DashboardHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(web.DashboardHTML)
}
