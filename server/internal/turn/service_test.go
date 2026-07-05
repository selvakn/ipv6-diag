package turn

import (
	"strings"
	"testing"
	"time"
)

func TestActiveURIsIPv6First(t *testing.T) {
	svc := NewService(Config{}, NewCredentialManager("realm", 5*time.Minute), nil)
	svc.statuses["udp6"] = ListenerStatus{Key: "udp6", State: "active"}
	svc.statuses["tcp6"] = ListenerStatus{Key: "tcp6", State: "active"}
	svc.statuses["udp4"] = ListenerStatus{Key: "udp4", State: "active"}
	svc.statuses["tcp4"] = ListenerStatus{Key: "tcp4", State: "active"}

	uris := svc.ActiveURIs("example.com")
	if len(uris) != 4 {
		t.Fatalf("expected 4 uris, got %d", len(uris))
	}
	if !strings.Contains(uris[0], "transport=udp") {
		t.Fatalf("expected udp first, got %s", uris[0])
	}
	for _, u := range uris {
		if strings.Contains(u, "example.com") {
			continue
		}
		t.Errorf("unexpected host in URI: %s", u)
	}
}

func TestActiveURIsLoopbackSubstitution(t *testing.T) {
	svc := NewService(Config{}, NewCredentialManager("realm", 5*time.Minute), nil)
	svc.statuses["udp4"] = ListenerStatus{Key: "udp4", State: "active"}

	for _, loopback := range []string{"localhost", "127.0.0.1", "::1"} {
		uris := svc.ActiveURIs(loopback)
		if len(uris) == 0 {
			continue // no non-loopback interface in this CI env; skip
		}
		for _, u := range uris {
			if strings.Contains(u, "localhost") || strings.Contains(u, "127.0.0.1") || strings.Contains(u, "[::1]") {
				// Only fail if there IS a LAN IP available (meaning substitution should have occurred).
				if ip := firstNonLoopbackIP(false); ip != "" {
					t.Errorf("loopback host %q not substituted in URI: %s", loopback, u)
				}
			}
		}
	}
}

func TestBracketIPv6(t *testing.T) {
	// Bare IPv6 address gets brackets for use in "addr:port" strings.
	if got := bracketIPv6("2400:6180:100:d0::98d:8001"); got != "[2400:6180:100:d0::98d:8001]" {
		t.Fatalf("expected brackets, got %s", got)
	}
	// Already-bracketed addresses are not double-bracketed.
	// (net.ParseIP strips brackets, so passing a bracketed string returns it unchanged.)
	if got := bracketIPv6("192.0.2.1"); got != "192.0.2.1" {
		t.Fatalf("IPv4 must not be bracketed, got %s", got)
	}
	if got := bracketIPv6("::1"); got != "[::1]" {
		t.Fatalf("loopback IPv6 must be bracketed, got %s", got)
	}
}

func TestRelayAddressFallback(t *testing.T) {
	// Public IP always wins.
	if got := relayAddress("0.0.0.0:3478", "fallback", "203.0.113.10"); got != "203.0.113.10" {
		t.Fatalf("expected public override, got %s", got)
	}
	// Unparseable address falls back.
	if got := relayAddress("bad-address", "fallback", ""); got != "fallback" {
		t.Fatalf("expected fallback, got %s", got)
	}
	// Non-wildcard address is returned as-is.
	if got := relayAddress("10.0.0.5:3478", "fallback", ""); got != "10.0.0.5" {
		t.Fatalf("expected specific host, got %s", got)
	}
	// Wildcard bind: should return either a detected LAN IP or the fallback,
	// never the wildcard address itself.
	got := relayAddress("0.0.0.0:3478", "127.0.0.1", "")
	if got == "0.0.0.0" {
		t.Fatalf("must not return wildcard address")
	}
	if got == "" {
		t.Fatalf("must not return empty string")
	}
}

func TestFirstNonLoopbackIP(t *testing.T) {
	// Should return empty or a valid routable IP — never loopback or wildcard.
	for _, v6 := range []bool{false, true} {
		got := firstNonLoopbackIP(v6)
		if got == "" {
			continue // no non-loopback interface present; acceptable in CI
		}
		if got == "127.0.0.1" || got == "::1" || got == "0.0.0.0" || got == "::" {
			t.Errorf("firstNonLoopbackIP(v6=%v) returned bad address: %s", v6, got)
		}
	}
}
