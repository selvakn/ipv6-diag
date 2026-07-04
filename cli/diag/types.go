package diag

import (
	"strings"
	"time"
)

// Protocol controls which IP stack tests run on.
type Protocol string

const (
	ProtocolIPv4 Protocol = "ipv4"
	ProtocolIPv6 Protocol = "ipv6"
	ProtocolBoth Protocol = "both"
)

// TestType identifies a diagnostic test.
type TestType string

const (
	TestHTTP  TestType = "http"
	TestHTTPS TestType = "https"
	TestICMP  TestType = "icmp"
	TestSTUN  TestType = "stun"
	TestTURN  TestType = "turn"
)

// AllTests is the default test suite in order.
var AllTests = []TestType{TestHTTP, TestHTTPS, TestICMP, TestSTUN, TestTURN}

// Config holds all CLI runtime configuration.
type Config struct {
	ServerURL      string
	Protocol       Protocol
	Tests          []TestType
	TimeoutMs      int
	TurnToken      string
	TurnMPS        int // 0 = use server default
	TurnPayload    int // bytes, 0 = use server default
	Upload         bool
	Insecure       bool
	InsecureUpload bool
	JSONOutput     bool
	Version        string
}

// Target is a resolved test endpoint from the server config.
type Target struct {
	TestType         string
	Label            string
	Value            string
	EnabledByDefault bool
}

// ServerConfig is fetched from /browser-diagnostics/config.
type ServerConfig struct {
	TurnCredentialMode string   `json:"turn_credential_mode"`
	TurnWindowSeconds  int      `json:"turn_transfer_window_seconds"`
	TurnPayloadBytes   int      `json:"turn_payload_size_bytes"`
	TurnMessagesPerSec int      `json:"turn_messages_per_second"`
	TurnQualityMin     float64  `json:"turn_quality_threshold_ratio"`
	IPDetectV4URL      string   `json:"ip_detect_v4_url"`
	IPDetectV6URL      string   `json:"ip_detect_v6_url"`
	DefaultTargets     []target `json:"default_targets"`
}

type target struct {
	TestType         string `json:"test_type"`
	Label            string `json:"label"`
	Value            string `json:"value"`
	EnabledByDefault bool   `json:"enabled_by_default"`
}

// TargetFor returns the configured target URL/URI for a given test type.
func (sc *ServerConfig) TargetFor(tt TestType) string {
	want := strings.ToUpper(string(tt))
	if tt == TestICMP {
		want = "ICMP_EQUIV"
	}
	for _, t := range sc.DefaultTargets {
		if strings.ToUpper(t.TestType) == want {
			return t.Value
		}
	}
	return ""
}

// TurnCredentials are fetched from /turn/credentials.
type TurnCredentials struct {
	Username string   `json:"username"`
	Password string   `json:"password"`
	Realm    string   `json:"realm"`
	URIs     []string `json:"uris"`
}

// TestStatus represents the outcome of a single test.
type TestStatus string

const (
	StatusPassed   TestStatus = "passed"
	StatusFailed   TestStatus = "failed"
	StatusSkipped  TestStatus = "skipped"
	StatusTimedOut TestStatus = "timed_out"
)

// TestResult holds the outcome of a single diagnostic test.
type TestResult struct {
	TestType      TestType
	AddressFamily string // "IPv4" or "IPv6"
	Target        string
	Status        TestStatus
	StartedAt     time.Time
	EndedAt       time.Time
	DurationMs    int64
	LatencyMs     *int64
	FailureReason string

	// TURN-only metrics
	TransferRateKbps     *float64
	BytesSent            *int64
	BytesReceived        *int64
	DeliveryQualityRatio *float64
	QualityThresholdRatio *float64
	TransferWindowSeconds *int
	PayloadProfile       string
}

// DiagRun represents one CLI invocation's results.
type DiagRun struct {
	SessionID  string
	StartedAt  time.Time
	FinishedAt time.Time
	Protocol   Protocol
	ServerURL  string
	Results    []TestResult
}
