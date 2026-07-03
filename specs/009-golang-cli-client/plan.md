# Implementation Plan: Go CLI Diagnostic Client

**Branch**: `009-golang-cli-client` | **Date**: 2026-07-03 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/009-golang-cli-client/spec.md`

## Summary

Build a self-contained Go CLI binary (`ipv6diag`) that runs the same five diagnostic tests as the web and Android clients (HTTP, HTTPS, ICMP-equiv, STUN, TURN) with explicit IPv4/IPv6 protocol-stack control. TURN uses full ICE negotiation via pion/webrtc to produce directly comparable metrics. Cross-platform binaries (Windows/Linux/macOS × amd64/arm64) are published as GitHub Release assets via a new CI workflow.

## Technical Context

**Language/Version**: Go 1.25 (matches server module)
**Primary Dependencies**:
- `github.com/pion/webrtc/v4` — full ICE + TURN via WebRTC data channels
- `github.com/pion/stun/v3` — STUN candidate gathering (already in server module)
- `github.com/mattn/go-isatty` — TTY detection for live progress spinner
- Standard library: `net/http`, `flag`, `encoding/json`, `os`, `runtime`

**Storage**: None (results printed to stdout; optional POST to server `/api/reports`)
**Testing**: `go test ./...` within `cli/` module
**Target Platform**: Windows amd64, Linux amd64/arm64, macOS amd64/arm64
**Project Type**: Standalone CLI binary (separate Go module in monorepo)
**Performance Goals**: Complete all non-TURN tests within 15s total; TURN test runs exactly the server-configured window (default 10s)
**Constraints**: Single static binary, no runtime dependencies; CGO disabled for cross-compilation
**Scale/Scope**: Single diagnostic run per invocation; no persistent state

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

Constitution document is still template-only and defines no enforceable gates.
Initial check: PASS.
Post-design check: PASS.

## Project Structure

### Documentation (this feature)

```text
specs/009-golang-cli-client/
├── plan.md              ← this file
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   └── cli-command-contract.md
└── tasks.md
```

### Source Code (repository root)

```text
cli/
├── go.mod                          # module github.com/selvakn/ipv6diag
├── go.sum
├── main.go                         # entry point; flag parsing; orchestration
├── cmd/
│   └── run.go                      # RunDiagnostics(cfg Config) []Result
├── diag/
│   ├── http.go                     # HTTP / HTTPS / ICMP-equiv tests
│   ├── stun.go                     # STUN ICE candidate test
│   ├── turn.go                     # TURN two-peer ICE transfer test (pion/webrtc)
│   └── dialer.go                   # forced IPv4 / IPv6 dialer helper
├── output/
│   ├── text.go                     # human-readable columnar output + spinner
│   └── json.go                     # JSON output mode
├── upload/
│   └── uploader.go                 # POST to /api/reports
└── .github/workflows/
    └── release-cli.yml             # cross-platform build + publish
```

**Structure Decision**: Single flat module under `cli/` — no shared code with `server/` to keep the binary dependency-light and CGO-free. The two modules share the wire format (JSON schema) but not Go types, avoiding an accidental coupling that would require SQLite or CertMagic in the CLI binary.

## Complexity Tracking

> **Fill ONLY if Constitution Check has violations that must be justified**

No violations.
