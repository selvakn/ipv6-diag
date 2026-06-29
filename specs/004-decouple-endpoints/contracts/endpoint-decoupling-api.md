# API Contract: Endpoint Decoupling Additions

Base URL: `http://{server-host}:{port}`

## POST /reports (extended payload)

**New request field**

| Field | Type | Required | Description |
|------|------|----------|-------------|
| `test_endpoint` | string | Yes (new uploads) | Effective test probe endpoint used for this run, format `host` or `host:port` |

**Extended request example**

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
  "network": {},
  "test_results": [],
  "xlat_summary": null,
  "pass_count": 0,
  "total_count": 0,
  "run_timestamp": 1751000000000,
  "test_endpoint": "qa-probe.example.net:8080"
}
```

## GET /reports/{id} (extended response)

**New response field**

| Field | Type | Description |
|------|------|-------------|
| `test_endpoint` | string \| null | Probe endpoint context for this report; null/empty for legacy reports |

**Extended response example**

```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "device_name": "Pixel 7",
  "device_model": "Pixel 7",
  "device_manufacturer": "Google",
  "android_version": "14",
  "device_id": "a1b2c3d4e5f6",
  "pass_count": 0,
  "total_count": 0,
  "run_timestamp": 1751000000000,
  "uploaded_at": 1751000005000,
  "test_endpoint": "qa-probe.example.net:8080",
  "network": {},
  "test_results": [],
  "xlat_summary": null
}
```

## Compatibility Notes

- Existing reports may omit `test_endpoint`; consumers must treat missing/empty values as legacy data.
- Report upload host remains fixed; `test_endpoint` captures probe target only.
