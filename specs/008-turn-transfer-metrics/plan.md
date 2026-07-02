# Implementation Plan: TURN Transfer Metrics Across Clients

**Branch**: `008-turn-transfer-metrics` | **Date**: 2026-07-02 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/008-turn-transfer-metrics/spec.md`

## Summary

Upgrade TURN diagnostics in both browser and Android clients from basic allocation checks to a 10-second two-client data transfer measurement that reports round-trip latency and aggregate successful payload transfer rate. Persist run metrics to backend reports while enforcing a fixed payload profile and delivery-quality threshold for pass/fail.

## Technical Context

**Language/Version**: Go 1.25 server, browser JavaScript (ES2020+), Kotlin (Android)  
**Primary Dependencies**: Existing server report APIs; browser WebRTC APIs; Android network stack/coroutines/Room/serialization already in project  
**Storage**: Existing SQLite report store on server, existing Room DB on Android, session history in browser  
**Testing**: `go test ./...` (server), `./gradlew testDebugUnitTest` (Android), browser manual quickstart scenarios  
**Target Platform**: Linux VPS server, modern browsers, Android devices  
**Project Type**: Multi-client diagnostics platform (web + Android + Go backend)  
**Performance Goals**: 10-second TURN transfer window with stable metric capture and >95% report persistence success  
**Constraints**: Fixed payload profile across clients; explicit delivery-quality threshold; preserve backward compatibility for existing report consumers  
**Scale/Scope**: Single diagnostic run at a time per client session; repeated runs persisted for support comparison

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

Constitution document is still template-only and defines no enforceable gates.  
Initial check: PASS.  
Post-design check: PASS.

## Project Structure

### Documentation (this feature)

```text
specs/008-turn-transfer-metrics/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   └── turn-transfer-report-contract.md
└── tasks.md
```

### Source Code (repository root)

```text
server/
├── internal/handler/browser_diagnostics.go                # MODIFIED: config payload profile + quality thresholds
├── web/browser_diagnostics.html                           # MODIFIED: two-client TURN transfer, metrics, report upload
├── internal/store/report_store.go                         # existing report ingestion (compatibility verified)
└── cmd/server/main.go                                     # existing route wiring reused

android/app/src/main/
├── java/selvakn/ipv6diag/diagnostic/StunTurnTest.kt      # MODIFIED: two-client 10s TURN transfer metric probe
├── java/selvakn/ipv6diag/diagnostic/DiagnosticRunner.kt  # MODIFIED: pass config + persist run metrics
├── java/selvakn/ipv6diag/data/model/TestResult.kt        # MODIFIED: add throughput/quality metric fields
├── java/selvakn/ipv6diag/data/db/Entities.kt             # MODIFIED: Room mapping for new metric fields
├── java/selvakn/ipv6diag/data/db/AppDatabase.kt          # MODIFIED: DB migration for new fields
├── java/selvakn/ipv6diag/ui/results/ResultsScreen.kt     # MODIFIED: show transfer metrics and quality status
└── res/values/config.xml                                  # MODIFIED: payload profile defaults/threshold config
```

**Structure Decision**: Keep architecture unchanged and extend existing TURN flow in-place across web and Android paths. Persisted report schema remains backward-compatible by appending optional metric fields rather than replacing existing ones.

## Complexity Tracking

> **Fill ONLY if Constitution Check has violations that must be justified**

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| [e.g., 4th project] | [current need] | [why 3 projects insufficient] |
| [e.g., Repository pattern] | [specific problem] | [why direct DB access insufficient] |
