# Implementation Plan: Cloud Diagnostic Report Dashboard

**Branch**: `003-cloud-report-dashboard` | **Date**: 2026-06-26 | **Spec**: [spec.md](spec.md)

## Summary

Extend the existing Go server with a SQLite-backed report store and a browser-based dashboard. Extend the Android app to automatically upload each completed diagnostic session to the server and display an upload status on the Results screen.

## Technical Context

**Language/Version**: Go 1.24 (server), Kotlin/Android API 26+ (app)
**Primary Dependencies**: `modernc.org/sqlite` (pure-Go SQLite driver, no CGO); existing OkHttp3 + Kotlin coroutines (Android)
**Storage**: SQLite file on server via `modernc.org/sqlite v1.x`; file path configurable via `--db` flag (default `./reports.db`)
**Testing**: `go test ./...` (server); `mise exec -- ./gradlew assembleDebug` (Android build verification)
**Target Platform**: Linux Docker container (server); Android API 26+ (app)
**Project Type**: Web service (server extension) + mobile app (Android extension)
**Performance Goals**: Upload completes within 5 s on stable connection; dashboard list loads within 3 s
**Constraints**: No CGO (pure-Go SQLite); no auth in v1; upload is fire-and-forget with retry on failure
**Scale/Scope**: Single SQLite file; no pagination required for v1 (limit=200 default)

## Constitution Check

Constitution is a placeholder template — no active gates. No violations.

## Project Structure

### Documentation (this feature)

```text
specs/003-cloud-report-dashboard/
├── plan.md              ← this file
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   └── report-api.md
└── tasks.md             ← created by /speckit-tasks
```

### Source Code

```text
server/
├── cmd/server/main.go                    ← MODIFIED: add --db flag, wire store + new handlers
├── internal/
│   ├── handler/
│   │   ├── diag.go                       ← existing (unchanged)
│   │   ├── health.go                     ← existing (unchanged)
│   │   ├── reports.go                    ← NEW: POST /reports, GET /reports, GET /reports/{id}
│   │   └── dashboard.go                  ← NEW: GET /dashboard (serves embedded HTML)
│   ├── listener/dual_stack.go            ← existing (unchanged)
│   └── store/
│       ├── db.go                         ← NEW: SQLite open/init, schema migration
│       └── report_store.go               ← NEW: Upsert, List, GetByID
├── web/
│   └── dashboard.html                    ← NEW: single-page HTML+CSS+JS dashboard
└── go.mod / go.sum                       ← MODIFIED: add modernc.org/sqlite

android/app/src/main/java/com/lenovo/mesh/ipv6diag/
├── data/model/
│   └── DeviceInfo.kt                     ← NEW: DeviceInfo data class (@Serializable)
├── diagnostic/
│   └── DeviceInfoCollector.kt            ← NEW: collects Build.* fields + Settings.Secure ID
├── upload/
│   └── CloudUploader.kt                  ← NEW: POST /reports with retry-on-failure
└── ui/home/
    └── HomeScreen.kt                     ← MODIFIED: launch upload after runTests() returns
└── ui/results/
    └── ResultsScreen.kt                  ← MODIFIED: show upload status chip
```

## Key Design Decisions

### Server-side storage (SQLite schema)

Single `reports` table — JSON columns for nested data (network snapshot, test results, XLAT summary) avoids schema churn when sub-models evolve:

```sql
CREATE TABLE IF NOT EXISTS reports (
    id                 TEXT PRIMARY KEY,
    device_name        TEXT NOT NULL,
    device_model       TEXT NOT NULL,
    device_manufacturer TEXT NOT NULL,
    android_version    TEXT NOT NULL,
    device_id          TEXT NOT NULL,
    network_json       TEXT NOT NULL,
    test_results_json  TEXT NOT NULL,
    xlat_summary_json  TEXT,
    pass_count         INTEGER NOT NULL,
    total_count        INTEGER NOT NULL,
    run_timestamp      INTEGER NOT NULL,
    uploaded_at        INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_reports_uploaded_at ON reports(uploaded_at);
CREATE INDEX IF NOT EXISTS idx_reports_device_name ON reports(device_name);
```

### Upload trigger (Android)

Upload fires in `HomeScreen.kt` immediately after `runner.runTests()` returns, inside the same coroutine scope. No ViewModel refactor needed — the existing pattern (scope.launch in composable) is sufficient.

### Upload status state

`UploadStatus` sealed class held in a `mutableStateOf` in `HomeScreen` and passed as a navigation argument or stored in the repository after upload completes. Simplest approach: store upload status in a `MutableStateFlow` keyed by session ID inside `CloudUploader`, read by `ResultsScreen` via `LaunchedEffect`.

### Web dashboard

Embedded in the Go binary via `//go:embed web/dashboard.html`. Single HTML file with vanilla JS — fetches `/reports` JSON and renders a table. No build step, no npm, no framework.

### Retry on upload failure

`CloudUploader` wraps the POST in a `runCatching`. On failure it logs the error and returns a `Failed` status. No background retry worker in v1 — the status shown on ResultsScreen allows the user to be aware; automatic retry is deferred.
