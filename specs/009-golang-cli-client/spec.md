# Feature Specification: Go CLI Diagnostic Client

**Feature Branch**: `009-golang-cli-client`
**Created**: 2026-07-03
**Status**: Draft
**Input**: User description: "we have android test client, and web test client. Build an equivalent golang client, which does the tests, it should be cli, and the binary it should take argument to specify if it needs to force ipv4 / ipv6 or do both the tests. add build jobs to build the binaries for windows, linux, darwin."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Run diagnostics with protocol selection (Priority: P1)

A network engineer or support technician downloads a single self-contained binary and runs it from the terminal to diagnose IPv6 connectivity on any machine. They can force IPv4-only, IPv6-only, or run both protocol stacks in a single invocation without needing a browser or Android device.

**Why this priority**: This is the core value of the feature — a portable, zero-dependency diagnostic tool that mirrors what the web and Android clients test, with explicit protocol-stack control.

**Independent Test**: Can be fully tested by running the binary with `--ipv4`, `--ipv6`, and `--both` flags against any reachable server endpoint and observing labelled pass/fail results.

**Acceptance Scenarios**:

1. **Given** a user has the binary on a Linux/macOS/Windows machine, **When** they run `ipv6diag --ipv4`, **Then** all tests are executed using IPv4-only connections and results are printed labelled as IPv4.
2. **Given** a user runs `ipv6diag --ipv6`, **When** the machine has an IPv6-capable network, **Then** all tests run over IPv6 and pass/fail results are printed; if the machine has no IPv6, tests fail clearly with a descriptive message.
3. **Given** a user runs `ipv6diag --both`, **When** the machine supports both stacks, **Then** all tests run twice (once per stack) and results are shown side by side or sequentially with per-protocol labelling.
4. **Given** a user runs the binary with no arguments, **Then** the tool defaults to `--both` behaviour and prints a brief usage hint.

---

### User Story 2 - Same test suite as web/Android clients (Priority: P2)

A support engineer runs the CLI client and gets the same set of diagnostic results that the Android or web client would produce — HTTP, HTTPS, reachability, STUN, and TURN — so results are directly comparable across clients.

**Why this priority**: Parity with existing clients is what makes the CLI a useful third data point. Without test equivalence, comparison is meaningless.

**Independent Test**: Can be tested by running the CLI against the same server endpoint the web client uses and verifying all five test types appear in the output.

**Acceptance Scenarios**:

1. **Given** default configuration, **When** the user runs the tool, **Then** it runs HTTP, HTTPS, ICMP-equivalent reachability, STUN, and TURN tests, reporting status, latency, and (for TURN) throughput and quality ratio.
2. **Given** the target server has TURN disabled, **When** the tool runs, **Then** the TURN test reports as skipped or unsupported rather than failing, and the other four tests complete normally.
3. **Given** a TURN test window of 10 seconds and the configured payload profile, **When** the TURN test runs, **Then** it reports round-trip latency, aggregate transfer rate, and a pass/fail quality verdict matching the server-configured threshold.

---

### User Story 3 - Configurable target endpoint (Priority: P2)

A developer or field engineer points the CLI at a custom server endpoint, not just the default production server, to test a staging or on-premises deployment.

**Why this priority**: Without this, the tool is locked to production and cannot be used for pre-deployment validation.

**Independent Test**: Run `ipv6diag --server https://staging.example.com` and observe the tool targeting that endpoint for all tests.

**Acceptance Scenarios**:

1. **Given** the user passes `--server <url>`, **When** the tool runs, **Then** all HTTP, HTTPS, reachability, and TURN credential lookups use that base URL.
2. **Given** no `--server` flag is provided, **When** the tool runs, **Then** it defaults to `https://ipv6-diag.selvakn.in`.

---

### User Story 4 - Upload results to the server (Priority: P3)

After running tests, the CLI optionally uploads the results to the server's report store, so support staff can view CLI runs alongside Android runs in the `/reports` dashboard.

