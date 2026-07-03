# Research: Go CLI Diagnostic Client

## Forced IPv4 / IPv6 Dialing in Go

**Decision**: Use a custom `net.Dialer` with `network` set to `"tcp4"` or `"tcp6"` explicitly, passed as a transport override to `http.Client` and as the dial function for pion's `SettingEngine`.

**Rationale**: Go's `net.Dial("tcp4", addr)` resolves only A records and opens IPv4 sockets; `"tcp6"` resolves only AAAA records. This is the same mechanism `curl --ipv4` / `--ipv6` uses. No third-party library required.

**pion/webrtc ICE forcing**: pion's `webrtc.SettingEngine` exposes `SetNetworkTypes([]webrtc.NetworkType)` which accepts `NetworkTypeUDP4`, `NetworkTypeUDP6`, `NetworkTypeTCP4`, `NetworkTypeTCP6`. Setting only the IPv4 or IPv6 variants forces ICE to gather candidates on that stack only. TURN allocation will then also use the corresponding stack.

**Alternatives considered**:
- OS-level socket options (`IP_BIND_ADDRESS_NO_PORT`, `IPV6_V6ONLY`) — too low-level, unnecessary
- Separate binary per stack — rejected, single binary is the requirement

---

## pion/webrtc for TURN Transfer Test

**Decision**: Use `github.com/pion/webrtc/v4` with two in-process `PeerConnection` objects connected via an in-process ICE signalling loop (no external signalling server), ICE transport policy set to `relay-only` to force TURN usage.

**Rationale**: Directly mirrors the browser implementation. Two `PeerConnection`s negotiate via local offer/answer exchange, gather only relay candidates (TURN), open a `DataChannel`, then pump the same payload profile (size × rate × window) as the browser. Metrics computed identically: `deliveryQualityRatio = receivedPackets / sentPackets`, `transferRateKbps`, `roundTripLatencyMs`.

**pion/webrtc vs pion/ice directly**: Using `pion/webrtc/v4` (not raw `pion/ice`) matches the browser's `RTCPeerConnection` API most closely and handles DTLS framing transparently.

**Localhost / relay policy**: Mirror the browser: if the target server hostname resolves to loopback (`127.0.0.1` / `::1`), use `ICETransportPolicyAll`; otherwise `ICETransportPolicyRelay`.

**Alternatives considered**:
- Simplified TURN allocation without ICE — rejected by user, must match browser exactly
- pion/turn directly (raw TURN allocation) — rejected for same reason

---

## Spinner / Live Progress in TTY

**Decision**: Use `github.com/mattn/go-isatty` to detect TTY, then use ANSI carriage-return (`\r`) to overwrite the current line with updated elapsed time. No third-party spinner library needed.

**Rationale**: Minimal dependency. The pattern `fmt.Fprintf(os.Stderr, "\r  TURN  running  %ds/%ds…", elapsed, total)` then `fmt.Fprintln(os.Stderr, finalLine)` is sufficient and well-understood.

**Non-TTY / JSON mode**: When `!isatty.IsTerminal(os.Stderr.Fd())` or `--json` flag is active, suppress the progress line entirely. Print nothing until the result is ready.

---

## Cross-Platform CGO-Free Build

**Decision**: `CGO_ENABLED=0` for all targets; use `modernc.org/sqlite` only in the server module (not CLI). pion/webrtc is pure Go.

**Matrix**:
| GOOS | GOARCH | Binary name |
|------|--------|-------------|
| linux | amd64 | ipv6diag-linux-amd64 |
| linux | arm64 | ipv6diag-linux-arm64 |
| darwin | amd64 | ipv6diag-darwin-amd64 |
| darwin | arm64 | ipv6diag-darwin-arm64 |
| windows | amd64 | ipv6diag-windows-amd64.exe |
| windows | arm64 | ipv6diag-windows-arm64.exe |

**GitHub Actions**: New workflow `release-cli.yml` triggers on `v*` tags alongside the existing APK workflow. Uses `actions/upload-release-asset` or `softprops/action-gh-release` (already in use) with `files:` glob pattern.

---

## Upload Schema (wire compatibility)

**Decision**: Use the same JSON schema as the web browser client's `uploadRun` payload, with `device.name = "ipv6diag-cli"`, `device.android_version = runtime.Version()`, `network.userAgent = "ipv6diag-cli/<version>"`.

**Field mapping**:
- `addressFamily`: `"IPv4"` or `"IPv6"` depending on which stack the test ran on
- `testType`: `"HTTP"`, `"HTTPS"`, `"ICMP"` (not `ICMP_EQUIV`), `"STUN"`, `"TURN"`
- `status`: `"PASS"`, `"FAIL"`, `"SKIPPED"`, `"ABORTED"` (uppercase, matches store schema)

---

## --insecure Flag and --upload Guard

**Decision**: `--insecure` sets `http.Transport.TLSClientConfig.InsecureSkipVerify = true` on the shared transport. If `--insecure` and `--upload` are both provided, the CLI prints a warning and requires a `--insecure-upload` confirmation flag to proceed with upload to an unverified endpoint.

**Rationale**: Prevents accidental submission of test results to a spoofed/MITM server.
