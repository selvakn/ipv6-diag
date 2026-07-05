package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	turnsvc "github.com/selvakn/ipv6diag-server/internal/turn"
)

func TestTurnCredentialsHandlerUnauthorized(t *testing.T) {
	h := &TurnCredentialsHandler{
		Token:       "secret",
		Credentials: turnsvc.NewCredentialManager("realm", 5*time.Minute),
		Service:     &turnsvc.Service{},
	}

	req := httptest.NewRequest(http.MethodGet, "/turn/credentials", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
}

func TestTurnCredentialsHandlerMethod(t *testing.T) {
	h := &TurnCredentialsHandler{}
	req := httptest.NewRequest(http.MethodPost, "/turn/credentials", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rr.Code)
	}
}

func TestTurnCredentialsHandlerNoListeners(t *testing.T) {
	cfg := turnsvc.Config{
		Enabled:       true,
		Realm:         "realm",
		CredentialTTL: 5 * time.Minute,
	}
	service := turnsvc.NewService(cfg, turnsvc.NewCredentialManager("realm", 5*time.Minute), nil)

	h := &TurnCredentialsHandler{
		Credentials: turnsvc.NewCredentialManager("realm", 5*time.Minute),
		Service:     service,
	}

	req := httptest.NewRequest(http.MethodGet, "/turn/credentials", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rr.Code)
	}
}

func TestTurnCredentialsResponseSchema(t *testing.T) {
	resp := turnCredentialsResponse{
		Username:   "u",
		Password:   "p",
		Realm:      "r",
		TTLSeconds: 300,
		ExpiresAt:  time.Now().UTC().Format(time.RFC3339),
		URIs:       []string{"turn:127.0.0.1:3478?transport=udp"},
	}
	raw, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	if len(raw) == 0 {
		t.Fatalf("marshal returned empty body")
	}
}

func TestHostWithoutPort(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{in: "localhost:6080", want: "localhost"},
		{in: "127.0.0.1:8080", want: "127.0.0.1"},
		{in: "[::1]:8080", want: "::1"},
		{in: "example.com", want: "example.com"},
		{in: "", want: ""},
	}
	for _, tc := range cases {
		if got := hostWithoutPort(tc.in); got != tc.want {
			t.Fatalf("hostWithoutPort(%q)=%q want=%q", tc.in, got, tc.want)
		}
	}
}
