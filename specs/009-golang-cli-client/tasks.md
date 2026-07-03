# Tasks: Go CLI Diagnostic Client

**Input**: Design documents from `/specs/009-golang-cli-client/`
**Prerequisites**: plan.md ✓, spec.md ✓, research.md ✓, data-model.md ✓, contracts/ ✓

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no shared dependencies)
- **[Story]**: Which user story this task belongs to (US1–US5)

---

## Phase 1: Setup

**Purpose**: Initialize the `cli/` Go module and project skeleton

- [x] T001 Create `cli/` directory and initialize Go module `github.com/selvakn/ipv6diag` with `go mod init` in `cli/`
- [x] T002 Add dependencies to `cli/go.mod`: `github.com/pion/webrtc/v4`, `github.com/pion/stun/v3`, `github.com/mattn/go-isatty` via `go get`
- [x] T003 [P] Create `cli/main.go` entry point with flag definitions matching the CLI contract (all flags declared, usage text, version flag)
- [x] T004 [P] Create `cli/cmd/run.go` with `RunDiagnostics(cfg Config) []TestResult` stub returning empty slice

**Checkpoint**: `cd cli && go build .` succeeds with no-op output

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core types, forced-protocol dialer, server config fetch, and output framework — everything user stories depend on

**⚠️ CRITICAL**: No user story work can begin until this phase is complete

- [x] T005 Create `cli/diag/types.go` with `Config`, `TestResult`, `DiagRun`, `ServerConfig`, `TurnCredentials`, `Target` structs matching data-model.md exactly
- [x] T006 Create `cli/diag/dialer.go` implementing `ForcedDialer(family string) *net.Dialer` returning a dialer restricted to `"tcp4"` or `"tcp6"` network; used by all HTTP and credential-fetch transports
- [x] T007 Create `cli/diag/config_fetch.go` — `FetchServerConfig(serverURL string, transport *http.Transport) (*ServerConfig, error)` fetching `/browser-diagnostics/config`
- [x] T008 Create `cli/output/text.go` with `PrintHeader`, `PrintResult`, `PrintSeparator`, `PrintSummary` functions for human-readable columnar output
- [x] T009 [P] Create `cli/output/json.go` with `PrintJSON(results []TestResult, run DiagRun) error` marshalling to the JSON contract shape
- [x] T010 [P] Create `cli/output/spinner.go` — TTY detection via `isatty`, live progress line writer with `Start(label, totalSec int)` / `Stop(finalLine string)` that suppresses output in non-TTY or `--json` mode

**Checkpoint**: `cd cli && go build .` succeeds; `go vet ./...` clean

---

## Phase 3: User Story 1 + 2 — Protocol Selection & Full Test Suite (Priority: P1/P2) 🎯 MVP

**Goal**: Binary runs all 5 tests with `--ipv4`, `--ipv6`, `--both` and prints results

**Independent Test**: `./ipv6diag --ipv4 --tests http` prints an HTTP IPv4 result line and exits 0 on a reachable server

### Implementation

- [x] T011 [US1] Create `cli/diag/http.go` — `RunHTTP(target string, transport *http.Transport, timeout time.Duration) TestResult` implementing HTTP GET to `/diag` endpoint; record status, duration, latency
- [x] T012 [P] [US1] Create `cli/diag/https.go` — `RunHTTPS(target string, transport *http.Transport, timeout time.Duration) TestResult` (same pattern as HTTP, separate function for distinct test type label)
- [x] T013 [P] [US1] Create `cli/diag/icmp.go` — `RunICMP(target string, transport *http.Transport, timeout time.Duration) TestResult` — HTTP HEAD to target URL; represents ICMP-equivalent reachability
- [x] T014 [US1] Create `cli/diag/stun.go` — `RunSTUN(stunURI string, family string, timeout time.Duration) TestResult` using `pion/stun/v3` to send a binding request and gather one candidate; record latency from RTT
- [x] T015 [US1] Create `cli/diag/turn.go` — `RunTURN(cfg *ServerConfig, creds *TurnCredentials, family string, timeout time.Duration, spinner *output.Spinner) TestResult` implementing full two-PeerConnection ICE negotiation via `pion/webrtc/v4`; relay-only ICE policy; 10s transfer window; same payload/ping protocol as browser client; live spinner updates during transfer window
- [x] T016 [US1] Create `cli/diag/turn_creds.go` — `FetchTurnCredentials(serverURL string, token string, transport *http.Transport) (*TurnCredentials, error)` fetching `/turn/credentials` with optional Bearer token
- [x] T017 [US1] Update `cli/cmd/run.go` `RunDiagnostics` to: build forced transport from `cfg.Protocol`, fetch ServerConfig, resolve targets, loop over selected tests calling the appropriate `Run*` functions, collect `[]TestResult`
- [x] T018 [US1] Wire `cli/main.go` to call `RunDiagnostics`, pass results to `output.PrintHeader` + per-result `output.PrintResult` + `output.PrintSummary`; handle `--both` sequential-block layout with `output.PrintSeparator` between IPv4 and IPv6 blocks
- [x] T019 [US1] Implement exit code logic in `cli/main.go`: exit 0 if all tests passed, exit 1 if any failed/errored, exit 2 for flag/config errors

**Checkpoint**: `./ipv6diag --ipv4 --tests http,https,stun` runs and prints results; `./ipv6diag --both` prints two blocks with separator; exit codes correct

---

## Phase 4: User Story 3 — Configurable Target & Safety Flags (Priority: P2)

