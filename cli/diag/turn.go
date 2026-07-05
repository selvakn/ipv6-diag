package diag

import (
	"fmt"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pion/webrtc/v4"

	"github.com/selvakn/ipv6diag/output"
)

// RunTURN performs a full ICE-negotiated TURN transfer test using two in-process
// PeerConnections, matching the browser client's implementation exactly.
// transport selects which protocol to use: "", "auto", "udp", "tcp", "tls", "dtls".
func RunTURN(cfg *ServerConfig, creds *TurnCredentials, stack, transport string, timeout time.Duration, spinner *output.Spinner) TestResult {
	start := time.Now()
	af := AddressFamily(stack)

	if creds == nil {
		return TestResult{
			TestType: TestTURN, AddressFamily: af, Target: "turn:n/a",
			Status: StatusSkipped, StartedAt: start, EndedAt: time.Now(),
			DurationMs: 0, FailureReason: "TURN not enabled on server",
		}
	}
	if creds.Username == "" || creds.Password == "" {
		return failResult(TestTURN, af, "turn:n/a", start, "TURN credentials missing username/password")
	}
	if len(creds.URIs) == 0 {
		return failResult(TestTURN, af, "turn:n/a", start, "TURN credential response has no URIs")
	}

	turnURIs := sanitizeTURNURIs(creds.URIs, stack, transport)
	if len(turnURIs) == 0 {
		return failResult(TestTURN, af, strings.Join(creds.URIs, " "), start,
			fmt.Sprintf("no usable TURN URIs for %s transport=%s after sanitization (raw: %v)", af, transport, creds.URIs))
	}
	target := strings.Join(turnURIs, " ")

	iceServer := webrtc.ICEServer{
		URLs:           turnURIs,
		Username:       creds.Username,
		Credential:     creds.Password,
		CredentialType: webrtc.ICECredentialTypePassword,
	}

	networkTypes := networkTypesForStack(stack)
	icePolicy := webrtc.ICETransportPolicyRelay

	newPC := func() (*webrtc.PeerConnection, error) {
		se := webrtc.SettingEngine{}
		se.SetNetworkTypes(networkTypes)
		api := webrtc.NewAPI(webrtc.WithSettingEngine(se))
		return api.NewPeerConnection(webrtc.Configuration{
			ICEServers:         []webrtc.ICEServer{iceServer},
			ICETransportPolicy: icePolicy,
		})
	}

	pcA, err := newPC()
	if err != nil {
		return failResult(TestTURN, af, target, start, "create PeerConnection A: "+err.Error())
	}
	defer pcA.Close()

	pcB, err := newPC()
	if err != nil {
		return failResult(TestTURN, af, target, start, "create PeerConnection B: "+err.Error())
	}
	defer pcB.Close()

	// Wire ICE candidates between the two in-process peers.
	pcA.OnICECandidate(func(c *webrtc.ICECandidate) {
		if c == nil {
			return
		}
		pcB.AddICECandidate(c.ToJSON()) //nolint:errcheck
	})
	pcB.OnICECandidate(func(c *webrtc.ICECandidate) {
		if c == nil {
			return
		}
		pcA.AddICECandidate(c.ToJSON()) //nolint:errcheck
	})

	// Create data channels before negotiation so they are included in the offer.
	chAB, err := pcA.CreateDataChannel("ab", nil)
	if err != nil {
		return failResult(TestTURN, af, target, start, "create data channel AB: "+err.Error())
	}

	// pcB will receive the channel via ondatachannel.
	chBAReady := make(chan *webrtc.DataChannel, 1)
	pcB.OnDataChannel(func(dc *webrtc.DataChannel) {
		if dc.Label() == "ab" {
			chBAReady <- dc
		}
	})

	// Offer/answer exchange.
	offer, err := pcA.CreateOffer(nil)
	if err != nil {
		return failResult(TestTURN, af, target, start, "create offer: "+err.Error())
	}
	if err := pcA.SetLocalDescription(offer); err != nil {
		return failResult(TestTURN, af, target, start, "set local desc A: "+err.Error())
	}
	if err := pcB.SetRemoteDescription(offer); err != nil {
		return failResult(TestTURN, af, target, start, "set remote desc B: "+err.Error())
	}
	answer, err := pcB.CreateAnswer(nil)
	if err != nil {
		return failResult(TestTURN, af, target, start, "create answer: "+err.Error())
	}
	if err := pcB.SetLocalDescription(answer); err != nil {
		return failResult(TestTURN, af, target, start, "set local desc B: "+err.Error())
	}
	if err := pcA.SetRemoteDescription(answer); err != nil {
		return failResult(TestTURN, af, target, start, "set remote desc A: "+err.Error())
	}

	// Wait for channel AB (local) to open.
	openA := make(chan struct{}, 1)
	chAB.OnOpen(func() { openA <- struct{}{} })

	// Wait for setup with overall timeout.
	deadline := time.After(timeout)
	var chBfromA *webrtc.DataChannel
	select {
	case <-deadline:
		return timeoutResult(TestTURN, af, target, start, "waiting for ICE/TURN setup")
	case ch := <-chBAReady:
		chBfromA = ch
	}
	select {
	case <-deadline:
		return timeoutResult(TestTURN, af, target, start, "waiting for data channel open")
	case <-openA:
	}

	// Transfer window — mirror browser protocol exactly.
	windowSec := cfg.TurnWindowSeconds
	payloadBytes := cfg.TurnPayloadBytes
	mps := cfg.TurnMessagesPerSec
	threshold := cfg.TurnQualityMin
	tickMs := 1000 / mps
	if tickMs < 10 {
		tickMs = 10
	}

	payload := buildPayload(payloadBytes, 0x01)

	// Use atomics for counters shared with OnMessage callbacks (different goroutine).
	var sentPackets, receivedPackets atomic.Int64
	var bytesSent, bytesReceived atomic.Int64

	// pendingPings is accessed from both the send loop and A's OnMessage callback.
	var pingMu sync.Mutex
	var rttSumMs float64
	var rttCount int
	pingCounter := uint32(0)
	pendingPings := make(map[uint32]time.Time)

	// B: echo data back to A so traffic flows both ways through the relay;
	// echo pings back so A can measure RTT. B does not count sent/received —
	// quality is measured exclusively at A (round-trip delivery ratio).
	chBfromA.OnMessage(func(msg webrtc.DataChannelMessage) {
		chBfromA.Send(msg.Data) //nolint:errcheck — echo everything back to A
	})

	// A: count echoed data packets (marker 0x01) as received;
	// handle pong packets (marker 0x02) for RTT without counting them.
	chAB.OnMessage(func(msg webrtc.DataChannelMessage) {
		if len(msg.Data) >= 5 && msg.Data[0] == 0x02 {
			id := uint32(msg.Data[1])<<24 | uint32(msg.Data[2])<<16 | uint32(msg.Data[3])<<8 | uint32(msg.Data[4])
			pingMu.Lock()
			if sent, ok := pendingPings[id]; ok {
				rttSumMs += float64(time.Since(sent).Milliseconds())
				rttCount++
				delete(pendingPings, id)
			}
			pingMu.Unlock()
			return
		}
		bytesReceived.Add(int64(len(msg.Data)))
		receivedPackets.Add(1)
	})

	// Start spinner.
	if spinner != nil {
		spinner.Start("TURN", windowSec)
	}

	windowDeadline := time.Now().Add(time.Duration(windowSec) * time.Second)
	tick := time.NewTicker(time.Duration(tickMs) * time.Millisecond)
	defer tick.Stop()

loop:
	for {
		select {
		case <-tick.C:
			if time.Now().After(windowDeadline) {
				break loop
			}
			// Send data payload A→B. B echoes it back. Quality = echoes received / sends.
			if err := chAB.Send(payload); err != nil {
				break loop
			}
			bytesSent.Add(int64(len(payload)))
			sentPackets.Add(1) // only data sends count toward quality

			// Send ping for RTT measurement — not counted in sentPackets.
			ping := buildPing(pingCounter)
			pingMu.Lock()
			pendingPings[pingCounter] = time.Now()
			pingMu.Unlock()
			pingCounter++
			if err := chAB.Send(ping); err != nil {
				break loop
			}
			bytesSent.Add(int64(len(ping)))

		case <-deadline:
			if spinner != nil {
				spinner.Stop("")
			}
			return timeoutResult(TestTURN, af, target, start, "per-test timeout during transfer window")
		}
	}

	if spinner != nil {
		spinner.Stop("")
	}

	elapsedMs := time.Since(start).Milliseconds()
	elapsedSec := float64(elapsedMs) / 1000.0
	if elapsedSec < 0.001 {
		elapsedSec = 0.001
	}

	sent := sentPackets.Load()
	received := receivedPackets.Load()
	rxBytes := bytesReceived.Load()

	rateKbps := float64(rxBytes*8) / elapsedSec / 1000.0
	quality := float64(0)
	if sent > 0 {
		quality = float64(received) / float64(sent)
	}
	var avgRTT *int64
	pingMu.Lock()
	rc, rs := rttCount, rttSumMs
	pingMu.Unlock()
	if rc > 0 {
		v := int64(rs / float64(rc))
		avgRTT = &v
	}

	passed := quality >= threshold && elapsedSec >= float64(windowSec)-0.5
	status := StatusPassed
	reason := ""
	if !passed {
		status = StatusFailed
		reason = "delivery quality below threshold or transfer window incomplete"
	}

	txBytes := bytesSent.Load()
	windowSecInt := windowSec
	payloadProfile := fmt.Sprintf("%dB@%dHz", payloadBytes, mps)
	return TestResult{
		TestType:              TestTURN,
		AddressFamily:         af,
		Target:                target,
		Status:                status,
		StartedAt:             start,
		EndedAt:               time.Now(),
		DurationMs:            elapsedMs,
		LatencyMs:             avgRTT,
		FailureReason:         reason,
		TransferRateKbps:      &rateKbps,
		BytesSent:             &txBytes,
		BytesReceived:         &rxBytes,
		DeliveryQualityRatio:  &quality,
		QualityThresholdRatio: &threshold,
		TransferWindowSeconds: &windowSecInt,
		PayloadProfile:        payloadProfile,
	}
}

