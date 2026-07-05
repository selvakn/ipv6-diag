package tlsutil

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"os"
	"testing"
	"time"
)

// makeCert generates a self-signed cert for the given DNS names and writes PEM
// files to certPath / keyPath. Returns the DER bytes for identity comparison.
func makeCert(t *testing.T, certPath, keyPath string, dnsNames []string) []byte {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject:      pkix.Name{CommonName: dnsNames[0]},
		DNSNames:     dnsNames,
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
		NotBefore:    time.Now().Add(-time.Minute),
		NotAfter:     time.Now().Add(time.Hour),
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &priv.PublicKey, priv)
	if err != nil {
		t.Fatal(err)
	}

	cf, _ := os.Create(certPath)
	_ = pem.Encode(cf, &pem.Block{Type: "CERTIFICATE", Bytes: der})
	cf.Close()

	kf, _ := os.Create(keyPath)
	kb, _ := x509.MarshalECPrivateKey(priv)
	_ = pem.Encode(kf, &pem.Block{Type: "EC PRIVATE KEY", Bytes: kb})
	kf.Close()

	return der
}

func TestNewFileConfig_LoadsInitialCert(t *testing.T) {
	dir := t.TempDir()
	makeCert(t, dir+"/cert.pem", dir+"/key.pem", []string{"example.com"})

	cfg, err := NewFileConfig(dir+"/cert.pem", dir+"/key.pem")
	if err != nil {
		t.Fatalf("NewFileConfig: %v", err)
	}
	cert, err := cfg.GetCertificate(&tls.ClientHelloInfo{ServerName: "example.com"})
	if err != nil {
		t.Fatalf("GetCertificate: %v", err)
	}
	if cert == nil {
		t.Fatal("expected non-nil certificate")
	}
}

func TestNewFileConfig_ReloadsOnFileChange(t *testing.T) {
	dir := t.TempDir()
	makeCert(t, dir+"/cert.pem", dir+"/key.pem", []string{"example.com"})

	cfg, err := NewFileConfig(dir+"/cert.pem", dir+"/key.pem")
	if err != nil {
		t.Fatalf("NewFileConfig: %v", err)
	}
	cert1, _ := cfg.GetCertificate(&tls.ClientHelloInfo{ServerName: "example.com"})

	time.Sleep(10 * time.Millisecond)
	makeCert(t, dir+"/cert.pem", dir+"/key.pem", []string{"example.com"})

	cert2, err := cfg.GetCertificate(&tls.ClientHelloInfo{ServerName: "example.com"})
	if err != nil {
		t.Fatalf("GetCertificate after renewal: %v", err)
	}
	// The renewed cert has a new serial (time-based), so leaf differs.
	if cert1.Leaf.SerialNumber.Cmp(cert2.Leaf.SerialNumber) == 0 {
		t.Fatal("expected certificate to be reloaded after file change")
	}
}

func TestNewFileConfig_MissingFile(t *testing.T) {
	_, err := NewFileConfig("/nonexistent/cert.pem", "/nonexistent/key.pem")
	if err == nil {
		t.Fatal("expected error for missing files")
	}
}

func TestNewMultiFileConfig_SNISelection(t *testing.T) {
	dir := t.TempDir()
	// Three separate certs, one per domain — like Caddy issues them.
	makeCert(t, dir+"/apex.crt", dir+"/apex.key", []string{"ipv6-diag.selvakn.in"})
	makeCert(t, dir+"/v4.crt", dir+"/v4.key", []string{"4.ipv6-diag.selvakn.in"})
	makeCert(t, dir+"/v6.crt", dir+"/v6.key", []string{"6.ipv6-diag.selvakn.in"})

	cfg, err := NewMultiFileConfig([]CertKeyPair{
		{dir + "/apex.crt", dir + "/apex.key"},
		{dir + "/v4.crt", dir + "/v4.key"},
		{dir + "/v6.crt", dir + "/v6.key"},
	})
	if err != nil {
		t.Fatalf("NewMultiFileConfig: %v", err)
	}

	cases := []struct {
		sni      string
		wantName string
	}{
		{"ipv6-diag.selvakn.in", "ipv6-diag.selvakn.in"},
		{"4.ipv6-diag.selvakn.in", "4.ipv6-diag.selvakn.in"},
		{"6.ipv6-diag.selvakn.in", "6.ipv6-diag.selvakn.in"},
		{"", "ipv6-diag.selvakn.in"}, // no SNI → fallback to first
	}
	for _, tc := range cases {
		cert, err := cfg.GetCertificate(&tls.ClientHelloInfo{ServerName: tc.sni})
		if err != nil {
			t.Fatalf("SNI=%q: %v", tc.sni, err)
		}
		got := cert.Leaf.Subject.CommonName
		if got != tc.wantName {
			t.Errorf("SNI=%q: got cert CN=%q, want %q", tc.sni, got, tc.wantName)
		}
	}
}

func TestNewMultiFileConfig_WildcardSelection(t *testing.T) {
	dir := t.TempDir()
	makeCert(t, dir+"/wild.crt", dir+"/wild.key", []string{"*.ipv6-diag.selvakn.in", "ipv6-diag.selvakn.in"})

	cfg, err := NewMultiFileConfig([]CertKeyPair{
		{dir + "/wild.crt", dir + "/wild.key"},
	})
	if err != nil {
		t.Fatalf("NewMultiFileConfig: %v", err)
	}

	for _, sni := range []string{"ipv6-diag.selvakn.in", "4.ipv6-diag.selvakn.in", "6.ipv6-diag.selvakn.in"} {
		cert, err := cfg.GetCertificate(&tls.ClientHelloInfo{ServerName: sni})
		if err != nil {
			t.Fatalf("SNI=%q: %v", sni, err)
		}
		if cert == nil {
			t.Fatalf("SNI=%q: expected non-nil cert", sni)
		}
	}
}
