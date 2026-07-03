package handler

import (
	"encoding/json"
	"net"
	"net/http"
	"strings"
)

type MyIPHandler struct{}

type myIPResponse struct {
	IP     string `json:"ip"`
	Family string `json:"family"`
}

func (h *MyIPHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	ip := resolveClientIP(r)
	family := "IPv4"
	if strings.Contains(ip, ":") {
		family = "IPv6"
	}

	w.Header().Set("Content-Type", "application/json")
	// Allow cross-origin fetches so the browser can call this endpoint
	// from pages served at a different origin (e.g. via a literal IP URL).
	w.Header().Set("Access-Control-Allow-Origin", "*")
	json.NewEncoder(w).Encode(myIPResponse{IP: ip, Family: family})
}

// resolveClientIP returns the best-effort client IP. It honours
// X-Real-IP and the leftmost entry of X-Forwarded-For when present
// (typical reverse-proxy deployments), otherwise falls back to the
// TCP remote address.
func resolveClientIP(r *http.Request) string {
	if v := strings.TrimSpace(r.Header.Get("X-Real-IP")); v != "" {
		if parsed := net.ParseIP(v); parsed != nil {
			return parsed.String()
		}
	}
	if v := r.Header.Get("X-Forwarded-For"); v != "" {
		first := strings.TrimSpace(strings.SplitN(v, ",", 2)[0])
		if parsed := net.ParseIP(first); parsed != nil {
			return parsed.String()
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	if parsed := net.ParseIP(host); parsed != nil {
		return parsed.String()
	}
	return host
}
