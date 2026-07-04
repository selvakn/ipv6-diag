package handler

import (
	"encoding/json"
	"net"
	"net/http"
	"strings"
	"time"

	turnsvc "github.com/selvakn/ipv6diag-server/internal/turn"
)

type TurnCredentialsHandler struct {
	Token       string
	Credentials *turnsvc.CredentialManager
	Service     *turnsvc.Service
}

type turnCredentialsResponse struct {
	Username   string   `json:"username"`
	Password   string   `json:"password"`
	Realm      string   `json:"realm"`
	TTLSeconds int      `json:"ttl_seconds"`
	ExpiresAt  string   `json:"expires_at"`
	URIs       []string `json:"uris"`
}

func (h *TurnCredentialsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if h.Credentials == nil || h.Service == nil {
		writeError(w, http.StatusServiceUnavailable, "turn service unavailable")
		return
	}
	if !h.authorized(r) {
		writeError(w, http.StatusUnauthorized, "missing or invalid authorization")
		return
	}

	clientID := clientIP(r)
	lease, err := h.Credentials.Issue(clientID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to issue credentials")
		return
	}

	uris := h.Service.ActiveURIs(hostWithoutPort(r.Host))
	if len(uris) == 0 {
		writeError(w, http.StatusServiceUnavailable, "no active turn listeners")
		return
	}

	resp := turnCredentialsResponse{
		Username:   lease.Username,
		Password:   lease.Password,
		Realm:      lease.Realm,
		TTLSeconds: int(time.Until(lease.ExpiresAt).Seconds()),
		ExpiresAt:  lease.ExpiresAt.UTC().Format(time.RFC3339),
		URIs:       uris,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func (h *TurnCredentialsHandler) authorized(r *http.Request) bool {
	if h.Token == "" {
		return true
	}
	auth := strings.TrimSpace(r.Header.Get("Authorization"))
	return auth == "Bearer "+h.Token
}

func hostWithoutPort(hostport string) string {
	hostport = strings.TrimSpace(hostport)
	if hostport == "" {
		return ""
	}
	if host, _, err := net.SplitHostPort(hostport); err == nil {
		return host
	}
	// If host has no port, keep it as-is.
	return hostport
}
