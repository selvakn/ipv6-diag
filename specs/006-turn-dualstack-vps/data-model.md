# Data Model: Embedded TURN Relay Service

## Entity: TurnServiceConfiguration

- **Purpose**: Runtime settings for enabling and binding TURN listeners and credential behavior.
- **Fields**:
  - `enabled` (bool): global TURN enable flag
  - `realm` (string): TURN realm presented to clients
  - `public_ipv4` (string, optional): advertised external IPv4 relay address
  - `public_ipv6` (string, optional): advertised external IPv6 relay address
  - `udp4_listen` (string): IPv4 UDP bind address (`host:port`)
  - `udp6_listen` (string): IPv6 UDP bind address (`[host]:port`)
  - `tcp4_listen` (string): IPv4 TCP bind address
  - `tcp6_listen` (string): IPv6 TCP bind address
  - `credential_ttl_seconds` (int): lease lifetime (fixed to 300 in v1)
  - `credentials_endpoint_enabled` (bool)
- **Validation rules**:
  - At least one listener must be configured when `enabled=true`
  - TTL must equal 300 in this feature version
  - Bind addresses must parse as valid socket endpoints

## Entity: TurnListenerStatus

- **Purpose**: Listener runtime health snapshot surfaced via logs/status output.
- **Fields**:
  - `listener_key` (enum-like string): `udp4`, `udp6`, `tcp4`, `tcp6`
  - `bind_address` (string)
  - `state` (string): `active` or `degraded`
  - `error_message` (string, optional): bind/start failure detail when degraded
  - `started_at` (timestamp, optional)
- **State transitions**:
  - `pending -> active` on successful start
  - `pending -> degraded` on start failure

## Entity: TurnCredentialLease

- **Purpose**: Short-lived credential data issued to clients.
- **Fields**:
  - `username` (string)
  - `password` (string)
  - `realm` (string)
  - `issued_at` (timestamp)
  - `expires_at` (timestamp, `issued_at + 5 minutes`)
  - `client_id` (string, optional): caller identity/context for observability
- **Validation rules**:
  - Lease is valid only when current time < `expires_at`
  - Lease must be removed/ignored after expiration
  - No persistent storage writes allowed for leases

## Entity: TurnAllocationAttempt

- **Purpose**: Operational record emitted in logs for allocation attempts.
- **Fields**:
  - `request_id` (string)
  - `transport` (string): `udp` or `tcp`
  - `ip_family` (string): `ipv4` or `ipv6`
  - `client_addr` (string)
  - `result` (string): `success` or `failed`
  - `reason` (string, optional)
  - `timestamp` (timestamp)

## Relationships

- `TurnServiceConfiguration` drives creation of multiple `TurnListenerStatus` entries.
- `TurnCredentialLease` instances are issued using `TurnServiceConfiguration.realm`.
- `TurnAllocationAttempt` references transport/IP-family chosen by active listeners and credentials validated against in-memory leases.
