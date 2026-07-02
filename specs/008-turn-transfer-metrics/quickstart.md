# Quickstart: TURN Transfer Metrics Across Clients

## Prerequisites
- TURN server reachable at configured endpoint with valid credentials.
- Server running and exposing diagnostics + reports endpoints.
- Android debug/release build installable on test device.
- Browser with WebRTC support.

## 1) Start/verify server
```bash
cd server
GOCACHE="../.gocache" go run ./cmd/server
```

Verify:
- `GET /browser-diagnostics`
- `GET /browser-diagnostics/config`
- `POST /reports` available

## 2) Run web TURN transfer test (US1)
- Open `/browser-diagnostics`.
- Run TURN test with default/fixed payload profile.
- Confirm:
  - Two-client relay session established
  - 10-second transfer window executed
  - Fixed payload profile is applied (message size + cadence)
  - Round-trip latency and aggregate transfer-rate shown
  - Delivery quality threshold evaluated for pass/fail

## 3) Run Android TURN transfer test (US2)
- Start diagnostics in app using STUN/TURN mode (or full mode).
- Confirm TURN results include:
  - 10-second transfer metrics
  - Same fixed payload profile semantics as web
  - Round-trip latency
  - Aggregate transfer rate
  - Delivery-quality threshold decision

## 4) Verify backend persistence (US2/US3)
- Confirm both web and Android runs are uploaded to `/reports`.
- Verify TURN test-result entries include transfer metric fields.
- Confirm historical runs remain viewable and comparable.

## 5) Validate repeated-run comparison (US3)
- Execute at least two runs under stable network conditions.
- Ensure run-to-run metric consistency remains within expected tolerance.
- Simulate degraded network and verify failure/partial labeling when quality threshold is missed.
