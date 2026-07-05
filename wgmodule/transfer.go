package wgmodule

import (
	"context"
	"fmt"
	"net"
	"time"
)

// TransferResult holds metrics from a single timed echo transfer window.
type TransferResult struct {
	BytesSent     int64
	BytesReceived int64
	SentPackets   int64
	RecvPackets   int64
	AvgRTTMs      float64
	RateKbps      float64 // based on bytes received
	Elapsed       time.Duration
}

// RunEchoTransfer sends UDP datagrams to serverAddr, receives echoes, and
// returns metrics after windowSec seconds. payloadBytes controls datagram size.
// The connection is created from the peer's netstack using DialUDP.
func RunEchoTransfer(ctx context.Context, peer *Peer, windowSec, payloadBytes int) (TransferResult, error) {
	if windowSec <= 0 {
		windowSec = 10
	}
	if payloadBytes <= 0 {
		payloadBytes = 1024
	}

	conn, err := peer.DialUDP(peer.EchoServerAddr())
	if err != nil {
		return TransferResult{}, fmt.Errorf("wgmodule: dial echo: %w", err)
	}
	defer conn.Close()

	payload := make([]byte, payloadBytes)
	for i := range payload {
		payload[i] = byte((i % 251) + 1)
	}
	recvBuf := make([]byte, payloadBytes+256)

	windowCtx, cancel := context.WithTimeout(ctx, time.Duration(windowSec)*time.Second)
	defer cancel()

	var result TransferResult
	var totalRTT time.Duration
	var rttSamples int64

	// Send packets at ~100 Hz (10ms cadence) to distribute load evenly.
	cadence := 10 * time.Millisecond
	ticker := time.NewTicker(cadence)
	defer ticker.Stop()

	// Set a short read deadline so we can check context cancellation frequently.
	const readTimeout = 200 * time.Millisecond

	sendCh := make(chan struct{}, 1)
	// Goroutine receives echoes concurrently with the send loop.
	recvDone := make(chan struct{})
	var recvErr error

	go func() {
		defer close(recvDone)
		for {
			if windowCtx.Err() != nil {
				return
			}
			deadline := time.Now().Add(readTimeout)
			_ = conn.SetReadDeadline(deadline)
			sentAt := time.Now()
			n, err := conn.Read(recvBuf)
			if err != nil {
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					continue
				}
				if windowCtx.Err() != nil {
					return
				}
				recvErr = err
				return
			}
			result.BytesReceived += int64(n)
			result.RecvPackets++
			rtt := time.Since(sentAt)
			totalRTT += rtt
			rttSamples++
		}
	}()
	_ = sendCh

	start := time.Now()
loop:
	for {
		select {
		case <-windowCtx.Done():
			break loop
		case <-ticker.C:
			sentAt := time.Now()
			if _, err := conn.Write(payload); err != nil {
				if windowCtx.Err() != nil {
					break loop
				}
				break loop
			}
			_ = sentAt
			result.BytesSent += int64(len(payload))
			result.SentPackets++
		}
	}

	// Let the receive goroutine drain briefly.
	drainCtx, drainCancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer drainCancel()
	_ = conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	select {
	case <-recvDone:
	case <-drainCtx.Done():
	}

	result.Elapsed = time.Since(start)
	if rttSamples > 0 {
		result.AvgRTTMs = float64(totalRTT.Milliseconds()) / float64(rttSamples)
	}
	elapsedSec := result.Elapsed.Seconds()
	if elapsedSec > 0 && result.BytesReceived > 0 {
		result.RateKbps = float64(result.BytesReceived*8) / elapsedSec / 1000.0
	}
	_ = recvErr
	return result, nil
}
