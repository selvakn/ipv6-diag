# Feature Specification: WireGuard Protocol Diagnostic Test

**Feature Branch**: `010-wireguard-test`  
**Created**: 2026-07-05  
**Status**: Draft  
**Input**: User description: "Add wireguard protocol to the list of tests. On the server we will run wireguard in user space, fetch short lived credentials for a session (just like turn credentials). For the cli, its probably straight forward, again use wireguard clients in userspace, two clients and exchange some data between them for the test. For the web and android its going to be complicated. Build a small gomodule, use in web with wasm and use it in android with native module. explore and clarify gaps."

## Clarifications

### Session 2026-07-05

- Q: Should the WireGuard test run in the web browser client? → A: No. Web/WASM support is explicitly out of scope for v1 due to browser raw UDP limitations.
- Q: Should the shared Go WireGuard module live in the monorepo or be published separately? → A: Monorepo-local module (e.g., `wgmodule/`), imported via `replace` directive — consistent with the existing `cli/` pattern.
- Q: Web is excluded — what is the target platform scope? → A: Server (credential issuance + userspace peer), CLI (two in-process userspace peers), Android (native Go module via JNI, no VPN Service).
- Q: What is the test payload size and duration for the WireGuard data transfer test? → A: Mirror the TURN test window — server-configured duration (default 10s), 1 MB payload transferred per direction.
- Q: How many concurrent WireGuard sessions should the server support, and what happens at the limit? → A: Configurable limit defaulting to 50 concurrent sessions; `/24` subnet for IP allocation; server returns HTTP 503 when the limit is reached — matching the TURN concurrency model.
- Q: What authentication mechanism protects the WireGuard credential endpoint? → A: Reuse the same time-based HMAC token already used for TURN credential issuance — no new auth infrastructure required.
- Q: What is the JNI call contract between the Android app and the Go native library? → A: Callback-based async — Go starts the test immediately and invokes a Java/Kotlin callback with the result when complete; the caller is never blocked.
- Q: What observability signals are required for server-side WireGuard session management? → A: Structured log lines at session create, expire, and cleanup events, plus a gauge metric for active session count — mirroring the existing TURN observability pattern.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - CLI WireGuard Connectivity Test (Priority: P1)

As a CLI user, I can run a WireGuard diagnostic test from the command line so I can verify that WireGuard tunneling is reachable and functional through my current network path.

**Why this priority**: The CLI path uses pure userspace WireGuard with no browser or OS restrictions, making it the most direct implementation and the reference test. It provides an immediately deliverable, independently useful MVP.

**Independent Test**: Run `ipv6diag` against a configured server that issues WireGuard session credentials. Verify that the WireGuard test result appears alongside STUN/TURN results with a clear pass/fail, throughput, and latency metric.

**Acceptance Scenarios**:

1. **Given** a server supports WireGuard and the CLI fetches valid session credentials, **When** the WireGuard diagnostic runs, **Then** two in-process userspace peers establish a tunnel, transfer 1 MB per direction over the server-configured window (default 10s), and the test reports `PASS` with measured throughput and round-trip latency.
2. **Given** a server that does not support WireGuard (no credential endpoint), **When** the WireGuard diagnostic runs, **Then** the test result is marked `SKIPPED` with reason `server unsupported`, and remaining tests continue unaffected.
3. **Given** WireGuard credentials are issued but tunnel setup fails (e.g., UDP blocked), **When** the diagnostic runs, **Then** the test is marked `FAIL` with a timeout or handshake-failure reason.

---

### User Story 2 - Android WireGuard Diagnostic (Priority: P2)

As an Android user, I can run a WireGuard diagnostic from the Android app so I can confirm WireGuard connectivity on mobile networks and compare it with other test results.

**Why this priority**: Mobile is a core platform for the diagnostic suite. Android supports userspace WireGuard without requiring VPN Service registration, but native module integration adds build complexity and cross-compilation overhead.

**Independent Test**: Launch the Android diagnostic app, select the WireGuard test (or run all tests), and verify the WireGuard result appears in the results list with status, latency, and throughput for both IPv4 and IPv6 stacks.

**Acceptance Scenarios**:

