package cmd

import (
	"fmt"
	"time"

	"github.com/selvakn/ipv6diag/diag"
	"github.com/selvakn/ipv6diag/output"
)

// RunStack executes the selected test suite over a single protocol stack
// ("ipv4" or "ipv6") and returns the results.
func RunStack(cfg diag.Config, serverCfg *diag.ServerConfig, creds *diag.TurnCredentials, stack string) []diag.TestResult {
	network := "tcp4"
	if stack == "ipv6" {
		network = "tcp6"
	}
	transport := diag.ForcedTransport(network, cfg.Insecure)
	timeout := time.Duration(cfg.TimeoutMs) * time.Millisecond
	spinner := output.NewSpinner(cfg.JSONOutput)

	var results []diag.TestResult
	for _, tt := range cfg.Tests {
		target := serverCfg.TargetFor(tt)
		if target == "" {
			ms := int64(0)
			results = append(results, diag.TestResult{
				TestType:      tt,
				AddressFamily: diag.AddressFamily(stack),
				Target:        "(no target configured)",
				Status:        diag.StatusSkipped,
				StartedAt:     time.Now(),
				EndedAt:       time.Now(),
				DurationMs:    ms,
				FailureReason: "no target configured for test type",
			})
			continue
		}

		var r diag.TestResult
		switch tt {
		case diag.TestHTTP:
			r = diag.RunHTTP(target, transport, timeout, stack)
		case diag.TestHTTPS:
			r = diag.RunHTTPS(target, transport, timeout, stack)
		case diag.TestICMP:
			r = diag.RunICMP(target, transport, timeout, stack)
		case diag.TestSTUN:
			r = diag.RunSTUN(target, stack, timeout)
		case diag.TestTURN:
			turnCfg := *serverCfg // shallow copy so we don't mutate the shared config
			if cfg.TurnMPS > 0 {
				turnCfg.TurnMessagesPerSec = cfg.TurnMPS
			}
			if cfg.TurnPayload > 0 {
				turnCfg.TurnPayloadBytes = cfg.TurnPayload
			}
			r = diag.RunTURN(&turnCfg, creds, stack, timeout, spinner)
		default:
			r = diag.TestResult{
				TestType: tt, AddressFamily: diag.AddressFamily(stack),
				Target: target, Status: diag.StatusSkipped,
				StartedAt: time.Now(), EndedAt: time.Now(),
				FailureReason: "unknown test type",
			}
		}
		results = append(results, r)

		if !cfg.JSONOutput {
			printResult(r)
		}
	}
	return results
}

func printResult(r diag.TestResult) {
	dur := formatDuration(r.DurationMs)
	extras := []string{}
	if r.TransferRateKbps != nil {
		extras = append(extras, fmt.Sprintf("%.0f kbps", *r.TransferRateKbps))
	}
	if r.LatencyMs != nil && r.TransferRateKbps == nil {
		extras = append(extras, fmt.Sprintf("rtt=%dms", *r.LatencyMs))
	}
	if r.LatencyMs != nil && r.TransferRateKbps != nil {
		extras = append(extras, fmt.Sprintf("rtt=%dms", *r.LatencyMs))
	}
	if r.DeliveryQualityRatio != nil {
		extras = append(extras, fmt.Sprintf("quality=%.2f", *r.DeliveryQualityRatio))
	}
	if r.Status == diag.StatusFailed || r.Status == diag.StatusTimedOut {
		extras = append(extras, "("+r.FailureReason+")")
	}
	output.PrintResult(string(r.TestType), string(r.Status), r.Target, dur, extras...)
}

func formatDuration(ms int64) string {
	if ms >= 1000 {
		return fmt.Sprintf("%.1fs", float64(ms)/1000.0)
	}
	return fmt.Sprintf("%dms", ms)
}

// FetchPrerequisites fetches server config and (if needed) TURN credentials.
// Uses a dual-stack transport for the config fetch itself.
func FetchPrerequisites(cfg diag.Config) (*diag.ServerConfig, *diag.TurnCredentials, error) {
	// Use a plain dual-stack transport for server config/cred fetches.
	transport := diag.ForcedTransport("tcp", cfg.Insecure)

	serverCfg, err := diag.FetchServerConfig(cfg.ServerURL, transport)
	if err != nil {
		return nil, nil, fmt.Errorf("fetching server config: %w", err)
	}

	var creds *diag.TurnCredentials
	for _, tt := range cfg.Tests {
		if tt == diag.TestTURN {
			if serverCfg.TurnCredentialMode != "none" {
				creds, err = diag.FetchTurnCredentials(cfg.ServerURL, cfg.TurnToken, transport)
				if err != nil {
					return serverCfg, nil, fmt.Errorf("fetching TURN credentials: %w", err)
				}
			}
			break
		}
	}
	return serverCfg, creds, nil
}
