# Research: Cloud Diagnostic Report Dashboard

## SQLite Driver for Go (no CGO)

**Decision**: `modernc.org/sqlite` v1.x
**Rationale**: Pure-Go port of SQLite — no C toolchain, no CGO, cross-compiles cleanly inside Docker. Mature (used in production by multiple projects). API is `database/sql` compatible so standard `database/sql` patterns apply.
**Alternatives considered**:
- `github.com/mattn/go-sqlite3` — CGO, faster, but requires gcc in Docker image and complicates cross-compilation
- `crawshaw.io/sqlite` — CGO-based low-level API, overkill for this use case

## Web Dashboard Embedding in Go

**Decision**: `//go:embed web/dashboard.html` via Go standard `embed` package (Go 1.16+)
**Rationale**: Zero build steps, no npm/webpack, single binary deployment. Vanilla HTML+JS is sufficient for a list+detail dashboard with date/device filters. The existing server already ships as a single binary.
**Alternatives considered**:
- Separate frontend repo (React/Vue) — overkill; adds build pipeline complexity
- Template-based server-side rendering — more Go code to maintain; JS fetch approach simpler for a single-page list/detail pattern

## Android Device Identifier

**Decision**: `android.provider.Settings.Secure.ANDROID_ID`
**Rationale**: Stable per-device identifier (non-PII, reset on factory reset), available without permissions on API 26+. Used to correlate reports from the same device.
**Alternatives considered**:
- `Build.SERIAL` — requires `READ_PHONE_STATE` permission on API 26+, rejected
- Random UUID persisted in SharedPreferences — works but not stable across app reinstalls without backup

## Upload Trigger Point (Android)

**Decision**: Trigger in `HomeScreen.kt` immediately after `runner.runTests()` returns, in the same coroutine scope
**Rationale**: No ViewModel refactor needed; existing pattern is scope.launch in composable. Upload is fire-and-forget — the coroutine launches upload and immediately navigates to ResultsScreen; upload status is tracked via a shared `MutableStateFlow<Map<String, UploadStatus>>` held in `IPv6DiagApplication`.
**Alternatives considered**:
- Trigger inside `DiagnosticRunner.executeTests()` — mixes network I/O concerns; runner is already complex
- Trigger in `ResultsScreen.LaunchedEffect` — too late; user sees results before upload starts

## Upload Payload Format

**Decision**: JSON matching the server's `UploadRequest` struct; Android serialises using existing kotlinx.serialization
**Rationale**: Consistent with existing app serialization; server uses standard `encoding/json`
**Schema**: `session_id`, `device` (object), `network` (NetworkInfo JSON), `test_results` (array), `xlat_summary` (nullable object), `pass_count`, `total_count`, `run_timestamp`

## Dashboard Filter Design

**Decision**: Client-side filtering in JS for v1 (fetch all ≤200 records, filter in browser)
**Rationale**: Report volumes are small in v1 (internal tool, limited fleet). Avoids server-side query complexity. If volume grows, server-side filtering is already wired into the GET /reports query params.
