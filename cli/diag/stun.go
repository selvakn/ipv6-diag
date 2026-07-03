package diag

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/pion/stun/v3"
)

// RunSTUN sends a STUN binding request to the target URI and reports whether
// a response (with a mapped address) was received within the timeout.
func RunSTUN(stunURI string, stack string, timeout time.Duration) TestResult {
	start := time.Now()
	af := AddressFamily(stack)
	network := familyFor(stack)

	// Parse stun: URI → host:port
	host, port, err := parseSTUNURI(stunURI)
	if err != nil {
		return failResult(TestSTUN, af, stunURI, start, "invalid STUN URI: "+err.Error())
	}
	addr := net.JoinHostPort(host, port)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// STUN is UDP; map tcp4→udp4, tcp6→udp6.
	udpNetwork := strings.Replace(network, "tcp", "udp", 1)
	conn, err := (&net.Dialer{}).DialContext(ctx, udpNetwork, addr)
	if err != nil {
		return failResult(TestSTUN, af, stunURI, start, "dial failed: "+err.Error())
	}
	defer conn.Close()

	conn.SetDeadline(time.Now().Add(timeout)) //nolint:errcheck

	stunConn, err := stun.NewClient(conn)
	if err != nil {
		return failResult(TestSTUN, af, stunURI, start, "stun client: "+err.Error())
	}
	defer stunConn.Close()

	var mappedAddr stun.XORMappedAddress
	var rtt time.Duration
	reqSent := time.Now()
	err = stunConn.Do(stun.MustBuild(stun.TransactionID, stun.BindingRequest), func(res stun.Event) {
		if res.Error != nil {
			err = res.Error
			return
		}
		rtt = time.Since(reqSent)
		if parseErr := mappedAddr.GetFrom(res.Message); parseErr != nil {
			err = parseErr
		}
	})
	if err != nil {
		return failResult(TestSTUN, af, stunURI, start, "STUN request failed: "+err.Error())
	}

	ms := time.Since(start).Milliseconds()
	rttMs := rtt.Milliseconds()
	return TestResult{
		TestType:      TestSTUN,
		AddressFamily: af,
		Target:        stunURI,
		Status:        StatusPassed,
		StartedAt:     start,
		EndedAt:       time.Now(),
		DurationMs:    ms,
		LatencyMs:     &rttMs,
		FailureReason: "",
	}
}

func parseSTUNURI(uri string) (host, port string, err error) {
	// stun:host:port or stun:host
	s := strings.TrimPrefix(uri, "stun:")
	s = strings.TrimPrefix(s, "//")
	if h, p, e := net.SplitHostPort(s); e == nil {
		return h, p, nil
	}
	if s == "" {
		return "", "", fmt.Errorf("empty host")
	}
	return s, "3478", nil
}
