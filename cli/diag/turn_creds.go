package diag

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// FetchTurnCredentials retrieves TURN credentials from /turn/credentials.
// token is optional; if non-empty it is sent as a Bearer token.
func FetchTurnCredentials(serverURL, token string, transport *http.Transport) (*TurnCredentials, error) {
	client := &http.Client{Transport: transport}
	url := strings.TrimRight(serverURL, "/") + "/turn/credentials"
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("building TURN cred request: %w", err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("TURN credential fetch: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusServiceUnavailable {
		return nil, nil // TURN not enabled on server
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("TURN credential endpoint returned HTTP %d", resp.StatusCode)
	}
	var creds TurnCredentials
	if err := json.NewDecoder(resp.Body).Decode(&creds); err != nil {
		return nil, fmt.Errorf("decoding TURN credentials: %w", err)
	}
	return &creds, nil
}
