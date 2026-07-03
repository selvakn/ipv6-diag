package upload

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/selvakn/ipv6diag/diag"
)

type uploadPayload struct {
	SessionID    string          `json:"session_id"`
	Device       deviceInfo      `json:"device"`
	Network      networkInfo     `json:"network"`
	TestResults  []uploadResult  `json:"test_results"`
	PassCount    int             `json:"pass_count"`
	TotalCount   int             `json:"total_count"`
	RunTimestamp int64           `json:"run_timestamp"`
	TestEndpoint string          `json:"test_endpoint"`
}

type deviceInfo struct {
	Name           string `json:"name"`
	Model          string `json:"model"`
	Manufacturer   string `json:"manufacturer"`
	AndroidVersion string `json:"android_version"`
	DeviceID       string `json:"device_id"`
}

type networkInfo struct {
	UserAgent string `json:"userAgent"`
	Online    bool   `json:"online"`
}

type uploadResult struct {
	ID                    string   `json:"id"`
	SessionID             string   `json:"sessionId"`
	TestType              string   `json:"testType"`
	AddressFamily         string   `json:"addressFamily"`
	Status                string   `json:"status"`
	LatencyMs             *int64   `json:"latencyMs,omitempty"`
	FailureReason         *string  `json:"failureReason,omitempty"`
	ResolvedAddress       string   `json:"resolvedAddress"`
	TransferRateKbps      *float64 `json:"transferRateKbps,omitempty"`
	BytesSent             *int64   `json:"bytesSent,omitempty"`
	BytesReceived         *int64   `json:"bytesReceived,omitempty"`
	DeliveryQualityRatio  *float64 `json:"deliveryQualityRatio,omitempty"`
	QualityThresholdRatio *float64 `json:"qualityThresholdRatio,omitempty"`
	TransferWindowSeconds *int     `json:"transferWindowSeconds,omitempty"`
	PayloadProfile        *string  `json:"payloadProfile,omitempty"`
	Timestamp             int64    `json:"timestamp"`
}

// Upload POSTs the run results to the server's /api/reports endpoint.
func Upload(run diag.DiagRun, results []diag.TestResult, cfg diag.Config, transport *http.Transport) error {
	hostname, _ := os.Hostname()

	var uploadResults []uploadResult
	pass := 0
	for i, r := range results {
		status := strings.ToUpper(string(r.Status))
		if status == "TIMED_OUT" {
			status = "ABORTED"
		}
		if r.Status == diag.StatusPassed {
			pass++
		}
		ur := uploadResult{
			ID:              fmt.Sprintf("%s-%d", r.TestType, i),
			SessionID:       run.SessionID,
			TestType:        string(r.TestType),
			AddressFamily:   r.AddressFamily,
			Status:          status,
			LatencyMs:       r.LatencyMs,
			ResolvedAddress: r.Target,
			TransferRateKbps:      r.TransferRateKbps,
			BytesSent:             r.BytesSent,
			BytesReceived:         r.BytesReceived,
			DeliveryQualityRatio:  r.DeliveryQualityRatio,
			QualityThresholdRatio: r.QualityThresholdRatio,
			TransferWindowSeconds: r.TransferWindowSeconds,
			Timestamp: time.Now().UnixMilli(),
		}
		if r.FailureReason != "" {
			ur.FailureReason = &r.FailureReason
		}
		if r.PayloadProfile != "" {
			ur.PayloadProfile = &r.PayloadProfile
		}
		uploadResults = append(uploadResults, ur)
	}

	payload := uploadPayload{
		SessionID: run.SessionID,
		Device: deviceInfo{
			Name:           "ipv6diag-cli",
			Model:          runtime.GOOS + "/" + runtime.GOARCH,
			Manufacturer:   "go",
			AndroidVersion: runtime.Version(),
			DeviceID:       "cli-" + hostname,
		},
		Network: networkInfo{
			UserAgent: "ipv6diag-cli/" + cfg.Version,
			Online:    true,
		},
		TestResults:  uploadResults,
		PassCount:    pass,
		TotalCount:   len(results),
		RunTimestamp: run.StartedAt.UnixMilli(),
		TestEndpoint: strings.TrimPrefix(strings.TrimPrefix(cfg.ServerURL, "https://"), "http://"),
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshalling upload payload: %w", err)
	}

	url := strings.TrimRight(cfg.ServerURL, "/") + "/api/reports"
	client := &http.Client{Transport: transport}
	resp, err := client.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("uploading results: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned HTTP %d", resp.StatusCode)
	}
	return nil
}
