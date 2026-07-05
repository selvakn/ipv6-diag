package diag

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	wgmod "github.com/selvakn/ipv6diag-wg"
	"github.com/selvakn/ipv6diag/output"
)

// RunWireGuard performs a WireGuard tunnel diagnostic test.
// It fetches two credential sets from the server, creates two in-process
// WireGuard peers, waits for handshakes, then runs concurrent echo transfers.
// Returns a single TestResult aggregating both peers' metrics.
func RunWireGuard(cfg *ServerConfig, serverURL, token string, transport *http.Transport, stack string, timeout time.Duration, spinner *output.Spinner) TestResult {
	start := time.Now()
	af := AddressFamily(stack)

	// Fetch credentials for both peers in parallel.
	type credResult struct {
		cred *wgmod.WireGuardCredential
		err  error
	}
	credCh := make(chan credResult, 2)
	for i := 0; i < 2; i++ {
		go func() {
			cred, err := wgmod.FetchCredential(serverURL, token, transport)
			credCh <- credResult{cred, err}
		}()
	}
	credsA := <-credCh
	credsB := <-credCh

	if credsA.err != nil {
		return failResult(TestWireGuard, af, "wg:n/a", start, "fetch credentials A: "+credsA.err.Error())
	}
	if credsB.err != nil {
		return failResult(TestWireGuard, af, "wg:n/a", start, "fetch credentials B: "+credsB.err.Error())
	}
	if credsA.cred == nil || credsB.cred == nil {
		return TestResult{
			TestType:      TestWireGuard,
			AddressFamily: af,
			Target:        "wg:n/a",
			Status:        StatusSkipped,
			StartedAt:     start,
			EndedAt:       time.Now(),
			DurationMs:    time.Since(start).Milliseconds(),
			FailureReason: "WireGuard not enabled on server",
		}
	}

	echoPort := cfg.WireGuardEchoPort
	if echoPort <= 0 {
		echoPort = 7000
	}
	windowSec := cfg.WireGuardWindowSec
	if windowSec <= 0 {
		windowSec = 10
	}
	payloadBytes := cfg.WireGuardPayloadBytes
	if payloadBytes <= 0 {
		payloadBytes = 1024
	}

	target := fmt.Sprintf("wg:%s", credsA.cred.ServerEndpoint.String())

	// Create peers in parallel.
	type peerResult struct {
		peer *wgmod.Peer
		err  error
	}
	peerCh := make(chan peerResult, 2)
	for _, cred := range []*wgmod.WireGuardCredential{credsA.cred, credsB.cred} {
		cred := cred
		go func() {
			p, err := wgmod.NewPeer(cred, echoPort)
			peerCh <- peerResult{p, err}
		}()
	}
	pA := <-peerCh
	pB := <-peerCh

	if pA.err != nil {
		return failResult(TestWireGuard, af, target, start, "create peer A: "+pA.err.Error())
	}
	if pB.err != nil {
		if pA.peer != nil {
			pA.peer.Close()
		}
		return failResult(TestWireGuard, af, target, start, "create peer B: "+pB.err.Error())
	}
	defer pA.peer.Close()
	defer pB.peer.Close()

	// Wait for handshakes in parallel with overall timeout.
	handshakeTimeout := 10 * time.Second
	if timeout > 0 && timeout < handshakeTimeout {
		handshakeTimeout = timeout
	}
	hCtx, hCancel := context.WithTimeout(context.Background(), handshakeTimeout)
	defer hCancel()

	var hWg sync.WaitGroup
	hErrs := make([]error, 2)
	for i, peer := range []*wgmod.Peer{pA.peer, pB.peer} {
		i, peer := i, peer
		hWg.Add(1)
		go func() {
			defer hWg.Done()
			hErrs[i] = peer.WaitHandshake(hCtx)
		}()
	}
	hWg.Wait()

	if hErrs[0] != nil {
		return failResult(TestWireGuard, af, target, start, "handshake peer A: "+hErrs[0].Error())
	}
	if hErrs[1] != nil {
		return failResult(TestWireGuard, af, target, start, "handshake peer B: "+hErrs[1].Error())
	}

	if spinner != nil {
		spinner.Start("WireGuard", windowSec)
	}

	// Run transfer windows in parallel.
	type xferResult struct {
		tr  wgmod.TransferResult
		err error
	}
	xferCh := make(chan xferResult, 2)
	xferCtx, xferCancel := context.WithTimeout(context.Background(), time.Duration(windowSec+5)*time.Second)
	defer xferCancel()

	for _, peer := range []*wgmod.Peer{pA.peer, pB.peer} {
		peer := peer
		go func() {
			tr, err := wgmod.RunEchoTransfer(xferCtx, peer, windowSec, payloadBytes)
			xferCh <- xferResult{tr, err}
		}()
	}
	trA := <-xferCh
	trB := <-xferCh

	if spinner != nil {
		spinner.Stop("")
	}

	if trA.err != nil {
		return failResult(TestWireGuard, af, target, start, "transfer A: "+trA.err.Error())
	}
	if trB.err != nil {
		return failResult(TestWireGuard, af, target, start, "transfer B: "+trB.err.Error())
	}

	// Aggregate metrics.
	totalSent := trA.tr.BytesSent + trB.tr.BytesSent
	totalRecv := trA.tr.BytesReceived + trB.tr.BytesReceived
	totalRate := trA.tr.RateKbps + trB.tr.RateKbps
	avgRTT := (trA.tr.AvgRTTMs + trB.tr.AvgRTTMs) / 2

	elapsedMs := time.Since(start).Milliseconds()
	elapsedSec := float64(elapsedMs) / 1000.0

	passed := totalRecv > 0 && elapsedSec >= float64(windowSec)-0.5
	status := StatusPassed
	reason := ""
	if !passed {
		status = StatusFailed
		if totalRecv == 0 {
			reason = "no echo packets received"
		} else {
			reason = "transfer window incomplete"
		}
	}

	avgRTTInt := int64(avgRTT)
	payloadProfile := fmt.Sprintf("%dB@100Hz", payloadBytes)
	windowSecInt := windowSec
	return TestResult{
		TestType:              TestWireGuard,
		AddressFamily:         af,
		Target:                target,
		Status:                status,
		StartedAt:             start,
		EndedAt:               time.Now(),
		DurationMs:            elapsedMs,
		LatencyMs:             &avgRTTInt,
		FailureReason:         reason,
		TransferRateKbps:      &totalRate,
		BytesSent:             &totalSent,
		BytesReceived:         &totalRecv,
		TransferWindowSeconds: &windowSecInt,
		PayloadProfile:        payloadProfile,
	}
}

// credentialsToJSON is a helper to pass WireGuardCredentials to wgmod as JSON.
func credentialsToJSON(c *WireGuardCredentials) ([]byte, error) {
	return json.Marshal(c)
}
