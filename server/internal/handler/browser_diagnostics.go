package handler

import (
	"encoding/json"
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
	PublicAccess       bool                `json:"public_access"`
	AllowCustomTargets bool                `json:"allow_custom_targets"`
	PerTestTimeoutMS   int                 `json:"per_test_timeout_ms"`
	RateLimiting       bool                `json:"rate_limiting_enabled"`
	DefaultTargets     []browserDiagTarget `json:"default_targets"`
	TurnCredentialMode string              `json:"turn_credential_mode"`
	TurnWindowSeconds  int                 `json:"turn_transfer_window_seconds"`
	TurnPayloadBytes   int                 `json:"turn_payload_size_bytes"`
	TurnMessagesPerSec int                 `json:"turn_messages_per_second"`
	TurnQualityMin     float64             `json:"turn_quality_threshold_ratio"`
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
		DefaultTargets:     loadDefaultBrowserTargets(),
		TurnCredentialMode: resolveTurnCredentialMode(),
		TurnWindowSeconds:  envInt("BROWSER_DIAG_TURN_WINDOW_SECONDS", 10),
		TurnPayloadBytes:   envInt("BROWSER_DIAG_TURN_PAYLOAD_BYTES", 1024),
		TurnMessagesPerSec: envInt("BROWSER_DIAG_TURN_MESSAGES_PER_SEC", 20),
		TurnQualityMin:     envFloat("BROWSER_DIAG_TURN_QUALITY_THRESHOLD_RATIO", 0.90),
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

func loadDefaultBrowserTargets() []browserDiagTarget {
	targets := []browserDiagTarget{
		{
			TestType:         "HTTP",
			Label:            "Default HTTP",
			Value:            envOr("BROWSER_DIAG_HTTP_TARGET", "http://ipv6-diag.r.selvakn.in/diag"),
			Origin:           "default",
			EnabledByDefault: true,
		},
		{
			TestType:         "HTTPS",
			Label:            "Default HTTPS",
			Value:            envOr("BROWSER_DIAG_HTTPS_TARGET", "https://ipv6-diag.r.selvakn.in/diag"),
			Origin:           "default",
			EnabledByDefault: true,
		},
		{
			TestType:         "ICMP_EQUIV",
			Label:            "Default Reachability",
			Value:            envOr("BROWSER_DIAG_ICMP_TARGET", "https://ipv6-diag.r.selvakn.in/diag"),
			Origin:           "default",
			EnabledByDefault: true,
		},
		{
			TestType:         "STUN",
			Label:            "Default STUN",
			Value:            envOr("BROWSER_DIAG_STUN_TARGET", "stun:ipv6-diag.r.selvakn.in:3478"),
			Origin:           "default",
			EnabledByDefault: true,
		},
	}

	if turnTarget := strings.TrimSpace(envOr("BROWSER_DIAG_TURN_TARGET", "turn:ipv6-diag.r.selvakn.in:3478?transport=udp")); turnTarget != "" {
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
