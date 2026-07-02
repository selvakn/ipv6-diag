# Research: TURN Transfer Metrics Across Clients

## Decision 1: Browser TURN measurement via two peer connections and relay-only ICE
- **Decision**: Implement web TURN transfer metrics using two browser peer connections that exchange payload over TURN with relay-only policy for a 10-second active window.
- **Rationale**: This is the closest standards-based browser-native method for two-client send/receive TURN data and enables round-trip latency plus throughput metrics.
- **Alternatives considered**:
  - Single-peer allocation probe only: rejected because it cannot satisfy two-client transfer metrics.
  - Server-side synthetic throughput probe: rejected because metrics must represent client path behavior.

## Decision 2: Android TURN transfer metric probe with two bound clients and fixed payload loop
- **Decision**: Use two client sockets in Android TURN diagnostics, execute fixed-profile send/receive loops for 10 seconds, and calculate round-trip latency plus aggregate successful payload transfer rate.
- **Rationale**: Reuses existing Android network-stack approach while meeting required two-client behavior and metric capture.
- **Alternatives considered**:
  - Introduce full Android WebRTC stack in this change: rejected due to high dependency and initialization complexity for current scope.
  - Keep current allocate-only check: rejected as it does not provide requested transfer metrics.

## Decision 3: Persist run metrics to backend reports from both clients
- **Decision**: Upload/store TURN transfer run metrics in `/reports` payloads while preserving compatibility for existing report consumers.
- **Rationale**: Clarification requires local display plus backend persistence for support analysis.
- **Alternatives considered**:
  - Client-only metric display: rejected by clarification.
  - Separate new report endpoint: deferred to avoid unnecessary API surface expansion.

## Decision 4: Fixed payload profile and explicit quality threshold
- **Decision**: Standardize payload profile (message size + cadence + duration) across web and Android and enforce minimum delivery-quality threshold for success.
- **Rationale**: Enables meaningful cross-client comparison and avoids false-positive success labels under heavy loss.
- **Alternatives considered**:
  - Client-selected payload profile: rejected due to comparability drift.
  - Informational quality only: rejected because spec requires pass/fail enforcement.

## Decision 5: Backward-compatible data model extension
- **Decision**: Add optional metric fields (transfer rate, bytes sent/received, delivery ratio, quality threshold, transfer duration) to client results and report JSON.
- **Rationale**: Preserves existing report parsing while enabling new TURN telemetry.
- **Alternatives considered**:
  - Replace existing test-result schema: rejected due to migration risk for dashboards/export tooling.
