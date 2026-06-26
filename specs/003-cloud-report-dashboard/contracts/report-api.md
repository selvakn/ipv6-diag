# API Contract: Report Endpoints

Base URL: `http://{server-host}:{port}`

---

## POST /reports

Upload a diagnostic report from the Android app.

**Request**
- Method: `POST`
- Content-Type: `application/json`

```json
{
  "session_id": "550e8400-e29b-41d4-a716-446655440000",
  "device": {
    "name": "Pixel 7",
    "model": "Pixel 7",
    "manufacturer": "Google",
    "android_version": "14",
    "device_id": "a1b2c3d4e5f6"
  },
  "network": {
    "mobileDataEnabled": true,
    "cellularInterfaceName": "rmnet0",
    "cellularIPv4Address": "10.0.0.1",
    "cellularIPv6Addresses": ["2001:db8::1"],
    "hasNativeIPv6": true,
    "clatPresent": false,
    "clatInterfaceName": null,
    "clatSyntheticIPv4": null,
    "dnsServers": ["8.8.8.8"],
    "dnsServerNames": ["dns.google"],
    "apiLevel": 34
  },
  "test_results": [
    {
      "sessionId": "550e8400-e29b-41d4-a716-446655440000",
      "testType": "HTTP",
      "addressFamily": "IPv4",
      "status": "PASS",
      "latencyMs": 42,
      "resolvedAddress": "203.0.113.1",
      "serverConfirmedFamily": "IPv4",
      "packetLoss": null,
      "failureReason": null
    }
  ],
  "xlat_summary": null,
  "pass_count": 1,
  "total_count": 1,
  "run_timestamp": 1751000000000
}
```

**Response — 200 OK** (upsert succeeded)
```json
{ "status": "ok", "id": "550e8400-e29b-41d4-a716-446655440000" }
```

**Response — 400 Bad Request** (malformed JSON or missing required fields)
```json
{ "error": "invalid request: missing session_id" }
```

**Response — 500 Internal Server Error**
```json
{ "error": "failed to save report" }
```

---

## GET /reports

List report summaries with optional filters.

**Query Parameters**

| Parameter | Type | Description |
|-----------|------|-------------|
| `from` | ISO date `YYYY-MM-DD` | Filter: uploaded_at ≥ start of this day (UTC) |
| `to` | ISO date `YYYY-MM-DD` | Filter: uploaded_at ≤ end of this day (UTC) |
| `device` | string | Filter: device_name contains this substring (case-insensitive) |
| `limit` | integer | Max results (default 200, max 500) |
| `offset` | integer | Pagination offset (default 0) |

**Response — 200 OK**
```json
{
  "reports": [
    {
      "id": "550e8400-e29b-41d4-a716-446655440000",
      "device_name": "Pixel 7",
      "device_model": "Pixel 7",
      "pass_count": 1,
      "total_count": 1,
      "run_timestamp": 1751000000000,
      "uploaded_at": 1751000005000
    }
  ],
  "total": 1
}
```

---

## GET /reports/{id}

Retrieve the full detail of one report by session ID.

**Path Parameter**: `id` — session UUID

**Response — 200 OK**
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "device_name": "Pixel 7",
  "device_model": "Pixel 7",
  "device_manufacturer": "Google",
  "android_version": "14",
  "device_id": "a1b2c3d4e5f6",
  "pass_count": 1,
  "total_count": 1,
  "run_timestamp": 1751000000000,
  "uploaded_at": 1751000005000,
  "network": { ... },
  "test_results": [ ... ],
  "xlat_summary": null
}
```

**Response — 404 Not Found**
```json
{ "error": "report not found" }
```

---

## GET /dashboard

Serves the embedded single-page HTML dashboard. No query parameters.

**Response — 200 OK**: `text/html` page that renders the report list and detail view using the above JSON endpoints via `fetch()`.
