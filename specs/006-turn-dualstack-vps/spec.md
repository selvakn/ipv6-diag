# Feature Specification: Embedded TURN Relay Service

**Feature Branch**: `006-turn-dualstack-vps`  
**Created**: 2026-07-01  
**Status**: Draft  
**Input**: User description: "bundle pion/turn in the server side. add support for ipv4 and ipv6 (dual stack), note, we will have to move away from fly.io for deploying this, most likely in a VPS, make sure dockerfile is equipped for that. we want to support tcp as well as udp"

## Clarifications

### Session 2026-07-02

- Q: Which TURN authentication mode should be used? → A: Time-limited credentials generated per client/session, fetched via an endpoint, valid for 5 minutes, and held only in memory (no backend DB persistence).
- Q: What should happen if one enabled TURN listener fails during startup? → A: Start service with healthy listeners and mark failed listeners as degraded.
- Q: What dual-stack preference policy should be used? → A: Prefer IPv6 first with automatic fallback to IPv4.
- Q: Should the credential endpoint enforce rate limiting now? → A: No rate limiting for this version; defer endpoint rate limiting to a later iteration.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Run TURN relay from one server package (Priority: P1)

As an operator, I can run the diagnostic web service and TURN relay capability together from a single deployment artifact so I can provide connectivity tests and relay support without maintaining separate services.

**Why this priority**: This is the core business value and the primary requested capability.

**Independent Test**: Deploy one container image on a VPS, verify the diagnostics web endpoints are reachable, and verify TURN allocation succeeds from a test client.

**Acceptance Scenarios**:

1. **Given** a server deployment with TURN enabled, **When** a client requests relay allocation, **Then** the server provides a valid relay allocation response.
2. **Given** a server deployment with TURN enabled, **When** an operator starts the service, **Then** both diagnostics and TURN functions become available without requiring a second service process.

---

### User Story 2 - Support IPv4/IPv6 and both transports (Priority: P2)

As an operator, I can offer TURN relay over dual stack networking and both UDP and TCP so clients in different network environments can still complete relay negotiation.

**Why this priority**: Transport and IP-version compatibility directly impacts real-world client success rates.

**Independent Test**: From separate test clients, execute allocation attempts over IPv4 UDP, IPv4 TCP, IPv6 UDP, and IPv6 TCP and confirm successful responses.

**Acceptance Scenarios**:

1. **Given** an IPv4-only client, **When** it performs TURN allocation over UDP or TCP, **Then** allocation succeeds.
2. **Given** an IPv6-capable client, **When** it performs TURN allocation over UDP or TCP, **Then** allocation succeeds.

---

### User Story 3 - Deploy reliably outside Fly.io (Priority: P3)

As an operator, I can deploy the service to a generic VPS environment using Docker so migration away from Fly.io is straightforward.

**Why this priority**: Deployment portability is required for planned infrastructure migration.

**Independent Test**: Build and run the container image on a non-Fly VPS host and verify required service ports and startup behavior match deployment instructions.

**Acceptance Scenarios**:

1. **Given** a standard VPS with Docker, **When** the image is built and started, **Then** all required services start successfully with documented runtime configuration.
2. **Given** a non-Fly deployment environment, **When** operators follow setup instructions, **Then** they can expose required TURN and diagnostics ports without Fly-specific dependencies.

---

### Edge Cases

