package turn

import (
	"strings"
	"testing"
	"time"
)

func TestActiveURIsIPv6First(t *testing.T) {
	svc := NewService(Config{}, NewCredentialManager("realm", 5*time.Minute))
	svc.statuses["udp6"] = ListenerStatus{Key: "udp6", State: "active"}
	svc.statuses["tcp6"] = ListenerStatus{Key: "tcp6", State: "active"}
	svc.statuses["udp4"] = ListenerStatus{Key: "udp4", State: "active"}
	svc.statuses["tcp4"] = ListenerStatus{Key: "tcp4", State: "active"}

	uris := svc.ActiveURIs("example.com:3478")
	if len(uris) != 4 {
		t.Fatalf("expected 4 uris, got %d", len(uris))
	}
	if !strings.Contains(uris[0], "transport=udp") {
		t.Fatalf("expected udp first, got %s", uris[0])
	}
}

func TestRelayAddressFallback(t *testing.T) {
	if got := relayAddress("0.0.0.0:3478", "fallback"); got != "0.0.0.0" {
		t.Fatalf("unexpected relay address: %s", got)
	}
	if got := relayAddress("bad-address", "fallback"); got != "fallback" {
		t.Fatalf("expected fallback, got %s", got)
	}
}
