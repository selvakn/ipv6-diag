//go:build android

package wgmodule

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

// WireGuardResult holds the outcome of a WireGuard test suitable for gomobile export.
// All fields are strings to ensure gomobile compatibility across the JNI boundary.
type WireGuardResult struct {
	Status        string // "pass", "fail", "skipped"
	FailureReason string
	AvgRTTMs      string // float64 as string
	RateKbps      string // float64 as string
	BytesSent     string // int64 as string
	BytesReceived string // int64 as string
}

// WireGuardCallback is the async callback interface implemented in Kotlin.
// gomobile generates a Java interface for this that Kotlin can implement.
type WireGuardCallback interface {
	OnResult(result *WireGuardResult, errMsg string)
}

// RunWireGuardTestAsync starts a WireGuard test in a background goroutine.
// The callback is invoked exactly once when the test completes.
// serverURL: base URL of the diagnostic server (e.g. "https://example.com")
// token: Bearer auth token (empty if not required)
// stack: "ipv4" or "ipv6" (currently used for logging; test uses the credential IP)
func RunWireGuardTestAsync(serverURL, token, stack string, callback WireGuardCallback) {
	go func() {
		result, err := runWireGuardTest(serverURL, token)
		errMsg := ""
		if err != nil {
			errMsg = err.Error()
		}
		callback.OnResult(result, errMsg)
	}()
}

func runWireGuardTest(serverURL, token string) (*WireGuardResult, error) {
	transport := &http.Transport{}
	cred, err := FetchCredential(serverURL, token, transport)
	if err != nil {
		return &WireGuardResult{Status: "fail", FailureReason: err.Error()}, nil
	}
	if cred == nil {
		return &WireGuardResult{Status: "skipped", FailureReason: "wireguard not enabled on server"}, nil
	}

	const echoPort = 7000
	peer, err := NewPeer(cred, echoPort)
	if err != nil {
		return &WireGuardResult{Status: "fail", FailureReason: fmt.Sprintf("peer setup: %v", err)}, nil
	}
	defer peer.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := peer.WaitHandshake(ctx); err != nil {
		return &WireGuardResult{Status: "fail", FailureReason: fmt.Sprintf("handshake: %v", err)}, nil
	}

	xferCtx, xferCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer xferCancel()

	tr, err := RunEchoTransfer(xferCtx, peer, 10, 1024)
	if err != nil {
		return &WireGuardResult{Status: "fail", FailureReason: fmt.Sprintf("transfer: %v", err)}, nil
	}

	status := "pass"
	reason := ""
	if tr.RecvPackets == 0 {
		status = "fail"
		reason = "no echo packets received"
	}

	return &WireGuardResult{
		Status:        status,
		FailureReason: reason,
		AvgRTTMs:      strconv.FormatFloat(tr.AvgRTTMs, 'f', 2, 64),
		RateKbps:      strconv.FormatFloat(tr.RateKbps, 'f', 2, 64),
		BytesSent:     strconv.FormatInt(tr.BytesSent, 10),
		BytesReceived: strconv.FormatInt(tr.BytesReceived, 10),
	}, nil
}
