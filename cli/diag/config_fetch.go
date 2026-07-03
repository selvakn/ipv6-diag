package diag

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
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
		cfg.TurnPayloadBytes = 1024
	}
	if cfg.TurnMessagesPerSec <= 0 {
		cfg.TurnMessagesPerSec = 20
	}
	if cfg.TurnQualityMin <= 0 {
		cfg.TurnQualityMin = 0.90
	}
	return &cfg, nil
}
