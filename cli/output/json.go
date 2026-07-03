package output

import (
	"encoding/json"
	"os"
)

// JSONResult is the JSON-output shape for a single test result.
type JSONResult struct {
	TestType             string   `json:"test_type"`
	AddressFamily        string   `json:"address_family"`
	Target               string   `json:"target"`
	Status               string   `json:"status"`
	DurationMs           int64    `json:"duration_ms"`
	LatencyMs            *int64   `json:"latency_ms,omitempty"`
	FailureReason        *string  `json:"failure_reason,omitempty"`
	TransferRateKbps     *float64 `json:"transfer_rate_kbps,omitempty"`
	DeliveryQualityRatio *float64 `json:"delivery_quality_ratio,omitempty"`
}

// JSONOutput is the top-level JSON document.
type JSONOutput struct {
	Version    string       `json:"version"`
	Server     string       `json:"server"`
	SessionID  string       `json:"session_id"`
	StartedAt  string       `json:"started_at"`
	FinishedAt string       `json:"finished_at"`
	Results    []JSONResult `json:"results"`
	PassCount  int          `json:"pass_count"`
	TotalCount int          `json:"total_count"`
}

// PrintJSON encodes the output document to stdout.
func PrintJSON(doc JSONOutput) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(doc)
}
