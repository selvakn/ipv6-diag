package diag

import (
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/pion/webrtc/v4"

	"github.com/selvakn/ipv6diag/output"
)

// RunTURN performs a full ICE-negotiated TURN transfer test using two in-process
// PeerConnections, matching the browser client's implementation exactly.
func RunTURN(cfg *ServerConfig, creds *TurnCredentials, stack string, timeout time.Duration, spinner *output.Spinner) TestResult {
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

	turnURIs := sanitizeTURNURIs(creds.URIs, stack)
	if len(turnURIs) == 0 {
		return failResult(TestTURN, af, strings.Join(creds.URIs, " "), start,
			fmt.Sprintf("no usable TURN URIs for %s after sanitization (raw: %v)", af, creds.URIs))
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

	var sentPackets, receivedPackets int64
	var bytesSent, bytesReceived int64
	var rttSum float64
	var rttCount int
	pingCounter := uint32(0)
	pendingPings := make(map[uint32]time.Time)

	// B echoes ping packets back to A.
	chBfromA.OnMessage(func(msg webrtc.DataChannelMessage) {
		if len(msg.Data) >= 5 && msg.Data[0] == 0x02 {
			chBfromA.Send(msg.Data) //nolint:errcheck
			return
		}
		bytesReceived += int64(len(msg.Data))
		receivedPackets++
	})

	// A receives data from B and pong from its own pings.
	chAB.OnMessage(func(msg webrtc.DataChannelMessage) {
		if len(msg.Data) >= 5 && msg.Data[0] == 0x02 {
			id := uint32(msg.Data[1])<<24 | uint32(msg.Data[2])<<16 | uint32(msg.Data[3])<<8 | uint32(msg.Data[4])
			if sent, ok := pendingPings[id]; ok {
				rttSum += float64(time.Since(sent).Milliseconds())
				rttCount++
				delete(pendingPings, id)
			}
			return
		}
		bytesReceived += int64(len(msg.Data))
		receivedPackets++
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
			// Send data payload on AB.
			if err := chAB.Send(payload); err != nil {
				break loop
			}
			bytesSent += int64(len(payload))
			sentPackets++

			// Send ping on AB.
			ping := buildPing(pingCounter)
			pendingPings[pingCounter] = time.Now()
			pingCounter++
			if err := chAB.Send(ping); err != nil {
				break loop
			}
			bytesSent += int64(len(ping))
			sentPackets++

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

	rateKbps := float64(bytesReceived*8) / elapsedSec / 1000.0
	quality := float64(0)
	if sentPackets > 0 {
		quality = float64(receivedPackets) / float64(sentPackets)
	}
	var avgRTT *int64
	if rttCount > 0 {
		v := int64(rttSum / float64(rttCount))
		avgRTT = &v
	}

	passed := quality >= threshold && elapsedSec >= float64(windowSec)-0.5
	status := StatusPassed
	reason := ""
	if !passed {
		status = StatusFailed
		reason = "delivery quality below threshold or transfer window incomplete"
	}

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
		BytesSent:             &bytesSent,
		BytesReceived:         &bytesReceived,
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

func sanitizeTURNURIs(raw []string, stack string) []string {
	var out []string
	for _, r := range raw {
		r = strings.TrimSpace(r)
		if !strings.HasPrefix(strings.ToLower(r), "turn:") {
			continue
		}
		withProto := strings.Replace(r, "turn:", "turn://", 1)
		// Extract host from turn://host:port?transport=udp
		withProto = strings.SplitN(withProto, "?", 2)[0]
		hostport := strings.TrimPrefix(withProto, "turn://")
		host, port, err := net.SplitHostPort(hostport)
		if err != nil {
			// might be host-only
			host = hostport
			port = "3478"
		}
		// Skip wildcard addresses.
		if host == "0.0.0.0" || host == "::" || host == "[::]" {
			continue
		}
		// For IPv6 stack, prefer URIs with IPv6 hosts; skip v4-only for v6 stack if we have v6 options.
		hostIP := net.ParseIP(host)
		if stack == "ipv6" && hostIP != nil && hostIP.To4() != nil {
			continue // skip literal IPv4 for IPv6 stack
		}
		if stack == "ipv4" && hostIP != nil && hostIP.To4() == nil {
			continue // skip literal IPv6 for IPv4 stack
		}
		hostForURI := host
		if strings.Contains(host, ":") {
			hostForURI = "[" + host + "]"
		}
		rebuilt := fmt.Sprintf("turn:%s:%s?transport=udp", hostForURI, port)
		out = append(out, rebuilt)
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
	// If no URIs matched the stack's IP family filter (e.g., hostname-based URIs), return all.
	if len(deduped) == 0 && len(raw) > 0 {
		// Hostnames can resolve to either family; return filtered list without IP-family filtering.
		for _, r := range raw {
			if strings.HasPrefix(strings.ToLower(r), "turn:") {
				deduped = append(deduped, r)
			}
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