**Why this priority**: Useful for persistent comparison but not critical for the core diagnostic use case.

**Independent Test**: Run `ipv6diag --upload` after a completed test run and verify a new entry appears in the `/reports` dashboard.

**Acceptance Scenarios**:

1. **Given** the user passes `--upload`, **When** all tests finish, **Then** results are POSTed to `/api/reports` on the target server and a confirmation (or failure) is printed.
2. **Given** `--upload` is not passed, **When** tests finish, **Then** no network request is made to the reports endpoint.

---

### User Story 5 - Download pre-built binaries from GitHub Releases (Priority: P2)

A user downloads a ready-to-run binary for their OS/architecture from the GitHub Releases page without needing a Go toolchain.

**Why this priority**: The tool's value depends on it being portable and installable without a build environment.

**Independent Test**: After triggering a release, verify that GitHub Releases contains downloadable binaries for Windows (amd64), Linux (amd64, arm64), and macOS (amd64, arm64).

**Acceptance Scenarios**:

1. **Given** a version tag is pushed, **When** the GitHub Actions release workflow completes, **Then** binaries for all six targets are attached to the GitHub Release.
2. **Given** a user downloads the Linux arm64 binary, **When** they run it on an arm64 Linux machine, **Then** it executes and produces diagnostic output without requiring any additional dependencies.

---

### Edge Cases

- What happens when both `--ipv4` and `--ipv6` flags are passed together? Treat as equivalent to `--both`.
- How does the tool behave when the target server is unreachable? Each test fails individually with a timeout/connection-refused message; remaining tests still run.
- What if STUN returns no candidates? Report as failed with a descriptive reason rather than panicking.
- What if the OS has IPv6 link-local addresses but no global IPv6 connectivity? IPv6 tests fail with a clear "no global IPv6 route" message rather than hanging.
- What if the TURN credential endpoint requires a token? The tool reads the token from a `--turn-token` flag or `TURN_TOKEN` env var; if not provided and token is required, TURN is skipped with a warning.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The CLI MUST accept `--ipv4`, `--ipv6`, and `--both` flags to control which protocol stack is used for all tests; default behaviour (no flag) MUST be equivalent to `--both`.
- **FR-012**: The CLI MUST accept a `--tests <list>` flag taking a comma-separated subset of `http,https,icmp,stun,turn`; when omitted, all five tests run. Invalid test names MUST produce a clear error before any test executes.
- **FR-002**: The CLI MUST run HTTP, HTTPS, ICMP-equivalent reachability, STUN, and TURN test types, matching the test suite of the web client.
- **FR-003**: The CLI MUST report per-test status (pass/fail/skipped/timed-out), duration, latency, and — for TURN — throughput rate and quality ratio. When running `--both`, all IPv4 results MUST be printed as a complete block followed by a visual separator, then all IPv6 results as a second complete block.
- **FR-011**: For tests with a known duration window (TURN), the CLI MUST display a live-updating progress line showing a spinner and elapsed/total time (e.g., `TURN ⠸ 7s / 10s…`). The progress line MUST be replaced in-place by the final result line upon completion. In non-TTY environments (CI pipes, `--json` mode), the progress line MUST be suppressed.
- **FR-004**: The CLI MUST accept a `--server <base-url>` flag to target any deployment; the default MUST be `https://ipv6-diag.selvakn.in`.
- **FR-013**: The CLI MUST accept an `--insecure` flag that disables TLS certificate verification for all connections in that run. When active, the CLI MUST print a warning to stderr (e.g., `WARNING: TLS verification disabled — do not use in production`). This flag MUST NOT be combinable with `--upload` without an explicit acknowledgement, to prevent accidental report upload to an unverified endpoint.
- **FR-005**: The CLI MUST accept a `--timeout <ms>` flag to override the per-test timeout; default is 15,000 ms.
- **FR-006**: The CLI MUST fetch TURN credentials from the server's `/browser-diagnostics/config` endpoint before running TURN tests, and honour the server-configured payload profile and quality threshold.
- **FR-007**: The CLI MUST support an optional `--upload` flag that POSTs results to `/api/reports` on the target server after all tests complete.
- **FR-008**: The CLI MUST exit with code 0 when all executed tests pass, and a non-zero code when any test fails or errors, to support scripted/CI use.
- **FR-009**: The CLI MUST support a `--json` output flag that emits results as a JSON document to stdout instead of human-readable text.
- **FR-010**: The release workflow MUST build and publish pre-built binaries for Windows (amd64), Linux (amd64, arm64), and macOS (amd64, arm64) as GitHub Release assets whenever a version tag is pushed.

