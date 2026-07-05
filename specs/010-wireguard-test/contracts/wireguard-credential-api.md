# Contract: WireGuard Credential API

**Feature**: `010-wireguard-test` | **Version**: v1 | **Date**: 2026-07-05

---

## Endpoint

```
GET /wireguard/credentials
```

Mirrors the existing `/turn/credentials` endpoint in structure and auth.

---

## Authentication

**Header**: `Authorization: Bearer <token>`

- Token is the same shared secret configured via `--token` / `TOKEN` env var on the server.
- When the server has no token configured, the endpoint is open (same as TURN).
- Missing or invalid token → `401 Unauthorized`.

---

## Response: 200 OK

```json
{
  "client_private_key": "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=",
  "client_ip":          "10.0.0.2/24",
  "server_public_key":  "BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB=",
  "server_endpoint":    "198.51.100.1:51820",
  "ttl_seconds":        120,
  "expires_at":         "2026-07-05T17:02:00Z"
}
```

| Field | Type | Constraints |
|-------|------|-------------|
| `client_private_key` | `string` | Base64-standard-encoded, 32 bytes decoded (Curve25519 scalar) |
| `client_ip` | `string` | CIDR notation, within `WG_SUBNET` |
| `server_public_key` | `string` | Base64-standard-encoded, 32 bytes decoded (Curve25519 point) |
| `server_endpoint` | `string` | `host:port` — same host as request, port from `WG_PORT` |
| `ttl_seconds` | `int` | Positive integer; remaining session lifetime |
| `expires_at` | `string` | RFC3339 UTC timestamp |

---

## Error Responses

| Status | Body `error` field | Condition |
|--------|-------------------|-----------|
| `401` | `"missing or invalid authorization"` | Bad or missing Bearer token |
| `503` | `"wireguard service unavailable"` | `WG_ENABLED=false` or service failed to start |
| `503` | `"wireguard session capacity exceeded"` | Active sessions ≥ `WG_MAX_SESSIONS` (default 50) |
| `500` | `"failed to issue credentials"` | Key generation or IP allocation error |

All error responses use the same JSON envelope as the rest of the server:
```json
{ "error": "<reason>" }
```

---

## Server Behaviour on Credential Issue

1. Validate Bearer token (if configured).
2. Check active session count ≤ `WG_MAX_SESSIONS`. If not → 503.
3. Prune expired sessions.
4. Allocate the next available IP from `WG_SUBNET` (sequential, wrapping after pool exhausted by cleanup).
5. Generate a fresh Curve25519 keypair for the client (client's private key is returned, public key derived and registered in WireGuard device).
6. Register the client's public key as a WireGuard peer in the server device with AllowedIPs = `<client_ip>/32`.
7. Return the credential JSON.

---

## WireGuard Peer Config Implied by Credential

The client should configure its WireGuard peer entry for the server as:

```ini
[Interface]
PrivateKey = <client_private_key>
Address    = <client_ip>

[Peer]
PublicKey  = <server_public_key>
Endpoint   = <server_endpoint>
AllowedIPs = 0.0.0.0/0, ::/0
```

(In practice this is constructed programmatically in `wgmodule/` — no config file is written.)

---

## Echo Service

When connected through the WireGuard tunnel, the server runs a UDP echo service on:
```
<server_tunnel_ip>:7000
```

where `<server_tunnel_ip>` is the first usable IP of `WG_SUBNET` (e.g. `10.0.0.1` for `10.0.0.0/24`).

Clients send arbitrary UDP datagrams to this address; the echo service reflects them back unchanged. This is the data path used by the WireGuard transfer test.

---

## Session Cleanup

- Sessions are pruned lazily on each credential request (same pattern as `CredentialManager.pruneLocked()` in `server/internal/turn/credentials.go`).
- A background goroutine also prunes every 60 seconds and removes stale peers from the WireGuard device.
- After pruning, the allocated `client_ip` is returned to the pool and can be reallocated.

---

## Observability Events (Structured Logs)

| Event | Log Level | Fields |
|-------|-----------|--------|
| Session created | `INFO` | `session_id`, `client_ip`, `expires_at` |
| Session expired (lazy prune) | `DEBUG` | `session_id`, `client_ip` |
| Session expired (background prune) | `INFO` | `session_id`, `client_ip` |
| Capacity limit hit | `WARN` | `active_sessions`, `max_sessions` |
| Peer registration failed | `ERROR` | `session_id`, `error` |

Active session count is exposed in `GET /health` response:
```json
{ ..., "wg_active_sessions": 3 }
```
