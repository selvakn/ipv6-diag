# Feature Specification: TURN Transfer Metrics Across Clients

**Feature Branch**: `008-turn-transfer-metrics`  
**Created**: 2026-07-02  
**Status**: Draft  
**Input**: User description: "on the turn test, the client should create two clients, send and receive data for 10 seconds, and capture the latency and transfer rate. Need this in both web as well as the android clients"

## Clarifications

### Session 2026-07-02

- Q: Which TURN metric definition should be standard across web and Android? → A: Round-trip latency and aggregate successful payload transfer rate.
- Q: Should TURN transfer metric runs be persisted to backend reports? → A: Yes, display locally and upload/store each run in backend reports.
- Q: Should payload profile be fixed across clients? → A: Use one fixed payload profile shared by both clients.
- Q: How should pass/fail be determined for delivery quality? → A: Require minimum delivery quality threshold, otherwise mark run failed or partial.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Measure TURN Transfer Metrics in Web Client (Priority: P1)

A diagnostics user runs the TURN test from the web client, which creates two TURN-connected peers, exchanges data for 10 seconds, and reports latency and transfer rate.

**Why this priority**: Web diagnostics is already available and is the fastest path to deliver immediate TURN quality visibility.

**Independent Test**: Can be tested by running the web TURN test against a reachable TURN server and verifying two-client setup, 10-second traffic exchange, and displayed latency/transfer-rate results.

**Acceptance Scenarios**:

1. **Given** a reachable TURN server and web diagnostics page, **When** the user starts the TURN transfer test, **Then** the client creates two peers, exchanges payload data for 10 seconds, and reports measured latency and transfer rate.
2. **Given** TURN credentials are invalid or unavailable, **When** the user starts the TURN transfer test, **Then** the test fails with a clear reason and does not report misleading performance values.

---

### User Story 2 - Measure TURN Transfer Metrics in Android Client (Priority: P2)

An Android diagnostics user runs the TURN test in the app, which performs the same two-client 10-second transfer flow and reports latency and transfer rate.

**Why this priority**: Android parity is required by feature scope, but can follow web once metric behavior is validated.

**Independent Test**: Can be tested by running the Android TURN test on a supported device and confirming results include latency and transfer rate from a 10-second exchange.

**Acceptance Scenarios**:

1. **Given** the Android app has a valid TURN configuration, **When** the user runs the TURN transfer test, **Then** two clients are created, data exchange runs for 10 seconds, and latency/transfer-rate metrics are shown.
2. **Given** the app loses connectivity during the transfer window, **When** the TURN transfer test runs, **Then** the test ends with a failure or partial result indicator and includes an actionable error message.

---

### User Story 3 - Compare TURN Quality Across Runs (Priority: P3)

A support engineer compares TURN transfer metrics across repeated test runs to identify degradations or improvements in relay quality.

**Why this priority**: Comparison improves troubleshooting value after core metrics are available on both clients.

**Independent Test**: Can be tested by executing multiple TURN transfer runs and confirming each run records separate metric values and timestamps for comparison.

**Acceptance Scenarios**:

1. **Given** at least two completed TURN transfer tests on the same client, **When** the user reviews prior runs, **Then** each run displays distinct latency and transfer-rate values with run timestamps.

---

### Edge Cases

- TURN allocation succeeds but peer connection negotiation does not complete.
- Data channel opens late, reducing effective transfer time within the 10-second window.
- One client sends but does not receive payload acknowledgements.
- Transfer completes with very low throughput due to network constraints.
- App/browser is backgrounded during the transfer window.
- TURN server is reachable but denies relaying for provided credentials.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST extend TURN diagnostics to create two TURN-connected clients per test run in both web and Android clients.
- **FR-002**: System MUST run bidirectional data exchange between the two TURN clients for exactly 10 seconds of active transfer time per run.
- **FR-003**: System MUST measure and report TURN latency for each completed run in both web and Android clients.
- **FR-004**: System MUST measure and report transfer rate for each completed run in both web and Android clients.
- **FR-005**: System MUST show test status transitions (pending, running, passed, failed, timed out/aborted) for TURN transfer tests.
- **FR-006**: System MUST provide clear failure reasons when TURN allocation, client pairing, or transfer execution fails.
- **FR-007**: System MUST use valid TURN credentials for the two-client transfer test and fail safely when credentials are unavailable or rejected.
- **FR-008**: System MUST prevent partial metric reporting from being shown as successful full-run metrics when the 10-second transfer window is not achieved.
- **FR-009**: Users MUST be able to run TURN transfer tests repeatedly and view metrics per run with timestamp context.
- **FR-010**: System MUST keep metric naming and units consistent between web and Android clients for cross-client comparability.
- **FR-011**: System MUST define reported latency as round-trip latency and reported transfer rate as aggregate successful payload transfer rate across the 10-second test window.
- **FR-012**: System MUST display TURN transfer metrics locally in each client and also upload/store each run in backend reports for later review.
- **FR-013**: System MUST use a single fixed payload profile for TURN transfer tests across both web and Android clients.
- **FR-014**: System MUST enforce a defined minimum delivery quality threshold for TURN transfer runs and mark runs as failed or partial when the threshold is not met.

### Key Entities *(include if feature involves data)*

- **TurnTransferRun**: One execution instance of two-client TURN transfer test, including start/end time, status, associated metrics, and backend report persistence status.
- **TurnPeerSession**: Pairing context for the two clients used in a run, including credential use outcome and connection establishment state.
- **TurnTransferMetrics**: Captured results for a run, including latency value(s), transfer rate value(s), transfer duration achieved, and validity flags.
- **TurnPayloadProfile**: Standardized payload shape used by both clients during TURN transfer tests, including message size and send cadence.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: In controlled validation environments, at least 95% of successful TURN transfer runs on web complete a full 10-second exchange and produce both latency and transfer-rate metrics.
- **SC-002**: In controlled validation environments, at least 95% of successful TURN transfer runs on Android complete a full 10-second exchange and produce both latency and transfer-rate metrics.
- **SC-003**: At least 90% of users can identify whether TURN relay quality is acceptable from reported latency and transfer-rate values without external interpretation.
- **SC-004**: Across repeated runs on the same client, metric output consistency remains within expected tolerance bands for stable network conditions in at least 90% of sessions.
- **SC-005**: 100% of successful TURN transfer results in both clients use the same metric definitions: round-trip latency and aggregate successful payload transfer rate.
- **SC-006**: At least 95% of completed TURN transfer runs from both clients are successfully persisted to backend reports with matching displayed metrics.
- **SC-007**: 100% of runs that fail minimum delivery quality thresholds are reported as failed or partial and are never labeled as successful runs.

## Assumptions

- Both web and Android clients can establish two TURN-enabled peer connections in supported runtime environments.
- TURN credentials are obtainable by each client through existing project mechanisms before starting transfer.
- Latency and transfer-rate values are presented in clearly labeled, user-facing units consistent across clients.
- This feature focuses on 10-second sampling windows and does not require long-duration endurance benchmarking in this release.
