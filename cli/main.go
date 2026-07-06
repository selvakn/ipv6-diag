package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/selvakn/ipv6diag/cmd"
	"github.com/selvakn/ipv6diag/diag"
	"github.com/selvakn/ipv6diag/output"
	"github.com/selvakn/ipv6diag/upload"
)

// version is injected at build time via -ldflags "-X main.version=vX.Y.Z"
var version = "dev"

func main() {
	var (
		flagIPv4          = flag.Bool("ipv4", false, "Force IPv4 stack only")
		flagIPv6          = flag.Bool("ipv6", false, "Force IPv6 stack only")
		flagBoth          = flag.Bool("both", false, "Run both stacks (default)")
		flagServer        = flag.String("server", "https://ipv6-diag.selvakn.in", "Base URL of the diagnostic server")
		flagTests         = flag.String("tests", "http,https,icmp,stun,turn,wireguard", "Comma-separated test subset")
		flagTimeout       = flag.Int("timeout", 15000, "Per-test timeout in milliseconds")
		flagTurnToken     = flag.String("turn-token", "", "Bearer token for /turn/credentials (or TURN_TOKEN env var)")
		flagTurnTransport = flag.String("turn-transport", "auto", "TURN transport: auto, udp, tcp, tls (TURNS/TCP), dtls (TURNS/UDP)")
		flagTurnMPS       = flag.Int("turn-mps", 0, "Override TURN messages per second (0 = use server default)")
		flagTurnPayload   = flag.Int("turn-payload", 0, "Override TURN payload size in bytes (0 = use server default)")
		flagTurnURL      = flag.String("turn-url", "", "Custom TURN server URL (e.g. turn:host:3478). Bypasses /turn/credentials when set.")
		flagTurnUsername = flag.String("turn-username", "", "Username for custom TURN server (used with --turn-url)")
		flagTurnPassword = flag.String("turn-password", "", "Password for custom TURN server (used with --turn-url)")
		flagUpload        = flag.Bool("upload", false, "POST results to /api/reports after run")
		flagInsecure      = flag.Bool("insecure", false, "Skip TLS certificate verification")
		flagInsecureUpload = flag.Bool("insecure-upload", false, "Allow --upload when --insecure is active")
		flagJSON          = flag.Bool("json", false, "Emit results as JSON to stdout")
		flagVersion       = flag.Bool("version", false, "Print version and exit")
	)
	flag.Parse()

	if *flagVersion {
		fmt.Println(version)
		os.Exit(0)
	}

	// Resolve protocol.
	protocol := diag.ProtocolBoth
	if *flagIPv4 && !*flagIPv6 {
		protocol = diag.ProtocolIPv4
	} else if *flagIPv6 && !*flagIPv4 {
		protocol = diag.ProtocolIPv6
	} else if *flagBoth || *flagIPv4 && *flagIPv6 {
		protocol = diag.ProtocolBoth
	}

	// Resolve test list.
	tests, err := parseTests(*flagTests)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(2)
	}

	// Resolve TURN token.
	turnToken := *flagTurnToken
	if turnToken == "" {
		turnToken = os.Getenv("TURN_TOKEN")
	}

	// Safety guard: --insecure + --upload requires explicit --insecure-upload.
	if *flagInsecure && *flagUpload && !*flagInsecureUpload {
		fmt.Fprintln(os.Stderr, "WARNING: uploading to unverified endpoint requires --insecure-upload")
		os.Exit(2)
	}
	if *flagInsecure {
		fmt.Fprintln(os.Stderr, "WARNING: TLS verification disabled — do not use in production")
	}

	cfg := diag.Config{
		ServerURL:      *flagServer,
		Protocol:       protocol,
		Tests:          tests,
		TimeoutMs:      *flagTimeout,
		TurnToken:      turnToken,
		TurnTransport:  *flagTurnTransport,
		TurnMPS:        *flagTurnMPS,
		TurnPayload:    *flagTurnPayload,
		TurnURL:        *flagTurnURL,
		TurnUsername:   *flagTurnUsername,
		TurnPassword:   *flagTurnPassword,
		Upload:         *flagUpload,
		Insecure:       *flagInsecure,
		InsecureUpload: *flagInsecureUpload,
		JSONOutput:     *flagJSON,
		Version:        version,
	}

	if !cfg.JSONOutput {
		output.PrintHeader(version, cfg.ServerURL)
	}

	// Fetch server config and TURN credentials.
	serverCfg, creds, err := cmd.FetchPrerequisites(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	sessionID := uuid.New().String()
	run := diag.DiagRun{
		SessionID: sessionID,
		StartedAt: time.Now(),
		Protocol:  protocol,
		ServerURL: cfg.ServerURL,
	}

	stacks := stacksFor(protocol)
	var allResults []diag.TestResult

	for i, stack := range stacks {
		if !cfg.JSONOutput {
			output.PrintBlockHeader(diag.AddressFamily(stack))
		}
		results := cmd.RunStack(cfg, serverCfg, creds, stack)
		allResults = append(allResults, results...)
		if !cfg.JSONOutput && i < len(stacks)-1 {
			output.PrintSeparator()
		}
	}

	run.FinishedAt = time.Now()
	run.Results = allResults

	if cfg.JSONOutput {
		printJSONOutput(cfg, run, allResults)
	} else {
		pass := countPassed(allResults)
		output.PrintSummary(pass, len(allResults))
	}

	if cfg.Upload {
		transport := diag.ForcedTransport("tcp", cfg.Insecure && cfg.InsecureUpload)
		if err := upload.Upload(run, allResults, cfg, transport); err != nil {
			fmt.Fprintf(os.Stderr, "Upload failed: %v\n", err)
		} else {
			fmt.Fprintln(os.Stderr, "Uploaded: OK")
		}
	}

	// Exit code: 1 if any test failed.
	for _, r := range allResults {
		if r.Status == diag.StatusFailed || r.Status == diag.StatusTimedOut {
			os.Exit(1)
		}
	}
}

