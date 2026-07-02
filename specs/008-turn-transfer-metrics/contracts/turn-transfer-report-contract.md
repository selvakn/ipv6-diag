# Contract: TURN Transfer Report Payload Extension

## Scope
Extend existing `/reports` upload payloads so both web and Android clients can persist TURN transfer metrics without breaking existing report consumers.

## Endpoint
- `POST /reports`

## Request Contract (additions)
`test_results[]` entries for TURN runs MAY include these additional fields:

```json
{
  "testType": "TURN",
  "status": "PASS",
  "latencyMs": 84,
  "transferRateKbps": 512.7,
  "bytesSent": 655360,
  "bytesReceived": 642048,
  "deliveryQualityRatio": 0.98,
  "qualityThresholdRatio": 0.9,
  "transferWindowSeconds": 10,
  "payloadProfile": "1024B@50Hz"
}
```

## Behavioral Rules
- Existing non-TURN test results remain unchanged.
- Existing required fields for report upload (`session_id`, `device`, `network`, `test_results`, counts, timestamps) remain unchanged.
- Consumers must tolerate missing new TURN fields for historical records.
- TURN result is considered successful only when `deliveryQualityRatio >= qualityThresholdRatio` and transfer window reaches 10 seconds.

## Compatibility
- Server persists `test_results` as JSON blob; no schema-breaking DB migration required server-side.
- Dashboard/report consumers should treat unknown fields as optional additions.

## Error Cases
- If report upload fails, clients retain local run metrics and flag persistence status as failed.
- If TURN test aborts before minimum viable data, clients include failure reason and quality metrics where available.
