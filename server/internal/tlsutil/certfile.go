// Package tlsutil provides helpers for building *tls.Config values from
// certificate files, with automatic reload when the files change on disk.
//
// This is designed for deployments where another process (e.g. Caddy) manages
// certificate issuance and renewal via Let's Encrypt. The server reads Caddy's
// certificate files directly; when Caddy renews the certificate the new files
// are picked up on the next TLS handshake without restarting the server.
package tlsutil

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

// CertKeyPair is a path to a certificate PEM file and its matching private key.
type CertKeyPair struct {
	CertFile string
	KeyFile  string
}

// certCache holds a loaded certificate and the file modification times it was
// loaded from. Concurrent TLS handshakes read from the cache; a reload happens
// only when at least one of the files has a newer mtime.
type certCache struct {
	mu      sync.RWMutex
	cert    *tls.Certificate
	certMod time.Time
	keyMod  time.Time

	certFile string
	keyFile  string
}

func (c *certCache) load() error {
	cert, err := tls.LoadX509KeyPair(c.certFile, c.keyFile)
	if err != nil {
		return err
	}
	// Parse the leaf so SAN/CN fields are available for SNI matching.
	if cert.Leaf == nil && len(cert.Certificate) > 0 {
		cert.Leaf, err = x509.ParseCertificate(cert.Certificate[0])
		if err != nil {
			return fmt.Errorf("parsing leaf certificate: %w", err)
		}
	}
	certInfo, err := os.Stat(c.certFile)
	if err != nil {
		return err
	}
	keyInfo, err := os.Stat(c.keyFile)
	if err != nil {
		return err
	}
	c.cert = &cert
	c.certMod = certInfo.ModTime()
	c.keyMod = keyInfo.ModTime()
	return nil
}

// get returns the (possibly reloaded) certificate, checking mtimes on every call.
// The stat calls are cheap OS operations and happen on every TLS handshake.
func (c *certCache) get() (*tls.Certificate, error) {
	certInfo, err := os.Stat(c.certFile)
	if err != nil {
		return nil, fmt.Errorf("stat cert file: %w", err)
	}
	keyInfo, err := os.Stat(c.keyFile)
	if err != nil {
		return nil, fmt.Errorf("stat key file: %w", err)
	}

	c.mu.RLock()
	upToDate := !certInfo.ModTime().After(c.certMod) && !keyInfo.ModTime().After(c.keyMod)
	if upToDate {
		cert := c.cert
		c.mu.RUnlock()
		return cert, nil
	}
	c.mu.RUnlock()

	// Files changed — reload under write lock.
	c.mu.Lock()
	defer c.mu.Unlock()
	// Re-check: another goroutine may have reloaded while we waited.
	if !certInfo.ModTime().After(c.certMod) && !keyInfo.ModTime().After(c.keyMod) {
		return c.cert, nil
	}
	if err := c.load(); err != nil {
		return nil, fmt.Errorf("reloading TLS certificate: %w", err)
	}
	return c.cert, nil
}

// names returns the DNS names this certificate covers (SANs + CN fallback).
// Called once at startup to build the SNI lookup table.
func (c *certCache) names() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.cert == nil || c.cert.Leaf == nil {
		return nil
	}
	leaf := c.cert.Leaf
	if len(leaf.DNSNames) > 0 {
		return leaf.DNSNames
	}
	if leaf.Subject.CommonName != "" {
		return []string{leaf.Subject.CommonName}
	}
	return nil
}

// multiCertSelector picks the right certificate for an incoming TLS handshake
// based on the SNI hostname. Each certCache covers one or more DNS names parsed
// from the certificate's SANs.
type multiCertSelector struct {
	// exact maps a lowercase hostname to the certCache that covers it.
	exact map[string]*certCache
	// wildcard maps a lowercase suffix (e.g. ".example.com") to its certCache.
	wildcard map[string]*certCache
	// fallback is returned when no SNI is sent or no name matches.
	fallback *certCache
}

func newMultiCertSelector(caches []*certCache) *multiCertSelector {
	sel := &multiCertSelector{
		exact:    make(map[string]*certCache),
		wildcard: make(map[string]*certCache),
		fallback: caches[0],
	}
	for _, cache := range caches {
		for _, name := range cache.names() {
			lower := strings.ToLower(name)
			if strings.HasPrefix(lower, "*.") {
				// "*.example.com" → suffix ".example.com"
				sel.wildcard[lower[1:]] = cache
			} else {
				sel.exact[lower] = cache
			}
		}
	}
	return sel
}

func (sel *multiCertSelector) getCertificate(info *tls.ClientHelloInfo) (*tls.Certificate, error) {
	name := strings.ToLower(info.ServerName)

	// 1. Exact match.
	if c, ok := sel.exact[name]; ok {
		return c.get()
	}

	// 2. Wildcard match: strip the first label and check for "*.rest" entry.
	if dot := strings.IndexByte(name, '.'); dot >= 0 {
		if c, ok := sel.wildcard[name[dot:]]; ok {
			return c.get()
		}
	}

	// 3. Fallback to the first certificate (handles clients without SNI).
	return sel.fallback.get()
}

// NewFileConfig returns a *tls.Config that loads the certificate from certFile
// and keyFile and automatically reloads them when the files change on disk.
// Use this for single-domain deployments. For multiple domains see NewMultiFileConfig.
func NewFileConfig(certFile, keyFile string) (*tls.Config, error) {
	return NewMultiFileConfig([]CertKeyPair{{certFile, keyFile}})
}

// NewMultiFileConfig returns a *tls.Config that serves the correct certificate
// for each domain based on the TLS SNI hostname. Each CertKeyPair covers one or
// more domains (as indicated by the certificate's Subject Alternative Names).
//
// This is the right choice when a reverse proxy (e.g. Caddy) manages separate
// certificates for multiple domains. Pass one pair per domain:
//
//	NewMultiFileConfig([]CertKeyPair{
//	    {"/caddy-data/certs/ipv6-diag.selvakn.in/....crt", ".../....key"},
//	    {"/caddy-data/certs/4.ipv6-diag.selvakn.in/....crt", ".../....key"},
//	    {"/caddy-data/certs/6.ipv6-diag.selvakn.in/....crt", ".../....key"},
//	})
//
// All certificates are auto-reloaded on the next handshake after the files
// change on disk, so certificate renewals by Caddy are picked up without
// restarting the server.
func NewMultiFileConfig(pairs []CertKeyPair) (*tls.Config, error) {
	if len(pairs) == 0 {
		return nil, fmt.Errorf("at least one CertKeyPair is required")
	}
	caches := make([]*certCache, 0, len(pairs))
	for _, p := range pairs {
		c := &certCache{certFile: p.CertFile, keyFile: p.KeyFile}
		if err := c.load(); err != nil {
			return nil, fmt.Errorf("loading TLS certificate from %s / %s: %w", p.CertFile, p.KeyFile, err)
		}
		caches = append(caches, c)
	}
	sel := newMultiCertSelector(caches)
	return &tls.Config{
		GetCertificate: sel.getCertificate,
		MinVersion:     tls.VersionTLS12,
	}, nil
}
