# Quickstart: Cloud Diagnostic Report Dashboard

## End-to-End Integration Scenario

### 1. Start the server with a DB file

```bash
# From repo root
./server/bin/server --db ./reports.db
# Dashboard available at http://localhost/dashboard
```

Or via Docker Compose (db file mounted as volume):
```bash
docker compose up
```

### 2. Android app auto-uploads after every run

Run any diagnostic in the app — the Results screen shows:
- "Uploading…" spinner → "Uploaded ✓" on success
- "Upload failed" in red if server is unreachable (report still saved locally)

### 3. Open the dashboard

Navigate to `http://<server-ip>/dashboard` in any browser.

- **List view**: all uploaded reports, newest first
- **Filter**: pick a date range and/or type a device name substring → list updates instantly
- **Detail view**: click any row to see full device info, network snapshot, test results, and 464XLAT section

### Verify with curl

```bash
# List all reports
curl http://localhost/reports

# Upload a test report
curl -X POST http://localhost/reports \
  -H "Content-Type: application/json" \
  -d '{
    "session_id": "test-123",
    "device": {"name":"Test","model":"Test","manufacturer":"Test","android_version":"14","device_id":"test"},
    "network": {"mobileDataEnabled":true,"cellularIPv4Address":"10.0.0.1","cellularIPv6Addresses":[],"hasNativeIPv6":false,"clatPresent":false,"dnsServers":[],"dnsServerNames":[],"apiLevel":34},
    "test_results": [],
    "xlat_summary": null,
    "pass_count": 0,
    "total_count": 0,
    "run_timestamp": 1751000000000
  }'

# Get the report back
curl http://localhost/reports/test-123
```
