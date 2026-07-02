# Implementation Plan: Embedded TURN Relay Service

**Branch**: `005-stun-turn-tests` | **Date**: 2026-07-02 | **Spec**: [spec.md](spec.md)  
**Input**: Feature specification from `/specs/006-turn-dualstack-vps/spec.md`

## Summary

Embed TURN support into the existing Go server so diagnostics and relay functionality run from one process and container. Add dual-stack UDP/TCP TURN listeners, a short-lived credential endpoint (5-minute leases, in-memory only), degraded startup behavior for partial listener failures, and VPS-oriented Docker/runtime documentation replacing Fly-specific assumptions.

## Technical Context

**Language/Version**: Go 1.25  
**Primary Dependencies**: Standard library networking; `github.com/pion/turn/v4`; existing `certmagic` for HTTPS; existing SQLite store packages unchanged for TURN credentials  
**Storage**: In-memory map/cache for TURN credential leases; existing SQLite for report data only  
**Testing**: `go test ./...`; targeted TURN integration checks via local UDP/TCP clients in server tests  
**Target Platform**: Linux container on generic VPS hosts (IPv4 + IPv6)  
**Project Type**: Web service with embedded TURN relay  
**Performance Goals**: Credential endpoint responds <200ms on local network; TURN allocation success >=95% in controlled dual-stack transport matrix  
**Constraints**: No DB persistence for TURN credentials; service remains available in degraded mode if some TURN listeners fail; no rate limiting in this version  
**Scale/Scope**: Single-instance deployment; one process providing diagnostics + TURN + credential endpoint

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

Constitution file is a placeholder template with no enforceable project gates.  
Initial result: PASS.  
Post-design result: PASS.

## Project Structure

### Documentation (this feature)

```text
specs/006-turn-dualstack-vps/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   └── turn-credential-api.md
└── tasks.md
```

### Source Code (repository root)

```text
server/
├── cmd/server/main.go                           # MODIFIED: TURN flags/env wiring, lifecycle/status logging
├── internal/
│   ├── handler/
│   │   ├── turn_credentials.go                  # NEW: short-lived TURN credential endpoint
│   │   ├── diag.go                              # existing
│   │   ├── reports.go                           # existing
│   │   └── health.go                            # existing
│   ├── turn/
│   │   ├── service.go                           # NEW: TURN service setup/start/stop/degraded states
│   │   ├── credentials.go                       # NEW: in-memory credential issuance/validation
│   │   └── config.go                            # NEW: TURN config parsing and defaults
│   └── listener/dual_stack.go                   # existing
├── Dockerfile                                   # MODIFIED: expose TURN UDP/TCP ports + VPS-friendly defaults
└── go.mod / go.sum                              # MODIFIED: add pion/turn dependency
```

**Structure Decision**: Extend the existing `server` module in place with a dedicated `internal/turn` package plus a TURN credential HTTP handler. Keep Android app unchanged for this feature.
