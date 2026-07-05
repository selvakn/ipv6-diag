package wgmodule

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"golang.org/x/crypto/curve25519"
)

// WireGuardCredential holds the complete WireGuard session configuration
// returned by the server credential endpoint.
type WireGuardCredential struct {
	ClientPrivateKey [32]byte
	ClientPublicKey  [32]byte // derived from ClientPrivateKey
	ClientIP         *net.IPNet
	ServerPublicKey  [32]byte
	ServerEndpoint   *net.UDPAddr
	TTLSeconds       int
}

// wireGuardCredentialJSON is the wire format returned by GET /wireguard/credentials.
type wireGuardCredentialJSON struct {
	ClientPrivateKey string `json:"client_private_key"`
	ClientIP         string `json:"client_ip"`
	ServerPublicKey  string `json:"server_public_key"`
	ServerEndpoint   string `json:"server_endpoint"`
	TTLSeconds       int    `json:"ttl_seconds"`
	ExpiresAt        string `json:"expires_at"`
}

// ParseCredential decodes a WireGuard credential from the server's JSON response body.
func ParseCredential(jsonBytes []byte) (*WireGuardCredential, error) {
	var raw wireGuardCredentialJSON
	if err := json.Unmarshal(jsonBytes, &raw); err != nil {
		return nil, fmt.Errorf("wgmodule: parse credential JSON: %w", err)
	}
	return parseCredentialJSON(raw)
}

func parseCredentialJSON(raw wireGuardCredentialJSON) (*WireGuardCredential, error) {
	privKey, err := decodeKey(raw.ClientPrivateKey)
	if err != nil {
		return nil, fmt.Errorf("wgmodule: decode client_private_key: %w", err)
	}
	var pubKey [32]byte
	curve25519.ScalarBaseMult(&pubKey, &privKey)

	serverPubKey, err := decodeKey(raw.ServerPublicKey)
	if err != nil {
		return nil, fmt.Errorf("wgmodule: decode server_public_key: %w", err)
	}

	_, clientIPNet, err := net.ParseCIDR(raw.ClientIP)
	if err != nil {
		return nil, fmt.Errorf("wgmodule: parse client_ip %q: %w", raw.ClientIP, err)
	}
	// Preserve the host IP, not the network address.
	hostIP := net.ParseIP(strings.Split(raw.ClientIP, "/")[0])
	if hostIP == nil {
		return nil, fmt.Errorf("wgmodule: invalid host IP in %q", raw.ClientIP)
	}
	clientIPNet.IP = hostIP.To4()
	if clientIPNet.IP == nil {
		clientIPNet.IP = hostIP.To16()
	}

	ep, err := net.ResolveUDPAddr("udp", raw.ServerEndpoint)
	if err != nil {
		return nil, fmt.Errorf("wgmodule: resolve server_endpoint %q: %w", raw.ServerEndpoint, err)
	}

	return &WireGuardCredential{
		ClientPrivateKey: privKey,
		ClientPublicKey:  pubKey,
		ClientIP:         clientIPNet,
		ServerPublicKey:  serverPubKey,
		ServerEndpoint:   ep,
		TTLSeconds:       raw.TTLSeconds,
	}, nil
}

// FetchCredential requests a fresh WireGuard session credential from the server.
// Returns (nil, nil) if the server responds with 404 or 503 (WireGuard disabled).
func FetchCredential(serverURL, token string, transport *http.Transport) (*WireGuardCredential, error) {
	client := &http.Client{
		Transport: transport,
		Timeout:   15 * time.Second,
	}
	url := strings.TrimRight(serverURL, "/") + "/wireguard/credentials"
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("wgmodule: build request: %w", err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("wgmodule: fetch credentials: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusServiceUnavailable {
		return nil, nil // server has WireGuard disabled; caller returns SKIPPED
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("wgmodule: credential endpoint returned HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if err != nil {
		return nil, fmt.Errorf("wgmodule: read credential response: %w", err)
	}
	return ParseCredential(body)
}

func decodeKey(s string) ([32]byte, error) {
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return [32]byte{}, fmt.Errorf("base64 decode: %w", err)
	}
	if len(b) != 32 {
		return [32]byte{}, fmt.Errorf("expected 32 bytes, got %d", len(b))
	}
	var key [32]byte
	copy(key[:], b)
	return key, nil
}

// keyToHex converts a [32]byte key to lowercase hex (WireGuard UAPI format).
func keyToHex(key [32]byte) string {
	return fmt.Sprintf("%x", key[:])
}

// generateKeypair generates a fresh Curve25519 keypair for a WireGuard peer.
func generateKeypair() (private, public [32]byte, err error) {
	if _, err = rand.Read(private[:]); err != nil {
		return
	}
	private[0] &= 248
	private[31] &= 127
	private[31] |= 64
	curve25519.ScalarBaseMult(&public, &private)
	return
}
