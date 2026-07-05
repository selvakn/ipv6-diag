# Data Model: WireGuard Protocol Diagnostic Test

**Feature**: `010-wireguard-test` | **Phase**: 1 — Design | **Date**: 2026-07-05

---

## Entities

### WireGuardCredential (wire format — JSON)

Returned by the server credential endpoint. Contains everything a client needs to configure a WireGuard peer and connect. Mirrors `turnCredentialsResponse`.

| Field | Type | Description |
|-------|------|-------------|
| `client_private_key` | `string` (base64, 32 bytes) | Ephemeral Curve25519 private key for the client peer |
| `client_ip` | `string` (CIDR) | Allocated tunnel IP for the client (e.g. `10.0.0.2/24`) |
| `server_public_key` | `string` (base64, 32 bytes) | Server peer's Curve25519 public key for this session |
| `server_endpoint` | `string` (host:port) | Server's WireGuard UDP endpoint |
| `ttl_seconds` | `int` | Remaining session lifetime in seconds |
| `expires_at` | `string` (RFC3339) | Absolute session expiry time (UTC) |

**Uniqueness**: `client_ip` is unique per active session. `client_private_key` and `server_public_key` are generated fresh per credential request.

**Lifecycle**: Created on credential request → active for TTL → server prunes peer config on expiry.

---

### WireGuardSession (server-side in-memory)

Tracks one active WireGuard session on the server. Not persisted to SQLite — lives only in the `SessionManager`'s in-memory map.

| Field | Type | Description |
|-------|------|-------------|
| `SessionID` | `string` (UUID) | Unique session identifier |
| `ClientPublicKey` | `[32]byte` | Derived from the issued client private key |
| `ClientIP` | `net.IP` | Allocated client tunnel IP |
| `ServerPrivateKey` | `[32]byte` | Per-session server Curve25519 private key |
| `IssuedAt` | `time.Time` | Session creation timestamp |
| `ExpiresAt` | `time.Time` | Session expiry timestamp |
| `ClientID` | `string` | Client identifier (IP-based, for logging) |

**Capacity**: Max 50 concurrent sessions (configurable via `WG_MAX_SESSIONS`). IP pool: `/24` subnet (254 usable IPs).

**Cleanup**: `SessionManager.pruneLocked()` removes expired sessions on each new credential request. A background goroutine runs every 60s to reclaim leaked sessions and update the active peer list in the WireGuard device.

---

### WireGuardConfig (server runtime)

Configuration for the server WireGuard service. Populated from environment variables / flags.

| Field | Go Type | Env Var | Default | Description |
|-------|---------|---------|---------|-------------|
| `Enabled` | `bool` | `WG_ENABLED` | `false` | Enable WireGuard service |
| `Port` | `int` | `WG_PORT` | `51820` | UDP port for WireGuard protocol |
| `Subnet` | `string` | `WG_SUBNET` | `10.0.0.0/24` | IP pool subnet for client allocations |
| `ServerTunnelIP` | `string` | (derived) | first IP of subnet | Server's WireGuard tunnel IP (e.g. `10.0.0.1`) |
| `MaxSessions` | `int` | `WG_MAX_SESSIONS` | `50` | Maximum concurrent WireGuard sessions |
| `SessionTTL` | `duration` | `WG_SESSION_TTL` | `2m` | Per-session credential lifetime |
| `EchoPort` | `int` | `WG_ECHO_PORT` | `7000` | UDP echo service port (on tunnel interface) |
| `PublicEndpoint` | `string` | `WG_PUBLIC_ENDPOINT` | (auto-detected) | Public IP:port advertised to clients |

---

### WireGuardTestResult (CLI + Android result extension)

Extends the existing `TestResult` type in both CLI and Android with WireGuard-specific metrics. No new table — stored in the existing result schema using the same fields used for TURN.

**CLI** (`cli/diag/types.go` `TestResult` struct):
- `TestType`: `"wireguard"` (new constant `TestWireGuard`)
- All existing transfer metrics reused: `TransferRateKbps`, `BytesSent`, `BytesReceived`, `DeliveryQualityRatio`, `TransferWindowSeconds`, `PayloadProfile`
- `LatencyMs`: Average RTT measured from UDP echo round-trips
- `FailureReason`: `"handshake timeout"`, `"credential fetch failed"`, `"server unsupported"`, `"capacity exceeded"`, etc.

**Android** (`TestResult.kt`):
- `TestType` enum: add `WIREGUARD` value
- All existing fields reused — no schema migration needed
- `DiagnosticRunner` registers `TestType.WIREGUARD` in `baseTypes` for `TestFilter.ALL`

---

### WireGuardCredential (Go in-memory — wgmodule)

Internal Go type in `wgmodule/` representing a parsed credential for use by WireGuard peer setup.

| Field | Go Type | Description |
|-------|---------|-------------|
| `ClientPrivateKey` | `[32]byte` | Decoded from base64 `client_private_key` |
| `ClientIP` | `*net.IPNet` | Parsed from `client_ip` CIDR |
| `ServerPublicKey` | `[32]byte` | Decoded from base64 `server_public_key` |
| `ServerEndpoint` | `*net.UDPAddr` | Parsed from `server_endpoint` |
| `TTLSeconds` | `int` | For timeout calculation |

---

## State Transitions

### Server Session Lifecycle

```
Request → [capacity check] → SessionManager.Issue()
                                  ↓
                         Credential JSON returned
                         WireGuard device: peer added
                                  ↓
                         [Client connects + handshakes]
                                  ↓
                         Session active (echo traffic)
                                  ↓
                     TTL expires (prune on next request
                                  or background cleanup goroutine)
                                  ↓
                         WireGuard device: peer removed
                         IP returned to pool
```

### CLI WireGuard Test Lifecycle

```
FetchWireGuardCredentials(A) + FetchWireGuardCredentials(B)  ← parallel
         ↓
CreateWireGuardPeer(credsA) + CreateWireGuardPeer(credsB)    ← parallel
         ↓
WireGuard handshake A + WireGuard handshake B                ← parallel, timeout 5s
         ↓
Transfer window (10s): A and B concurrently send/echo
         ↓
Close peers, collect metrics, return TestResult
```

---

## Key Invariants

- A `ClientIP` is never allocated to two concurrent sessions.
- A credential's `ClientPublicKey` is uniquely registered in the WireGuard device — duplicate public keys cause the server to reject or overwrite the old session.
- WireGuard sessions MUST be cleaned up from the device's peer list when expired, or they occupy the IP pool indefinitely.
- The server's WireGuard device uses a single long-lived private key (generated at startup, not per-session) for the server peer identity.
- Client sessions use per-session server keypairs? No — the server has ONE private key (its own WireGuard identity). The `server_public_key` in the credential response is always the same server public key. Only the client keypairs are per-session.
