# Quickstart: Decouple Test and Reporting Endpoints

## Scenario A: Custom test endpoint with fixed reporting

1. Open app Settings and save test endpoint as `qa-probe.example.net:8080`.
2. Run diagnostics from Home screen.
3. Verify probe execution targets the configured test endpoint.
4. Verify upload still posts to reporting host (`androidipv6diag.fly.dev`).
5. Open dashboard report detail and confirm `test_endpoint` shows `qa-probe.example.net:8080`.

## Scenario B: No custom endpoint (default behavior)

1. Reset settings to default endpoint.
2. Run diagnostics.
3. Verify tests run against default host `androidipv6diag.fly.dev`.
4. Verify report upload succeeds and dashboard displays default `test_endpoint`.

## Scenario C: Probe target failure with upload continuity

1. Configure an intentionally unreachable test endpoint (e.g., `badhost.invalid:1234`).
2. Run diagnostics and observe probe failures.
3. Verify app does not fallback probes to another endpoint.
4. Verify upload still occurs to fixed reporting host.
5. Confirm dashboard shows failed probe results and captured `test_endpoint`.

## API spot-check

```bash
curl -X POST http://localhost/reports \
  -H "Content-Type: application/json" \
  -d '{
    "session_id": "test-endpoint-1",
    "device": {"name":"Test","model":"Test","manufacturer":"Test","android_version":"14","device_id":"test"},
    "network": {},
    "test_results": [],
    "xlat_summary": null,
    "pass_count": 0,
    "total_count": 0,
    "run_timestamp": 1751000000000,
    "test_endpoint": "qa-probe.example.net:8080"
  }'

curl http://localhost/reports/test-endpoint-1
```
