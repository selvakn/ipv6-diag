package diag

import (
	"net/http"
	"time"
)

// RunHTTP performs an HTTP GET to the target URL and returns a TestResult.
func RunHTTP(target string, transport *http.Transport, timeout time.Duration, stack string) TestResult {
	return runHTTPLike("HTTP", target, transport, timeout, stack)
}

// RunHTTPS performs an HTTPS GET to the target URL and returns a TestResult.
func RunHTTPS(target string, transport *http.Transport, timeout time.Duration, stack string) TestResult {
	return runHTTPLike("HTTPS", target, transport, timeout, stack)
}

// RunICMP performs an HTTP HEAD to the target URL as an ICMP-equivalent reachability check.
func RunICMP(target string, transport *http.Transport, timeout time.Duration, stack string) TestResult {
	return runHTTPLike("ICMP", target, transport, timeout, stack)
}

func runHTTPLike(testType, target string, transport *http.Transport, timeout time.Duration, stack string) TestResult {
	start := time.Now()
	af := AddressFamily(stack)

	client := &http.Client{
		Transport: transport,
		Timeout:   timeout,
	}

	method := http.MethodGet
	if testType == "ICMP" {
		method = http.MethodHead
	}

	req, err := http.NewRequest(method, target, nil)
	if err != nil {
		return failResult(TestType(testType), af, target, start, "invalid target URL: "+err.Error())
	}
	req.Header.Set("User-Agent", "ipv6diag-cli")

	resp, err := client.Do(req)
	if err != nil {
		return failResult(TestType(testType), af, target, start, err.Error())
	}
	resp.Body.Close()

	elapsed := time.Since(start)
	ms := elapsed.Milliseconds()
	status := StatusPassed
	reason := ""
	if resp.StatusCode >= 500 {
		status = StatusFailed
		reason = "server error HTTP " + http.StatusText(resp.StatusCode)
	}
	return TestResult{
		TestType:      TestType(testType),
		AddressFamily: af,
		Target:        target,
		Status:        status,
		StartedAt:     start,
		EndedAt:       time.Now(),
		DurationMs:    ms,
		LatencyMs:     &ms,
		FailureReason: reason,
	}
}

func failResult(tt TestType, af, target string, start time.Time, reason string) TestResult {
	now := time.Now()
	ms := now.Sub(start).Milliseconds()
	return TestResult{
		TestType:      tt,
		AddressFamily: af,
		Target:        target,
		Status:        StatusFailed,
		StartedAt:     start,
		EndedAt:       now,
		DurationMs:    ms,
		FailureReason: reason,
	}
}