**Goal**: `--server`, `--tests`, `--timeout`, `--insecure`, `--turn-token` all work correctly

**Independent Test**: `./ipv6diag --server https://staging.example.com --tests http --insecure` prints a WARNING to stderr and runs the HTTP test against the custom server

### Implementation

- [x] T020 [US3] Implement `--server` flag wiring in `cli/main.go`: validate it's a valid base URL; use it for all config/credential fetches and target resolution
- [x] T021 [P] [US3] Implement `--tests` flag parsing in `cli/main.go`: split comma-separated list, validate each token against allowed set (`http`, `https`, `icmp`, `stun`, `turn`), exit 2 on unknown token
- [x] T022 [P] [US3] Implement `--timeout` flag wiring: pass parsed `time.Duration` to all `Run*` functions
- [x] T023 [US3] Implement `--insecure` flag: create `http.Transport` with `TLSClientConfig.InsecureSkipVerify=true`; print `WARNING: TLS verification disabled — do not use in production` to stderr; block `--upload` combination without `--insecure-upload`
- [x] T024 [P] [US3] Implement `--turn-token` / `TURN_TOKEN` env var lookup in `cli/main.go`; pass token to `FetchTurnCredentials`

**Checkpoint**: All flags parse and behave per contract; invalid `--tests` value exits 2 with error message

---

## Phase 5: User Story 5 — Cross-Platform Release CI (Priority: P2)

**Goal**: Pushing a `v*` tag publishes 6 binaries to GitHub Releases

**Independent Test**: Manually trigger or push a test tag; verify 6 assets appear in GitHub Release

### Implementation

- [x] T025 [US5] Create `.github/workflows/release-cli.yml` — triggers on `v*` tags; matrix build for `linux/amd64`, `linux/arm64`, `darwin/amd64`, `darwin/arm64`, `windows/amd64`, `windows/arm64`; `CGO_ENABLED=0`; injects version via `-ldflags "-X main.version=${GITHUB_REF_NAME}"`; uploads all binaries to GitHub Release via `softprops/action-gh-release`
- [x] T026 [US5] Add `version` variable to `cli/main.go` and wire it into `--version` flag output and into the upload `network.userAgent` field

**Checkpoint**: `go build -ldflags "-X main.version=v1.1.13" .` and `./ipv6diag --version` prints `v1.1.13`

---

## Phase 6: User Story 4 — Upload Results (Priority: P3)

**Goal**: `--upload` POSTs results to `/api/reports` on the target server

**Independent Test**: Run `./ipv6diag --upload` and verify a new entry appears in the `/reports` dashboard

### Implementation

- [x] T027 [US4] Create `cli/upload/uploader.go` — `Upload(run DiagRun, results []TestResult, cfg Config) error` marshalling results to the `UploadPayload` JSON schema (matching browser wire format) and POST-ing to `<serverURL>/api/reports`
- [x] T028 [US4] Wire `--upload` flag in `cli/main.go`: call `upload.Upload(...)` after all tests complete; print `Uploaded: OK` or `Upload failed: <reason>` to stderr; upload failure does NOT change exit code

**Checkpoint**: `./ipv6diag --upload` produces a visible entry in `/reports` dashboard

---

## Phase 7: Polish & Cross-Cutting Concerns

**Purpose**: `--json` output, spinner integration, `--version`, `.gitignore` for `cli/`

- [x] T029 [P] Wire `--json` flag in `cli/main.go`: when set, suppress all human-readable output and spinner; call `output.PrintJSON(results, run)` instead; ensure non-TTY environments also suppress spinner
- [x] T030 [P] Create `cli/.gitignore` with Go binary patterns: `ipv6diag`, `ipv6diag-*`, `*.exe`, `*.test`, `vendor/`
- [x] T031 Validate TURN test produces metrics comparable to browser client: run both CLI and browser against same server, compare `deliveryQualityRatio` and `transferRateKbps` are within reasonable variance
- [x] T032 [P] Run `go vet ./...` and `go build` for all 6 target OS/ARCH combinations locally to confirm cross-compilation is clean

---

## Dependencies & Execution Order

### Phase Dependencies

- **Phase 1 (Setup)**: No dependencies — start immediately
- **Phase 2 (Foundational)**: Depends on Phase 1 — BLOCKS all user stories
- **Phase 3 (US1+US2)**: Depends on Phase 2 — Core MVP
- **Phase 4 (US3)**: Depends on Phase 3 (flags build on working run loop)
- **Phase 5 (US5)**: Depends on Phase 3 (binary must exist to release)
- **Phase 6 (US4)**: Depends on Phase 3 (results must exist to upload)
- **Phase 7 (Polish)**: Depends on Phases 3–6

### Parallel Opportunities per Phase

- Phase 2: T006, T009, T010 can run in parallel (different files)
- Phase 3: T011, T012, T013 can run in parallel (separate test type files); T014, T015 sequential after T006
- Phase 4: T021, T022, T024 can run in parallel

---

## Implementation Strategy

### MVP (User Stories 1 + 2 only)

1. Phase 1: Setup
2. Phase 2: Foundational
3. Phase 3: Core tests + protocol selection
4. **STOP and VALIDATE**: `./ipv6diag --both` runs 10 tests and exits 0 on reachable server

### Full Delivery Order

Phase 1 → Phase 2 → Phase 3 (MVP) → Phase 4 + Phase 5 in parallel → Phase 6 → Phase 7
