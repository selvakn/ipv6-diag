package handler

import (
	"encoding/json"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/selvakn/ipv6diag-server/web"
)

type BrowserDiagnosticsPageHandler struct{}

func (h *BrowserDiagnosticsPageHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(web.BrowserDiagnosticsHTML)
}

type BrowserDiagnosticsConfigHandler struct{}

type browserDiagTarget struct {
	TestType         string `json:"test_type"`
	Label            string `json:"label"`
	Value            string `json:"value"`
	Origin           string `json:"origin"`
	EnabledByDefault bool   `json:"enabled_by_default"`
}

type browserDiagConfigResponse struct {
	PublicAccess           bool                `json:"public_access"`
	AllowCustomTargets     bool                `json:"allow_custom_targets"`
	PerTestTimeoutMS       int                 `json:"per_test_timeout_ms"`
	RateLimiting           bool                `json:"rate_limiting_enabled"`
	DefaultTargets         []browserDiagTarget `json:"default_targets"`
	TurnCredentialMode     string              `json:"turn_credential_mode"`
	TurnWindowSeconds      int                 `json:"turn_transfer_window_seconds"`
	TurnPayloadBytes       int                 `json:"turn_payload_size_bytes"`
	TurnMessagesPerSec     int                 `json:"turn_messages_per_second"`
	TurnQualityMin         float64             `json:"turn_quality_threshold_ratio"`
	IPDetectV4URL          string              `json:"ip_detect_v4_url"`
	IPDetectV6URL          string              `json:"ip_detect_v6_url"`
	WireGuardEnabled       bool                `json:"wireguard_enabled"`
	WireGuardEchoPort      int                 `json:"wireguard_echo_port"`
	WireGuardWindowSeconds int                 `json:"wireguard_transfer_window_seconds"`
	WireGuardPayloadBytes  int                 `json:"wireguard_payload_size_bytes"`
}

func (h *BrowserDiagnosticsConfigHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	cfg := browserDiagConfigResponse{
		PublicAccess:       true,
		AllowCustomTargets: envBool("BROWSER_DIAG_ALLOW_CUSTOM_TARGETS", true),
		PerTestTimeoutMS:   envInt("BROWSER_DIAG_PER_TEST_TIMEOUT_MS", 15000),
		RateLimiting:       false,
		DefaultTargets:     loadDefaultBrowserTargets(r),
		TurnCredentialMode: resolveTurnCredentialMode(),
		TurnWindowSeconds:  envInt("BROWSER_DIAG_TURN_WINDOW_SECONDS", 10),
		TurnPayloadBytes:   envInt("BROWSER_DIAG_TURN_PAYLOAD_BYTES", 16000),
		TurnMessagesPerSec: envInt("BROWSER_DIAG_TURN_MESSAGES_PER_SEC", 20),
		TurnQualityMin:         envFloat("BROWSER_DIAG_TURN_QUALITY_THRESHOLD_RATIO", 0.90),
		IPDetectV4URL:          envOr("BROWSER_DIAG_IP_DETECT_V4_URL", "https://4.ipv6-diag.selvakn.in/my-ip"),
		IPDetectV6URL:          envOr("BROWSER_DIAG_IP_DETECT_V6_URL", "https://6.ipv6-diag.selvakn.in/my-ip"),
		WireGuardEnabled:       envBool("WG_ENABLED", false),
		WireGuardEchoPort:      envInt("WG_ECHO_PORT", 7000),
		WireGuardWindowSeconds: envInt("WG_TRANSFER_WINDOW_SECONDS", 10),
		WireGuardPayloadBytes:  envInt("WG_PAYLOAD_SIZE_BYTES", 1024),
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(cfg); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to encode response")
	}
}

func resolveTurnCredentialMode() string {
	if !envBool("TURN_ENABLED", false) {
		return "none"
	}
	if strings.TrimSpace(os.Getenv("TURN_CREDENTIALS_TOKEN")) == "" {
		return "tokenless_endpoint"
	}
	return "token_required"
}

func loadDefaultBrowserTargets(r *http.Request) []browserDiagTarget {
	self := selfBase(r) // e.g. "http://192.168.15.10:6080"
	selfHost := selfHostOnly(r) // e.g. "192.168.15.10" (no port, for STUN/TURN URIs)

	targets := []browserDiagTarget{
		{
			TestType:         "HTTP",
			Label:            "Default HTTP",
			Value:            envOr("BROWSER_DIAG_HTTP_TARGET", self+"/diag"),
			Origin:           "default",
			EnabledByDefault: true,
		},
		{
			TestType:         "HTTPS",
			Label:            "Default HTTPS",
			Value:            envOr("BROWSER_DIAG_HTTPS_TARGET", self+"/diag"),
			Origin:           "default",
			EnabledByDefault: true,
		},
		{
			TestType:         "ICMP_EQUIV",
			Label:            "Default Reachability",
			Value:            envOr("BROWSER_DIAG_ICMP_TARGET", self+"/diag"),
			Origin:           "default",
			EnabledByDefault: true,
		},
		{
			TestType:         "STUN",
			Label:            "Default STUN",
			Value:            envOr("BROWSER_DIAG_STUN_TARGET", "stun:"+selfHost+":3478"),
			Origin:           "default",
			EnabledByDefault: true,
		},
	}

	if turnTarget := strings.TrimSpace(envOr("BROWSER_DIAG_TURN_TARGET", "turn:"+selfHost+":3478?transport=udp")); turnTarget != "" {
		targets = append(targets, browserDiagTarget{
			TestType:         "TURN",
			Label:            "Default TURN",
			Value:            turnTarget,
			Origin:           "default",
			EnabledByDefault: true,
		})
	}

	return targets
}

// selfBase returns the scheme+host of the current request for building self-referencing URLs.
// scheme is http unless TLS is active or X-Forwarded-Proto says https.
func selfBase(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") {
		scheme = "https"
	}
	return scheme + "://" + r.Host
}

// selfHostOnly returns the bare hostname (no port) from the request Host header.
// Used for STUN/TURN URIs which carry their own port numbers.
func selfHostOnly(r *http.Request) string {
	host := r.Host
	if h, _, err := net.SplitHostPort(host); err == nil {
		return h
	}
	return host
}

func envBool(name string, fallback bool) bool {
	v := strings.TrimSpace(strings.ToLower(os.Getenv(name)))
	if v == "" {
		return fallback
	}
	return v == "1" || v == "true" || v == "yes" || v == "on"
}

func envInt(name string, fallback int) int {
	v := strings.TrimSpace(os.Getenv(name))
	if v == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(v)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func envOr(name, fallback string) string {
	v := strings.TrimSpace(os.Getenv(name))
	if v == "" {
		return fallback
	}
	return v
}

func envFloat(name string, fallback float64) float64 {
	v := strings.TrimSpace(os.Getenv(name))
	if v == "" {
		return fallback
	}
	parsed, err := strconv.ParseFloat(v, 64)
	if err != nil || parsed <= 0 || parsed > 1 {
		return fallback
	}
	return parsed
}