- What happens when IPv6 is unavailable on the host but IPv4 is available?
- How does the system handle clients that can connect only via TCP due to blocked UDP?
- What happens when TURN is enabled but required credentials or relay settings are missing?
- How does the service behave when one transport listener (UDP or TCP) fails to bind while the other succeeds?

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The system MUST provide bundled TURN relay capability on the server side as part of the same deployable service footprint used for diagnostics.
- **FR-002**: The system MUST support TURN allocations over both UDP and TCP transports.
- **FR-003**: The system MUST support TURN over both IPv4 and IPv6 networking.
- **FR-004**: The system MUST allow operators to configure TURN listener addresses and ports for IPv4 and IPv6 independently.
- **FR-005**: The system MUST expose clear startup status indicating whether TURN UDP and TURN TCP listeners are active.
- **FR-005**: The system MUST expose clear startup status indicating whether each enabled TURN listener variant is active or degraded.
- **FR-006**: The system MUST continue serving diagnostic endpoints even when TURN is disabled by configuration.
- **FR-007**: The deployment artifact MUST be runnable on a generic VPS using Docker without requiring Fly.io-specific platform behavior.
- **FR-008**: The Docker-based deployment flow MUST document all required port mappings and runtime environment variables needed for diagnostics and TURN operation.
- **FR-009**: The system MUST reject TURN allocation requests when authentication or relay prerequisites are not met and return a clear failure reason in logs.
- **FR-010**: The system MUST provide operator-visible logs that identify protocol (UDP/TCP), IP family (IPv4/IPv6), and outcome for TURN allocation attempts.
- **FR-010**: The system MUST provide operator-visible logs that identify protocol (UDP/TCP), IP family (IPv4/IPv6), and outcome for TURN allocation attempts.
- **FR-011**: The system MUST expose an endpoint that issues TURN credentials to authorized clients.
- **FR-012**: Issued TURN credentials MUST expire 5 minutes after issuance.
- **FR-013**: TURN credentials MUST be stored only in memory for validation during their validity window and MUST NOT be persisted in any backend database.
- **FR-014**: If one or more enabled TURN listeners fail to bind at startup, the service MUST continue running with healthy listeners and report failed listeners as degraded in status output and logs.
- **FR-015**: The service MUST prefer IPv6 listener/address candidates first and automatically fallback to IPv4 when IPv6 is unavailable or fails.
- **FR-016**: The credential endpoint MUST operate without rate limiting in this version and clearly document that abuse protection via rate limiting is deferred.

### Key Entities *(include if feature involves data)*

- **TurnServiceConfiguration**: Operator-provided settings that define enabled transports, IP-family listener bindings, credentials, and advertised relay behavior.
- **TurnListenerStatus**: Runtime status record describing whether each listener variant (IPv4/IPv6 x UDP/TCP) initialized successfully.
- **TurnAllocationAttempt**: Observable event describing a client allocation request, including transport/IP family and outcome.
- **DeploymentProfile**: Container runtime configuration for non-Fly environments, including exposed ports and required environment values.
- **TurnCredentialLease**: Short-lived credential object issued by the credential endpoint, valid for 5 minutes, tracked in memory only.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Operators can complete a first-time VPS deployment using Docker and reach a healthy running state within 20 minutes using the provided instructions.
- **SC-002**: TURN allocation success rate is at least 95% across controlled validation tests for IPv4 UDP, IPv4 TCP, IPv6 UDP, and IPv6 TCP paths when network connectivity is available.
- **SC-003**: In operator validation runs, 100% of service startups clearly report active and failed listener states for each enabled transport/IP-family combination.
- **SC-003**: In operator validation runs, 100% of service startups clearly report active and degraded states for each enabled transport/IP-family combination.
- **SC-004**: During migration testing, operators can run diagnostics and TURN features from a single deployment artifact with no Fly.io-specific runtime dependencies.
- **SC-005**: In validation tests, expired TURN credentials are consistently rejected within 5 minutes of issuance, and no credential records are written to persistent storage.
- **SC-006**: In connectivity validation, IPv6-capable clients use IPv6 TURN paths by default, and clients automatically succeed over IPv4 when IPv6 path setup fails.

## Assumptions

- VPS targets allow opening and forwarding the required diagnostics and TURN ports at the firewall and network level.
- Existing authentication and credential management patterns for diagnostics can be extended to TURN access control without introducing a new identity system in this feature.
- Initial rollout targets one deployment instance; clustering and horizontal relay scaling are out of scope for this version.
- Monitoring and alerting integration may remain minimal as long as logs provide enough detail for operational troubleshooting.
- Credential endpoint callers already use an existing access control mechanism outside the scope of this feature.
- Additional credential endpoint abuse controls (such as throttling) are intentionally postponed to a future feature.
