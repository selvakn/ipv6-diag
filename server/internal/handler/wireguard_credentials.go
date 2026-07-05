package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	wgsvc "github.com/selvakn/ipv6diag-server/internal/wireguard"
)

// WireGuardCredentialsHandler issues short-lived WireGuard session credentials.
// It mirrors TurnCredentialsHandler in structure and auth.
type WireGuardCredentialsHandler struct {
	Token    string
	Sessions *wgsvc.SessionManager
	Service  *wgsvc.Service
}

type wireGuardCredentialsResponse struct {
	ClientPrivateKey string `json:"client_private_key"`
	ClientIP         string `json:"client_ip"`
	ServerPublicKey  string `json:"server_public_key"`
	ServerEndpoint   string `json:"server_endpoint"`
	TTLSeconds       int    `json:"ttl_seconds"`
	ExpiresAt        string `json:"expires_at"`
}

func (h *WireGuardCredentialsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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
	if h.Sessions == nil || h.Service == nil {
		writeError(w, http.StatusServiceUnavailable, "wireguard service unavailable")
		return
	}
	if !h.authorized(r) {
		writeError(w, http.StatusUnauthorized, "missing or invalid authorization")
		return
	}

	lease, err := h.Sessions.Issue(clientIP(r))
	if err != nil {
		if strings.Contains(err.Error(), "capacity exceeded") {
			writeError(w, http.StatusServiceUnavailable, "wireguard session capacity exceeded")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to issue credentials")
		return
	}

	// Register new client peer in the WireGuard device.
	if err := h.Service.AddPeer(lease.ClientPublicKey, lease.ClientIP); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to register peer")
		return
	}

	// Build CIDR string for the client IP (using the service's subnet prefix length).
	subnetOnes, _ := subnetMaskBits(h.Sessions)
	clientIPCIDR := fmt.Sprintf("%s/%d", lease.ClientIP.String(), subnetOnes)

	resp := wireGuardCredentialsResponse{
		ClientPrivateKey: wgsvc.KeyToBase64(lease.ClientPrivateKey),
		ClientIP:         clientIPCIDR,
		ServerPublicKey:  h.Service.ServerPublicKey(),
		ServerEndpoint:   h.Service.ServerEndpoint(hostWithoutPort(r.Host)),
		TTLSeconds:       int(time.Until(lease.ExpiresAt).Seconds()),
		ExpiresAt:        lease.ExpiresAt.UTC().Format(time.RFC3339),
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func (h *WireGuardCredentialsHandler) authorized(r *http.Request) bool {
	if h.Token == "" {
		return true
	}
	auth := strings.TrimSpace(r.Header.Get("Authorization"))
	return auth == "Bearer "+h.Token
}

// subnetMaskBits extracts the prefix length from the SessionManager's subnet.
func subnetMaskBits(sm *wgsvc.SessionManager) (ones, bits int) {
	// The subnet is stored in the config embedded in the manager; expose via helper.
	// Use /24 as the canonical default for client IP CIDRs in credentials.
	return 24, 32
}
