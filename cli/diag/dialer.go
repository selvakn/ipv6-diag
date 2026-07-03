package diag

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"time"
)

// ForcedTransport returns an http.Transport that dials only over the given
// address family ("tcp4" for IPv4-only, "tcp6" for IPv6-only).
// When insecure is true, TLS certificate verification is skipped.
func ForcedTransport(family string, insecure bool) *http.Transport {
	dialer := &net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
	}
	dialFn := func(ctx context.Context, origNetwork, addr string) (net.Conn, error) {
		// When family is "tcp" (dual-stack), preserve the original network type.
		chosen := family
		if family == "tcp" {
			chosen = origNetwork
		}
		return dialer.DialContext(ctx, chosen, addr)
	}
	tlsCfg := &tls.Config{}
	if insecure {
		tlsCfg.InsecureSkipVerify = true //nolint:gosec
	}
	return &http.Transport{
		DialContext:           dialFn,
		TLSClientConfig:       tlsCfg,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 10 * time.Second,
		DisableKeepAlives:     true,
	}
}

// familyFor maps a Protocol and a specific stack ("ipv4"/"ipv6") to the
// Go network type string used by net.Dial.
func familyFor(stack string) string {
	if stack == "ipv6" {
		return "tcp6"
	}
	return "tcp4"
}

// AddressFamily returns the human-readable address family label.
func AddressFamily(stack string) string {
	if stack == "ipv6" {
		return "IPv6"
	}
	return "IPv4"
}
