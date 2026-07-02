package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBrowserDiagnosticsPageHandlerMethod(t *testing.T) {
	h := &BrowserDiagnosticsPageHandler{}
	req := httptest.NewRequest(http.MethodPost, "/browser-diagnostics", nil)
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rr.Code)
	}
}

func TestBrowserDiagnosticsConfigHandlerMethod(t *testing.T) {
	h := &BrowserDiagnosticsConfigHandler{}
	req := httptest.NewRequest(http.MethodPost, "/browser-diagnostics/config", nil)
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rr.Code)
	}
}

func TestBrowserDiagnosticsConfigHandlerPayload(t *testing.T) {
	t.Setenv("BROWSER_DIAG_ALLOW_CUSTOM_TARGETS", "true")
	t.Setenv("BROWSER_DIAG_PER_TEST_TIMEOUT_MS", "20000")
	t.Setenv("TURN_ENABLED", "true")
	t.Setenv("TURN_CREDENTIALS_TOKEN", "")
	t.Setenv("BROWSER_DIAG_HTTP_TARGET", "http://localhost:8080/diag")
	t.Setenv("BROWSER_DIAG_TURN_WINDOW_SECONDS", "10")
	t.Setenv("BROWSER_DIAG_TURN_PAYLOAD_BYTES", "1200")
	t.Setenv("BROWSER_DIAG_TURN_MESSAGES_PER_SEC", "30")
	t.Setenv("BROWSER_DIAG_TURN_QUALITY_THRESHOLD_RATIO", "0.85")

	h := &BrowserDiagnosticsConfigHandler{}
	req := httptest.NewRequest(http.MethodGet, "/browser-diagnostics/config", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	var payload browserDiagConfigResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatalf("invalid payload: %v", err)
	}
	if payload.PerTestTimeoutMS != 20000 {
		t.Fatalf("expected timeout 20000, got %d", payload.PerTestTimeoutMS)
	}
	if payload.TurnCredentialMode != "tokenless_endpoint" {
		t.Fatalf("expected tokenless_endpoint mode, got %s", payload.TurnCredentialMode)
	}
	if len(payload.DefaultTargets) == 0 {
		t.Fatalf("expected default targets")
	}
	if payload.DefaultTargets[0].Value != "http://localhost:8080/diag" {
		t.Fatalf("unexpected default HTTP target: %s", payload.DefaultTargets[0].Value)
	}
	if payload.TurnWindowSeconds != 10 {
		t.Fatalf("expected turn window 10, got %d", payload.TurnWindowSeconds)
	}
	if payload.TurnPayloadBytes != 1200 {
		t.Fatalf("expected turn payload 1200, got %d", payload.TurnPayloadBytes)
	}
	if payload.TurnMessagesPerSec != 30 {
		t.Fatalf("expected turn messages/sec 30, got %d", payload.TurnMessagesPerSec)
	}
	if payload.TurnQualityMin != 0.85 {
		t.Fatalf("expected turn quality threshold 0.85, got %f", payload.TurnQualityMin)
	}
}

func TestBrowserDiagnosticsConfigDefaultStunTurnTargets(t *testing.T) {
	t.Setenv("TURN_ENABLED", "true")
	t.Setenv("TURN_CREDENTIALS_TOKEN", "")
	t.Setenv("BROWSER_DIAG_HTTP_TARGET", "")
	t.Setenv("BROWSER_DIAG_HTTPS_TARGET", "")
	t.Setenv("BROWSER_DIAG_ICMP_TARGET", "")
	t.Setenv("BROWSER_DIAG_STUN_TARGET", "")
	t.Setenv("BROWSER_DIAG_TURN_TARGET", "")

	h := &BrowserDiagnosticsConfigHandler{}
	req := httptest.NewRequest(http.MethodGet, "/browser-diagnostics/config", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	var payload browserDiagConfigResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatalf("invalid payload: %v", err)
	}

	seen := map[string]string{}
	for _, t := range payload.DefaultTargets {
		seen[t.TestType] = t.Value
	}

	if got := seen["STUN"]; got != "stun:ipv6-diag.r.selvakn.in:3478" {
		t.Fatalf("unexpected STUN default: %s", got)
	}
	if got := seen["TURN"]; got != "turn:ipv6-diag.r.selvakn.in:3478?transport=udp" {
		t.Fatalf("unexpected TURN default: %s", got)
	}
}
