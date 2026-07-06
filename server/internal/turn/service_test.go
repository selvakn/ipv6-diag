package turn

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/pion/dtls/v3"
)

// selfSignedTLSConfig generates a throw-away self-signed cert and returns a
// *tls.Config that serves it. Used to exercise DTLS listener startup in tests.
func selfSignedTLSConfig(t *testing.T) *tls.Config {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "test"},
		NotBefore:    time.Now().Add(-time.Minute),
		NotAfter:     time.Now().Add(time.Hour),
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &priv.PublicKey, priv)
	if err != nil {
		t.Fatal(err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	kb, _ := x509.MarshalECPrivateKey(priv)
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: kb})
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		t.Fatal(err)
	}
	return &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12}
}

// freeDTLSPort finds an available UDP port by briefly binding and releasing it.
func freeDTLSPort(t *testing.T, network string) string {
	t.Helper()
	conn, err := net.ListenPacket(network, "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := conn.LocalAddr().String()
	conn.Close()
	return addr
}

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

func TestDTLSConfigFromTLS_Nil(t *testing.T) {
	if dtlsConfigFromTLS(nil) != nil {
		t.Fatal("expected nil dtls config for nil tls config")
	}
}

func TestDTLSConfigFromTLS_BridgesGetCertificate(t *testing.T) {
	called := false
	tlsCfg := &tls.Config{
		GetCertificate: func(info *tls.ClientHelloInfo) (*tls.Certificate, error) {
			called = true
			if info.ServerName != "example.com" {
				return nil, fmt.Errorf("unexpected SNI: %s", info.ServerName)
			}
			return &tls.Certificate{}, nil
		},
	}
	dtlsCfg := dtlsConfigFromTLS(tlsCfg)
	if dtlsCfg == nil {
		t.Fatal("expected non-nil dtls config")
	}
	if dtlsCfg.GetCertificate == nil {
		t.Fatal("expected GetCertificate to be set")
	}
	cert, err := dtlsCfg.GetCertificate(&dtls.ClientHelloInfo{ServerName: "example.com"})
	if err != nil {
		t.Fatalf("GetCertificate returned error: %v", err)
	}
	if cert == nil {
		t.Fatal("expected non-nil certificate")
	}
	if !called {
		t.Fatal("tls GetCertificate was not called")
	}
}

func TestDTLSConfigFromTLS_FallsBackToCertificates(t *testing.T) {
	tlsCfg := &tls.Config{
		Certificates: []tls.Certificate{{}, {}},
	}
	dtlsCfg := dtlsConfigFromTLS(tlsCfg)
	if dtlsCfg == nil {
		t.Fatal("expected non-nil dtls config")
	}
	if len(dtlsCfg.Certificates) != 2 {
		t.Fatalf("expected 2 certificates, got %d", len(dtlsCfg.Certificates))
	}
}

func TestActiveURIsDTLS(t *testing.T) {
	svc := NewService(Config{
		DTLS4Addr: "0.0.0.0:5349",
		DTLS6Addr: "[::]:5349",
	}, NewCredentialManager("realm", 5*time.Minute), nil)

	svc.statuses["dtls6"] = ListenerStatus{Key: "dtls6", State: "active"}
	svc.statuses["dtls4"] = ListenerStatus{Key: "dtls4", State: "active"}

	uris := svc.ActiveURIs("example.com")
	if len(uris) != 2 {
		t.Fatalf("expected 2 uris, got %d: %v", len(uris), uris)
	}
	for _, u := range uris {
		if !strings.HasPrefix(u, "turns:") {
			t.Errorf("DTLS URI must use turns: scheme, got %s", u)
		}
		if !strings.Contains(u, "transport=udp") {
			t.Errorf("DTLS URI must have transport=udp, got %s", u)
		}
		if !strings.Contains(u, "5349") {
			t.Errorf("DTLS URI must include port 5349, got %s", u)
		}
	}
	// IPv6-first
	if !strings.Contains(uris[0], "[example.com]") && !strings.Contains(uris[0], "example.com") {
		t.Errorf("unexpected host in URI: %s", uris[0])
	}
}

func TestActiveURIsEncryptedBeforePlain(t *testing.T) {
	// When TLS, DTLS, UDP, and TCP are all active, encrypted URIs (turns:)
	// must appear before plain (turn:) within each IP family.
	svc := NewService(Config{
		TLS4Addr:  "0.0.0.0:5349",
		DTLS4Addr: "0.0.0.0:5349",
	}, NewCredentialManager("realm", 5*time.Minute), nil)

	svc.statuses["tls4"]  = ListenerStatus{Key: "tls4", State: "active"}
	svc.statuses["dtls4"] = ListenerStatus{Key: "dtls4", State: "active"}
	svc.statuses["udp4"]  = ListenerStatus{Key: "udp4", State: "active"}
	svc.statuses["tcp4"]  = ListenerStatus{Key: "tcp4", State: "active"}

	uris := svc.ActiveURIs("example.com")
	if len(uris) != 4 {
		t.Fatalf("expected 4 uris, got %d: %v", len(uris), uris)
	}
	// First two must be turns:, last two must be turn:
	for i, u := range uris[:2] {
		if !strings.HasPrefix(u, "turns:") {
			t.Errorf("uri[%d] expected turns: scheme, got %s", i, u)
		}
	}
	for i, u := range uris[2:] {
		if !strings.HasPrefix(u, "turn:") {
			t.Errorf("uri[%d] expected turn: scheme, got %s", i+2, u)
		}
	}
	// TLS/TCP before DTLS/UDP among encrypted
	if !strings.Contains(uris[0], "transport=tcp") {
		t.Errorf("first encrypted URI should be TLS/TCP, got %s", uris[0])
	}
	if !strings.Contains(uris[1], "transport=udp") {
		t.Errorf("second encrypted URI should be DTLS/UDP, got %s", uris[1])
	}
}

func TestDTLSListenerBinds(t *testing.T) {
	tlsCfg := selfSignedTLSConfig(t)
	dtls4Addr := freeDTLSPort(t, "udp4")

	cfg := Config{
		Enabled:   true,
		Realm:     "test",
		DTLS4Addr: dtls4Addr,
	}
	svc := NewService(cfg, NewCredentialManager("test", time.Minute), tlsCfg)
	if err := svc.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer svc.Stop()

	statuses := svc.Statuses()
	var dtls4Status string
	for _, st := range statuses {
		if st.Key == "dtls4" {
			dtls4Status = st.State
			if st.State == "degraded" {
				t.Errorf("dtls4 degraded: %s", st.Error)
			}
		}
	}
	if dtls4Status == "" {
		t.Error("dtls4 status not found")
	}
}