func buildPayload(size int, marker byte) []byte {
	b := make([]byte, size)
	b[0] = marker
	for i := 1; i < size; i++ {
		b[i] = byte(i % 251)
	}
	return b
}

func buildPing(counter uint32) []byte {
	p := make([]byte, 5)
	p[0] = 0x02
	p[1] = byte(counter >> 24)
	p[2] = byte(counter >> 16)
	p[3] = byte(counter >> 8)
	p[4] = byte(counter)
	return p
}

// sanitizeTURNURIs filters and normalizes TURN URIs for the given stack and transport.
//
// transport selects which protocol to include:
//   - "", "auto" — all URIs in server order (let ICE negotiate best)
//   - "udp"      — turn:?transport=udp  (plain UDP)
//   - "tcp"      — turn:?transport=tcp  (plain TCP)
//   - "tls"      — turns:?transport=tcp (TURNS over TLS/TCP)
//   - "dtls"     — turns:?transport=udp (TURNS over DTLS/UDP)
func sanitizeTURNURIs(raw []string, stack, transport string) []string {
	var out []string
	for _, r := range raw {
		r = strings.TrimSpace(r)
		lower := strings.ToLower(r)

		var scheme string
		switch {
		case strings.HasPrefix(lower, "turns:"):
			scheme = "turns"
		case strings.HasPrefix(lower, "turn:"):
			scheme = "turn"
		default:
			continue
		}

		// Parse hostport and query string.
		rest := r[len(scheme)+1:]
		var hostport, query string
		if q := strings.IndexByte(rest, '?'); q >= 0 {
			hostport, query = rest[:q], rest[q+1:]
		} else {
			hostport = rest
		}

		// Determine transport from query.
		queryTransport := "udp"
		for _, part := range strings.Split(query, "&") {
			kv := strings.SplitN(part, "=", 2)
			if len(kv) == 2 && strings.EqualFold(kv[0], "transport") {
				queryTransport = strings.ToLower(kv[1])
				break
			}
		}

		// Apply transport filter.
		switch strings.ToLower(transport) {
		case "", "auto":
			// keep all
		case "udp":
			if scheme != "turn" || queryTransport != "udp" {
				continue
			}
		case "tcp":
			if scheme != "turn" || queryTransport != "tcp" {
				continue
			}
		case "tls":
			if scheme != "turns" || queryTransport != "tcp" {
				continue
			}
		case "dtls":
			if scheme != "turns" || queryTransport != "udp" {
				continue
			}
		// unknown transport acts like "auto"
		}

		// Parse host for IP-family filtering.
		host, port, err := net.SplitHostPort(hostport)
		if err != nil {
			host = hostport
			if scheme == "turns" {
				port = "5349"
			} else {
				port = "3478"
			}
		}

		// Skip wildcard bind addresses.
		if host == "0.0.0.0" || host == "::" || host == "[::]" {
			continue
		}

		// IP-family filter (only applies to literal IP hosts).
		hostIP := net.ParseIP(host)
		if stack == "ipv6" && hostIP != nil && hostIP.To4() != nil {
			continue
		}
		if stack == "ipv4" && hostIP != nil && hostIP.To4() == nil {
			continue
		}

		// Rebuild canonical URI.
		hostForURI := host
		if strings.Contains(host, ":") && !strings.HasPrefix(host, "[") {
			hostForURI = "[" + host + "]"
		}
		out = append(out, fmt.Sprintf("%s:%s:%s?transport=%s", scheme, hostForURI, port, queryTransport))
	}

	// Deduplicate.
	seen := make(map[string]bool)
	var deduped []string
	for _, u := range out {
		if !seen[u] {
			seen[u] = true
			deduped = append(deduped, u)
		}
	}

	// Hostname fallback: if IP-family filter eliminated everything (URIs use hostnames that
	// resolve to both families), return URIs without that filter applied.
	if len(deduped) == 0 && len(raw) > 0 {
		for _, r := range raw {
			r = strings.TrimSpace(r)
			lower := strings.ToLower(r)
			var scheme string
			switch {
			case strings.HasPrefix(lower, "turns:"):
				scheme = "turns"
			case strings.HasPrefix(lower, "turn:"):
				scheme = "turn"
			default:
				continue
			}
			queryTransport := "udp"
			if strings.Contains(lower, "transport=tcp") {
				queryTransport = "tcp"
			}
			switch strings.ToLower(transport) {
			case "", "auto":
			case "udp":
				if scheme != "turn" || queryTransport != "udp" {
					continue
				}
			case "tcp":
				if scheme != "turn" || queryTransport != "tcp" {
					continue
				}
			case "tls":
				if scheme != "turns" || queryTransport != "tcp" {
					continue
				}
			case "dtls":
				if scheme != "turns" || queryTransport != "udp" {
					continue
				}
			}
			deduped = append(deduped, r)
		}
	}

	return deduped
}

func networkTypesForStack(stack string) []webrtc.NetworkType {
	if stack == "ipv6" {
		return []webrtc.NetworkType{webrtc.NetworkTypeUDP6, webrtc.NetworkTypeTCP6}
	}
	return []webrtc.NetworkType{webrtc.NetworkTypeUDP4, webrtc.NetworkTypeTCP4}
}

func timeoutResult(tt TestType, af, target string, start time.Time, reason string) TestResult {
	now := time.Now()
	ms := now.Sub(start).Milliseconds()
	return TestResult{
		TestType:      tt,
		AddressFamily: af,
		Target:        target,
		Status:        StatusTimedOut,
		StartedAt:     start,
		EndedAt:       now,
		DurationMs:    ms,
		FailureReason: reason,
	}
}