1. **Given** the Android device has network access and the server issues WireGuard credentials, **When** the WireGuard diagnostic runs, **Then** the native Go module creates userspace WireGuard peers in-process, exchanges test data, and reports a `PASS` with metrics.
2. **Given** the Android device's UDP traffic is filtered, **When** the WireGuard test runs, **Then** the result is `FAIL` with a handshake timeout reason, consistent with TURN failure reporting.
3. **Given** the server does not support WireGuard, **When** the test runs on Android, **Then** the result is `SKIPPED` and the remaining test suite continues.

---

### User Story 3 - Server-Side WireGuard Credential Issuance (Priority: P1)

As a diagnostic infrastructure operator, I can configure the server to issue short-lived WireGuard session credentials via an API endpoint so that all clients (CLI, Android) can establish test tunnels without pre-configuring static keys.

**Why this priority**: Credential issuance is the foundational gate for all client-side tests. No client test can run without the server-side credential endpoint working first.

**Independent Test**: Call the WireGuard credential endpoint and verify the response includes all fields necessary to establish a userspace WireGuard tunnel session.

**Acceptance Scenarios**:

1. **Given** a client makes an authenticated request to the credential endpoint, **When** the server responds, **Then** the response includes an ephemeral server public key, an allocated client IP, the server's WireGuard endpoint address/port, and a session TTL.
2. **Given** a session credential reaches its TTL, **When** a WireGuard handshake is attempted with the expired credential, **Then** the handshake is rejected by the server peer and the credential is no longer valid.
3. **Given** two simultaneous credential requests from different clients, **When** the server responds, **Then** each client receives a distinct, isolated set of credentials (unique ephemeral key pair and client IP allocation).

---

### Edge Cases

- What happens when WireGuard UDP port is blocked by a firewall with no fallback?
- How does credential TTL interact with test execution time — can a long test outlive the session?
- How does the system behave when Android NDK build produces a mismatched ABI?
- What happens if two CLI test peers fail to agree on a WireGuard handshake after credential exchange?
- How does the system report partial results if only one direction of the data transfer completes?
- What happens when the server-side WireGuard peer process fails to clean up after session expiry, exhausting the `/24` IP pool or the 50-session cap?

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The server MUST expose an API endpoint that issues short-lived WireGuard session credentials containing an ephemeral server public key, an allocated client IP within a session-specific `/24` subnet, the WireGuard endpoint address and port, and a session TTL. The endpoint MUST be protected by the same time-based HMAC token mechanism used for TURN credential issuance.
- **FR-002**: The server MUST run a userspace WireGuard peer that accepts connections from credentialed clients and reclaims session resources after TTL expiry.
- **FR-003**: The server MUST reject WireGuard handshakes from clients presenting expired or invalid credentials.
- **FR-003a**: The server MUST enforce a configurable maximum concurrent WireGuard session limit (default: 50). When the limit is reached, the credential endpoint MUST return HTTP 503 with an explicit capacity-exceeded reason. The server MUST allocate client IPs from a dedicated `/24` subnet for WireGuard sessions.
- **FR-004**: The CLI diagnostic MUST fetch WireGuard session credentials from the server and establish two in-process userspace WireGuard peers within the same process, without requiring root privileges or kernel WireGuard modules.
- **FR-005**: The CLI diagnostic MUST transfer 1 MB of data per direction between the two in-process peers through the server-relayed WireGuard tunnel over a server-configured test window (default 10s), and record throughput and round-trip latency — mirroring the TURN test methodology.
- **FR-006**: The Android client MUST use a native shared library built from a monorepo-local Go module and exposed via JNI. The JNI interface MUST be callback-based and asynchronous — the Go function starts the test and invokes a provided Java/Kotlin callback with the result on completion, never blocking the calling thread. WireGuard runs in-process without requiring Android VPN Service registration or system routing changes.
- **FR-007**: The monorepo-local Go WireGuard module MUST implement peer setup, key generation, handshake, data transfer, and teardown using only the standard WireGuard protocol without OS kernel interfaces.
- **FR-008**: The monorepo-local Go WireGuard module MUST be a separate Go module (e.g., `wgmodule/`) imported by both the CLI and Android native builds via a `replace` directive.
- **FR-009**: All clients MUST classify WireGuard test outcomes using the existing result semantics: `PASS`, `FAIL`, `SKIPPED` (server unsupported), with a protocol-specific reason string.
- **FR-010**: The WireGuard test MUST be included in the default `ALL` diagnostics run on CLI and Android.
- **FR-011**: WireGuard test results MUST be persisted and included in report uploads with the same behavior as existing test results.
- **FR-012**: WireGuard test results MUST be displayed in user-facing results views alongside STUN/TURN results.
- **FR-013**: The system MUST support running the WireGuard test independently without affecting other diagnostic tests when it is skipped or fails.
- **FR-014**: The web client MUST display the WireGuard test as `NOT SUPPORTED` (or omit it from the test catalog) — WireGuard is explicitly excluded from the browser client in v1.
- **FR-015**: The server MUST emit structured log lines at WireGuard session lifecycle events: session created (with allocated IP and TTL), session expired, and session cleanup completed or failed. The server MUST maintain and expose a gauge metric representing the count of currently active WireGuard sessions, consistent with how TURN session observability is implemented.

