# Contract: TURN Credential Endpoint

## Endpoint

- **Method**: `GET`
- **Path**: `/turn/credentials`
- **Purpose**: Issue short-lived TURN credentials for client allocation attempts.

## Request

### Headers

- `Authorization: Bearer <token>` (required; existing app/server auth mechanism)

### Query Parameters

- None required in v1.

## Successful Response

- **Status**: `200 OK`
- **Body**:

```json
{
  "username": "1720000000:client",
  "password": "generated-secret",
  "realm": "androidipv6diag",
  "ttl_seconds": 300,
  "expires_at": "2026-07-02T04:00:00Z",
  "uris": [
    "turn:example.com:3478?transport=udp",
    "turn:example.com:3478?transport=tcp"
  ]
}
```

## Error Responses

- `401 Unauthorized`: missing/invalid bearer token
- `503 Service Unavailable`: TURN disabled or no active listeners
- `500 Internal Server Error`: credential generation failure

## Behavioral Rules

- Issued credentials are valid for exactly 5 minutes (`ttl_seconds=300`).
- Credential leases are tracked in memory only and not persisted to database.
- No rate limiting is applied in this feature version.
- Endpoint must return URIs matching currently active listener transports/IP families.
