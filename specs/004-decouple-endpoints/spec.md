# Feature Specification: Decouple Test and Reporting Endpoints

**Feature Branch**: `004-decouple-endpoints`  
**Created**: 2026-06-29  
**Status**: Draft  
**Input**: User description: "lets decouple the test endpoint and the reporting endpoint. The reporting endpoint can be always androidipv6diag.fly.dev, but make the test endpoint (to test icmp, http, etc) as configurable. Note the client ip from the /diag endpoint can be with the reporting endpoint itself."

## Clarifications

### Session 2026-06-29

- Q: What should the default test endpoint be when custom configuration is absent? → A: `androidipv6diag.fly.dev` (same as reporting host).
- Q: Should failed probes against the configured test endpoint automatically fallback to default endpoint? → A: No fallback; keep failures tied to configured endpoint and continue report upload.
- Q: What test endpoint input format should be accepted? → A: Host or host:port only (no path).

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Configure test target independently (Priority: P1)

As a diagnostic operator, I can set a dedicated test endpoint for active connectivity checks without changing where reports are uploaded, so I can run tests against different environments while keeping reporting centralized.

**Why this priority**: Endpoint decoupling is the core value of this feature and unblocks test flexibility across environments.

**Independent Test**: Configure a non-default test endpoint, run a session, and verify that test execution uses that endpoint while upload still goes to the fixed reporting endpoint.

**Acceptance Scenarios**:

1. **Given** reporting is fixed to the default reporting host, **When** a user sets a custom test endpoint and starts diagnostics, **Then** all runtime test probes use the custom test endpoint.
2. **Given** a custom test endpoint is configured, **When** diagnostics complete, **Then** the report upload is sent to the fixed reporting endpoint.

---

### User Story 2 - Keep report-viewed client IP meaningful (Priority: P2)

As a dashboard viewer, I can still interpret client IP observations from diagnostics even when testing and reporting are separated, so results remain useful and not misleading.

**Why this priority**: Users rely on reported client-IP observations for troubleshooting; preserving clarity prevents incorrect conclusions.

**Independent Test**: Run diagnostics with a custom test endpoint and confirm report details clearly indicate that `/diag` client-IP observations can originate from the reporting endpoint context.

**Acceptance Scenarios**:

1. **Given** test and reporting endpoints differ, **When** report details are viewed, **Then** endpoint context for client-IP observations is clearly represented.
2. **Given** report upload succeeds, **When** report metadata is inspected, **Then** users can distinguish test-target context from reporting-target context.

---

### User Story 3 - Safe fallback behavior (Priority: P3)

As a mobile user, I can still run diagnostics when no custom test endpoint is set, so the app behavior remains predictable with sensible defaults.

**Why this priority**: Backward compatibility lowers rollout risk and avoids disruption for existing users.

**Independent Test**: Clear custom endpoint configuration, run diagnostics, and verify tests and reporting continue to function with default behavior.

**Acceptance Scenarios**:

1. **Given** no custom test endpoint is configured, **When** a user runs diagnostics, **Then** the system uses the default test endpoint behavior and completes normally.

---

### Edge Cases

- What happens when the configured test endpoint is unreachable but reporting endpoint is reachable?
- What happens when the configured test endpoint is invalid or malformed?
- How does the system behave when test endpoint and reporting endpoint are intentionally set to the same host?
- How does the system handle stale custom endpoint values after app update or reinstall?

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The system MUST support separate endpoint configuration for diagnostic test execution and report upload.
- **FR-002**: The system MUST keep the reporting endpoint fixed to `androidipv6diag.fly.dev` for report upload flows.
- **FR-003**: Users MUST be able to configure and update the diagnostic test endpoint independently of reporting.
- **FR-004**: The system MUST validate user-provided test endpoint values and reject invalid configurations with user-visible guidance; accepted format is host or host:port only (no path).
- **FR-005**: The system MUST use the configured test endpoint for runtime diagnostic probes (including ICMP-, HTTP-, and related connectivity checks).
- **FR-006**: The system MUST preserve existing behavior when no custom test endpoint is configured by applying default test endpoint `androidipv6diag.fly.dev`.
- **FR-007**: The system MUST record enough session context so consumers can distinguish which endpoint context produced each reported network observation, including the exact test endpoint used for the run.
- **FR-008**: The system MUST clearly communicate in report data or presentation that `/diag` client-IP observation context can be tied to the reporting endpoint.
- **FR-009**: The system MUST continue report upload attempts even if individual diagnostic probes against the configured test endpoint fail, without silently retrying those probes against another endpoint.
- **FR-010**: The system MUST ensure existing reports remain readable and comparable after endpoint decoupling is introduced.

### Key Entities *(include if feature involves data)*

- **Endpoint Configuration**: User-visible settings that define the diagnostic test endpoint and fixed reporting endpoint context.
- **Diagnostic Session Context**: Metadata attached to each run indicating which endpoint was used for tests and which endpoint handled reporting-related observations.
- **Report Observation Context**: Structured report attributes that clarify the source context for observed client IP and related network evidence.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: 100% of diagnostic runs with a custom test endpoint execute runtime probes against the configured test endpoint.
- **SC-002**: 100% of diagnostic runs upload reports to `androidipv6diag.fly.dev` regardless of configured test endpoint value.
- **SC-003**: At least 95% of users can correctly identify test-endpoint context versus reporting-endpoint context in report details during acceptance testing.
- **SC-004**: Introducing endpoint decoupling causes no increase greater than 5% in failed report uploads over a 7-day release validation window.

## Assumptions

- Existing users need backward-compatible defaults and should not be forced to configure a test endpoint before running diagnostics.
- Reporting remains centralized to a single production host for operational consistency.
- The default test endpoint is `androidipv6diag.fly.dev` unless explicitly overridden by user configuration.
- Users configuring custom test endpoints are primarily testers, QA, or operators validating multiple environments.
- Existing report consumers require explicit endpoint context labeling to avoid misinterpretation of client-IP observations.
