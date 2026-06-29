package handler

import (
	"encoding/json"
	"net"
	"net/http"
	"strings"
	"time"
)

type diagResponse struct {
	ServerAddress string `json:"server_address"`
	ClientAddress string `json:"client_address"`
	AddressFamily string `json:"address_family"`
	Protocol      string `json:"protocol"`
	Timestamp     string `json:"timestamp"`
}

// DiagHandler serves GET /diag. IsTLS distinguishes HTTP from HTTPS listeners.
// Address family is derived from the local address of the accepted connection,
// which is unambiguous because we bind IPv4 and IPv6 on separate listeners.
type DiagHandler struct {
	IsTLS bool
}

func (h *DiagHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	localAddr := localIP(r)
	clientAddr := clientIP(r)

	proto := "HTTP"
	if h.IsTLS {
		proto = "HTTPS"
	}

	resp := diagResponse{
		ServerAddress: localAddr,
		ClientAddress: clientAddr,
		AddressFamily: classifyFamily(localAddr),
		Protocol:      proto,
		Timestamp:     time.Now().UTC().Format(time.RFC3339),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// localIP extracts the host part of the server's local address from the request context.
func localIP(r *http.Request) string {
	lc := r.Context().Value(http.LocalAddrContextKey)
	if lc == nil {
		return "unknown"
	}
	host, _, err := net.SplitHostPort(lc.(net.Addr).String())
	if err != nil {
		return lc.(net.Addr).String()
	}
	return host
}

// remoteIP extracts the host part of a host:port string.
func remoteIP(addr string) string {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return addr
	}
	return host
}

// clientIP resolves the end-client IP in reverse-proxy environments.
//
// Fly.io sets Fly-Client-IP and X-Forwarded-For. Prefer Fly-Client-IP for
// direct Fly edge traffic; fall back to the left-most valid X-Forwarded-For IP
// (original client), then socket remote address.
func clientIP(r *http.Request) string {
	if v := strings.TrimSpace(r.Header.Get("Fly-Client-IP")); v != "" && net.ParseIP(v) != nil {
		return v
	}

	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		for _, part := range parts {
			candidate := strings.TrimSpace(part)
			if net.ParseIP(candidate) != nil {
				return candidate
			}
		}
	}

	return remoteIP(r.RemoteAddr)
}

// classifyFamily returns "IPv4" for IPv4 addresses (including ::ffff: mapped ones)
// and "IPv6" for all other addresses.
func classifyFamily(addr string) string {
	ip := net.ParseIP(addr)
	if ip == nil {
		return "unknown"
	}
	if ip.To4() != nil {
		return "IPv4"
	}
	if strings.HasPrefix(addr, "::ffff:") {
		return "IPv4"
	}
	return "IPv6"
}

func writeError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