func stacksFor(p diag.Protocol) []string {
	switch p {
	case diag.ProtocolIPv4:
		return []string{"ipv4"}
	case diag.ProtocolIPv6:
		return []string{"ipv6"}
	default:
		return []string{"ipv4", "ipv6"}
	}
}

func parseTests(s string) ([]diag.TestType, error) {
	valid := map[string]diag.TestType{
		"http": diag.TestHTTP, "https": diag.TestHTTPS,
		"icmp": diag.TestICMP, "stun": diag.TestSTUN, "turn": diag.TestTURN,
		"wireguard": diag.TestWireGuard,
	}
	var out []diag.TestType
	for _, tok := range strings.Split(s, ",") {
		tok = strings.TrimSpace(strings.ToLower(tok))
		if tok == "" {
			continue
		}
		tt, ok := valid[tok]
		if !ok {
			return nil, fmt.Errorf("unknown test type %q (valid: http,https,icmp,stun,turn,wireguard)", tok)
		}
		out = append(out, tt)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no tests specified")
	}
	return out, nil
}

func countPassed(results []diag.TestResult) int {
	n := 0
	for _, r := range results {
		if r.Status == diag.StatusPassed {
			n++
		}
	}
	return n
}

func printJSONOutput(cfg diag.Config, run diag.DiagRun, results []diag.TestResult) {
	type jsonResult struct {
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
	type jsonDoc struct {
		Version    string       `json:"version"`
		Server     string       `json:"server"`
		SessionID  string       `json:"session_id"`
		StartedAt  string       `json:"started_at"`
		FinishedAt string       `json:"finished_at"`
		Results    []jsonResult `json:"results"`
		PassCount  int          `json:"pass_count"`
		TotalCount int          `json:"total_count"`
	}

	var jResults []jsonResult
	pass := 0
	for _, r := range results {
		if r.Status == diag.StatusPassed {
			pass++
		}
		jr := jsonResult{
			TestType:             string(r.TestType),
			AddressFamily:        r.AddressFamily,
			Target:               r.Target,
			Status:               string(r.Status),
			DurationMs:           r.DurationMs,
			LatencyMs:            r.LatencyMs,
			TransferRateKbps:     r.TransferRateKbps,
			DeliveryQualityRatio: r.DeliveryQualityRatio,
		}
		if r.FailureReason != "" {
			jr.FailureReason = &r.FailureReason
		}
		jResults = append(jResults, jr)
	}

	doc := jsonDoc{
		Version:    cfg.Version,
		Server:     cfg.ServerURL,
		SessionID:  run.SessionID,
		StartedAt:  run.StartedAt.UTC().Format(time.RFC3339),
		FinishedAt: run.FinishedAt.UTC().Format(time.RFC3339),
		Results:    jResults,
		PassCount:  pass,
		TotalCount: len(results),
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.Encode(doc) //nolint:errcheck
}
