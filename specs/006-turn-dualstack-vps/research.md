# Research: Embedded TURN Relay Service

## Decision 1: Embed TURN in existing Go process using `pion/turn`

- **Decision**: Use `github.com/pion/turn/v4` in-process instead of deploying a separate TURN daemon.
- **Rationale**: Matches the requirement for a single deployable artifact and simplifies VPS deployment/operations.
- **Alternatives considered**:
  - Standalone coturn sidecar/container: mature but violates single-service footprint requirement.
  - External managed TURN provider: fast to adopt but adds dependency/cost and reduces control.

## Decision 2: Dual-stack listeners per transport with degraded startup

- **Decision**: Attempt startup for IPv4/IPv6 UDP/TCP listeners independently and keep service running when a subset fails.
- **Rationale**: VPS networking support varies by provider; degraded behavior prevents total outage and matches clarified expectation.
- **Alternatives considered**:
  - Hard fail on any listener bind error: stricter but causes avoidable downtime.
  - Auto-disable silently: hides operational issues and complicates troubleshooting.

## Decision 3: 5-minute TURN credentials issued by HTTP endpoint

- **Decision**: Add authenticated HTTP endpoint for short-lived (5 min) TURN credentials.
- **Rationale**: Aligns with clarified security posture and avoids static shared credentials.
- **Alternatives considered**:
  - Static credentials: simpler but higher abuse risk.
  - Open relay: unacceptable security posture.

## Decision 4: In-memory credential tracking only

- **Decision**: Keep lease metadata in memory and do not persist to SQLite.
- **Rationale**: Explicit requirement; lower complexity; avoids schema changes.
- **Alternatives considered**:
  - Persist credentials in DB: improves multi-instance continuity but out of scope.
  - Stateless derived credentials only: possible, but still needs bounded validity checks and revocation strategy beyond this scope.

## Decision 5: No rate limiting on credential endpoint for v1

- **Decision**: Intentionally omit rate limiting in this iteration and document as deferred hardening.
- **Rationale**: Explicit clarification; reduces immediate scope and implementation risk.
- **Alternatives considered**:
  - Per-client throttling: preferred for production hardening, deferred.
  - Global rate cap: simpler but less effective for abuse isolation.

## Decision 6: Docker and runtime defaults optimized for generic VPS

- **Decision**: Update container/runtime docs to expose required TURN ports and remove Fly-specific assumptions from feature guidance.
- **Rationale**: Supports migration path to non-Fly hosts and reproducible deployment.
- **Alternatives considered**:
  - Keep Fly defaults and add notes only: insufficient for direct VPS operations.
