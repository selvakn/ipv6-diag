# Data Model: Go CLI Diagnostic Client

## Config (runtime, from flags)

```
Config
├── ServerURL       string          // --server, default https://ipv6-diag.selvakn.in
├── Protocol        enum{ipv4,ipv6,both}  // --ipv4 / --ipv6 / --both
├── Tests           []TestType      // --tests, default all
├── TimeoutMs       int             // --timeout, default 15000
├── TurnToken       string          // --turn-token or TURN_TOKEN env
├── Upload          bool            // --upload
├── Insecure        bool            // --insecure
├── InsecureUpload  bool            // --insecure-upload
├── JSONOutput      bool            // --json
└── Version         string          // injected at build time via -ldflags
```

## ServerConfig (fetched from /browser-diagnostics/config)

```
ServerConfig
├── TurnCredentialMode    string   // "none" | "tokenless_endpoint" | "token_required"
├── TurnWindowSeconds     int      // default 10
├── TurnPayloadBytes      int      // default 1024
├── TurnMessagesPerSec    int      // default 20
├── TurnQualityMinRatio   float64  // default 0.90
├── IPDetectV4URL         string
├── IPDetectV6URL         string
└── DefaultTargets        []Target
```

## Target

```
Target
├── TestType          string   // "HTTP" | "HTTPS" | "ICMP_EQUIV" | "STUN" | "TURN"
├── Label             string
├── Value             string   // URL or stun:/turn: URI
└── EnabledByDefault  bool
```

## TurnCredentials (fetched from /turn/credentials)

```
TurnCredentials
├── Username    string
├── Password    string
├── Realm       string
├── TTLSeconds  int
└── URIs        []string
```

## TestResult (in-memory per test run)

```
TestResult
├── TestType            string
├── AddressFamily       string    // "IPv4" | "IPv6"
├── Target              string
├── Status              string    // "passed" | "failed" | "skipped" | "timed_out"
├── StartedAt           time.Time
├── EndedAt             time.Time
├── DurationMs          int64
├── LatencyMs           *int64
├── FailureReason       string
├── ResolvedAddress     string
│   // TURN-only metrics
├── TransferRateKbps    *float64
├── BytesSent           *int64
├── BytesReceived       *int64
├── DeliveryQualityRatio *float64
├── QualityThresholdRatio *float64
├── TransferWindowSeconds *int
└── PayloadProfile      string
```

## DiagRun (one invocation)

```
DiagRun
├── SessionID    string        // uuid v4
├── StartedAt    time.Time
├── FinishedAt   time.Time
├── Protocol     string        // "ipv4" | "ipv6" | "both"
├── ServerURL    string
└── Results      []TestResult
```

## UploadPayload (POST /api/reports — wire format)

Matches the browser client's schema exactly for dashboard compatibility:

```json
{
  "session_id": "<uuid>",
  "device": {
    "name": "ipv6diag-cli",
    "model": "<GOOS>/<GOARCH>",
    "manufacturer": "go",
    "android_version": "<runtime.Version()>",
    "device_id": "<hostname-hash>"
  },
  "network": {
    "userAgent": "ipv6diag-cli/<version>",
    "online": true
  },
  "test_results": [ <TestResult as upload shape> ],
  "pass_count": <int>,
  "total_count": <int>,
  "run_timestamp": <unix-ms>,
  "test_endpoint": "<ServerURL host>"
}
```
