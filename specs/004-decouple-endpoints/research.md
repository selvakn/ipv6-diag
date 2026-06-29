# Research: Decouple Test and Reporting Endpoints

## Fixed Reporting Endpoint While Test Endpoint Varies

**Decision**: Keep report uploads hard-wired to `androidipv6diag.fly.dev` and decouple only probe execution target.  
**Rationale**: This preserves centralized report intake and dashboard continuity while enabling QA/operator validation against alternate probe targets.  
**Alternatives considered**:
- Reuse active test endpoint for uploads — rejected because it fragments reporting and can break dashboard visibility.
- Dual configurable endpoints for both tests and uploads — rejected for this iteration because reporting must remain fixed by requirement.

## Test Endpoint Input Validation

**Decision**: Accept only `host` or `host:port` (no scheme/path/query).  
**Rationale**: Simple input shape prevents malformed URLs, keeps UX concise, and aligns with diagnostics code that already derives protocol/ports separately.  
**Alternatives considered**:
- Full URL input — rejected due to ambiguity around scheme/path handling and extra validation complexity.
- Curated preset list only — rejected because operators need ad-hoc environments.

## Probe Failure Handling

**Decision**: Do not fallback failed probes to another endpoint; still upload report.  
**Rationale**: Falling back hides environment-specific failures and pollutes test signal quality. Upload continuity still preserves observability.  
**Alternatives considered**:
- Automatic fallback to default endpoint — rejected because it can produce false positives.
- Prompt user during run — rejected for added UX friction in an automated flow.

## Capturing Endpoint Context in Reports

**Decision**: Include `test_endpoint` in upload payload and persist it server-side with each report.  
**Rationale**: Dashboard users need explicit context to interpret `/diag` client-IP observations when reporting and test paths differ.  
**Alternatives considered**:
- Infer from uploaded network fields only — rejected because endpoint source remains ambiguous.
- Store only on device, not on server — rejected because dashboard consumers would still lack context.

## Backward Compatibility for Existing Reports

**Decision**: Add nullable server DB field for `test_endpoint` and default to empty/unknown for old records.  
**Rationale**: Keeps historical data readable while enabling new context for future runs.  
**Alternatives considered**:
- Hard migration requiring full backfill — rejected due to unnecessary complexity for internal tooling scope.
