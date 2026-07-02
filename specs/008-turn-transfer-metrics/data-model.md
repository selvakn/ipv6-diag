# Data Model: TURN Transfer Metrics Across Clients

## Entity: TurnTransferRun
- **Description**: One TURN transfer measurement execution in a client (web or Android).
- **Fields**:
  - `runId` (string, required): unique run identifier.
  - `clientType` (enum, required): `web|android`.
  - `startedAt` (timestamp, required).
  - `endedAt` (timestamp, required).
  - `status` (enum, required): `pass|fail|partial|aborted`.
  - `persistenceStatus` (enum, required): `pending|uploaded|failed`.
  - `failureReason` (string, optional).

## Entity: TurnPeerSession
- **Description**: Two-client TURN session setup state for a run.
- **Fields**:
  - `peerAState` (enum, required): connection lifecycle status.
  - `peerBState` (enum, required): connection lifecycle status.
  - `credentialsUsed` (boolean, required).
  - `credentialSource` (enum, required): `api|static|none`.
  - `relayTarget` (string, required): TURN endpoint.

## Entity: TurnPayloadProfile
- **Description**: Fixed payload configuration shared by web and Android.
- **Fields**:
  - `messageSizeBytes` (integer, required).
  - `messagesPerSecond` (integer, required).
  - `transferWindowSeconds` (integer, required; fixed 10).
  - `qualityThresholdRatio` (float, required): minimum delivered/attempted payload ratio for pass.

## Entity: TurnTransferMetrics
- **Description**: Measured metrics for one run.
- **Fields**:
  - `roundTripLatencyMs` (number, required for successful/partial runs).
  - `aggregateTransferRateKbps` (number, required for successful/partial runs).
  - `bytesSent` (integer, required).
  - `bytesReceived` (integer, required).
  - `deliveryQualityRatio` (float, required).
  - `qualityThresholdRatio` (float, required).
  - `windowAchievedSeconds` (number, required).
  - `profile` (`TurnPayloadProfile`, required).

## Relationships
- `TurnTransferRun` has one `TurnPeerSession`.
- `TurnTransferRun` has one `TurnTransferMetrics`.
- `TurnTransferMetrics` embeds one `TurnPayloadProfile`.

## Validation Rules
- `transferWindowSeconds` must be exactly `10`.
- `aggregateTransferRateKbps` must be derived from successful payload over achieved window duration.
- `roundTripLatencyMs` must represent round-trip (never one-way) metric semantics.
- Run is `pass` only when `deliveryQualityRatio >= qualityThresholdRatio` and window completion is successful.
- `persistenceStatus` must be `uploaded` only when backend report upload confirms success.