### Key Entities

- **WireGuardSessionCredential**: Ephemeral per-session artifact containing server public key, allocated client IP, server WireGuard endpoint (address + port), and session TTL — analogous to TURN short-term credentials.
- **WireGuardTestResult**: Diagnostic result entry containing protocol type, status, measured throughput (bytes/sec), round-trip latency (ms), and failure reason if applicable.
- **WireGuardPeer**: An in-process userspace WireGuard peer created during test execution, holding an ephemeral key pair and tunnel state for the duration of the test.
- **WireGuardGoModule**: The monorepo-local Go module (`wgmodule/`) implementing userspace WireGuard, imported by CLI and compiled to an Android native shared library via the NDK.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: 100% of completed diagnostic runs on CLI and Android include an explicit WireGuard result entry (either `PASS`, `FAIL`, or `SKIPPED`).
- **SC-002**: WireGuard test setup (credential fetch + tunnel handshake) completes within 5 seconds on a network with no filtering, for both CLI and Android clients. The data transfer phase runs for the server-configured window (default 10s), transferring 1 MB per direction.
- **SC-003**: For servers without WireGuard support or at session capacity (HTTP 503), 100% of runs produce a `SKIPPED` or `FAIL` result with an explicit reason — not an unhandled error or silent omission.
- **SC-004**: The Android native library adds no more than 5 MB to the APK size per ABI.
- **SC-005**: 100% of WireGuard test results are persisted and visible in session history and uploaded reports, with the same completeness as STUN/TURN results.
- **SC-006**: The Go WireGuard module builds successfully for at minimum: `linux/amd64` (CLI), `android/arm64`, and `android/amd64` targets — verified by CI.
- **SC-007**: Server structured logs capture all WireGuard session lifecycle events (create, expire, cleanup) and the active session gauge metric is queryable — verifiable in integration testing by inspecting log output and metric endpoint.

## Assumptions

- The server already has a TURN credential issuance pattern (REST endpoint + TTL) that the WireGuard credential endpoint mirrors in structure and access control.
- WireGuard userspace implementation is provided by `wireguard-go` (`golang.zx2c4.com/wireguard`), which supports Android cross-compilation without CGO.
- The CLI test uses two userspace peers within the same process rather than two separate processes, for simplicity and to avoid privilege requirements.
- The server-side WireGuard peer acts as a relay/hub — both CLI test peers connect to the server peer (star topology), not directly to each other, consistent with the TURN relay model.
- The Android native module does NOT require Android VPN Service, does NOT modify system routing, and runs the WireGuard tunnel entirely in userspace within the app's process.
- Web/WASM WireGuard support is deferred to a future feature due to the absence of raw UDP socket access in browsers.
- The existing diagnostic test result schema (PASS/FAIL/SKIPPED + reason + latency) is sufficient for WireGuard without introducing new status states.
- IPv4 and IPv6 stack-forced variants of the WireGuard test follow the same dual-stack pattern already established for STUN/TURN.
- The monorepo-local `wgmodule/` is referenced via a `replace` directive in both `cli/go.mod` and the Android native build — no external publishing is required for v1.