### Key Entities

- **DiagnosticRun**: A single invocation's worth of test results — protocol stack(s) tested, target server, per-test outcomes, timestamps.
- **TestResult**: One test's outcome — test type, protocol family (IPv4/IPv6), status, duration, latency, optional throughput/quality metrics, failure reason.
- **ServerConfig**: Configuration fetched from the server before tests run — TURN profile, credential mode, default targets, quality threshold.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A user with no Go toolchain can download a single binary and produce a complete diagnostic report in under 30 seconds on any supported OS/architecture.
- **SC-002**: CLI test results for HTTP, HTTPS, STUN, and TURN are directly comparable (same pass/fail verdict) to results produced by the web client against the same server under the same network conditions.
- **SC-003**: The release workflow publishes binaries for all six OS/architecture targets within 10 minutes of a version tag being pushed.
- **SC-004**: When run in a CI pipeline with `--json` output, the exit code reliably reflects the overall pass/fail status, enabling automated gating on connectivity health.
- **SC-005**: On a machine with no IPv6 connectivity, running `--ipv6` completes (with failures) in under the configured timeout, never hanging indefinitely.

## Clarifications

### Session 2026-07-03

- Q: When running `--both`, how should results be displayed? → A: Sequential blocks — all IPv4 results printed first, then a separator line, then all IPv6 results.
- Q: How should the TURN test be implemented? → A: Full ICE negotiation using the pion/webrtc Go library, matching the browser client's implementation exactly.
- Q: Should long-running tests show live progress? → A: Yes — a live-updating line showing a spinner and elapsed/total time (e.g., `TURN ⠸ 7s / 10s…`) that resolves to the final result line when done.
- Q: Can users select individual test types to run? → A: Yes — via `--tests http,https,icmp,stun,turn` flag accepting a comma-separated subset; default (flag omitted) runs all five.
- Q: Should the CLI support skipping TLS verification for staging/custom servers? → A: Yes — `--insecure` flag disables TLS certificate verification and MUST print a visible warning line to stderr when active.

## Assumptions

- The Go CLI client will live as a new top-level directory (`cli/`) in the existing monorepo, alongside `server/` and `android/`.
- The same GitHub Actions workflow file used to build the Android APK will be extended (or a new workflow added) for the CLI binaries; signing is not required for the CLI.
- The TURN test MUST use full ICE negotiation via the pion/webrtc Go library, matching the browser client's implementation. This is the most critical test in the suite and its result must be directly comparable to browser TURN results.
- IPv4/IPv6 forcing is achieved by dialling TCP connections to the resolved IPv4 or IPv6 address of the target host explicitly, rather than relying on OS Happy Eyeballs.
- The `--upload` flag uploads results in the same JSON schema accepted by the Android client's `CloudUploader`, ensuring dashboard compatibility.
- Plain-text output is the default; `--json` is the machine-readable alternative. No interactive TUI is in scope for this version.
- The TURN test uses pion/webrtc for full ICE negotiation, fetching the 10-second transfer window, payload size, message rate, and quality threshold from the server's `/browser-diagnostics/config` endpoint at runtime.
