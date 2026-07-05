package diag

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// FetchServerConfig fetches /browser-diagnostics/config from the target server.
func FetchServerConfig(serverURL string, transport *http.Transport) (*ServerConfig, error) {
	client := &http.Client{Transport: transport}
	url := strings.TrimRight(serverURL, "/") + "/browser-diagnostics/config"
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("fetching server config: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server config returned HTTP %d", resp.StatusCode)
	}
	var cfg ServerConfig
	if err := json.NewDecoder(resp.Body).Decode(&cfg); err != nil {
		return nil, fmt.Errorf("decoding server config: %w", err)
	}
	// Apply safe defaults.
	if cfg.TurnWindowSeconds <= 0 {
		cfg.TurnWindowSeconds = 10
	}
	if cfg.TurnPayloadBytes <= 0 {
		cfg.TurnPayloadBytes = 16000
	}
	if cfg.TurnMessagesPerSec <= 0 {
		cfg.TurnMessagesPerSec = 20
	}
	if cfg.TurnQualityMin <= 0 {
		cfg.TurnQualityMin = 0.90
	}
	if cfg.WireGuardEchoPort <= 0 {
		cfg.WireGuardEchoPort = 7000
	}
	if cfg.WireGuardWindowSec <= 0 {
		cfg.WireGuardWindowSec = 10
	}
	if cfg.WireGuardPayloadBytes <= 0 {
		cfg.WireGuardPayloadBytes = 1024
	}
	return &cfg, nil
}

// FetchWireGuardCredentials fetches a WireGuard session credential from the server.
// Returns (nil, nil) when WireGuard is disabled on the server (404 or 503).
func FetchWireGuardCredentials(serverURL, token string, transport *http.Transport) (*WireGuardCredentials, error) {
	client := &http.Client{Transport: transport, Timeout: 15 * time.Second}
	url := strings.TrimRight(serverURL, "/") + "/wireguard/credentials"
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("fetching wireguard credentials: %w", err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching wireguard credentials: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusServiceUnavailable {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("wireguard credential endpoint returned HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if err != nil {
		return nil, fmt.Errorf("reading wireguard credentials: %w", err)
	}
	var creds WireGuardCredentials
	if err := json.Unmarshal(body, &creds); err != nil {
		return nil, fmt.Errorf("decoding wireguard credentials: %w", err)
	}
	return &creds, nil
}
