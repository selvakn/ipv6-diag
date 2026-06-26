# Tasks: Cloud Diagnostic Report Dashboard

## Phase 1 — Server Foundational (Storage & API)

- [x] T001 Add `modernc.org/sqlite` dependency to server/go.mod and run `go mod tidy`
- [x] T002 Create `server/internal/store/db.go` — open SQLite, create `reports` table + indexes on init
- [x] T003 Create `server/internal/store/report_store.go` — `ReportStore` with `Upsert`, `List` (date/device filters), `GetByID`
- [x] T004 [P] Create `server/internal/handler/reports.go` — `POST /reports`, `GET /reports`, `GET /reports/{id}` handlers wired to `ReportStore`
- [x] T005 [P] Create `server/web/dashboard.html` — single-page HTML+CSS+JS dashboard (list view with date+device filters, detail view on row click, back navigation)
- [x] T006 [P] Create `server/internal/handler/dashboard.go` — `GET /dashboard` handler serving embedded `web/dashboard.html` via `//go:embed`
- [x] T007 Modify `server/cmd/server/main.go` — add `--db` flag (default `./reports.db`), initialise store, register `/reports`, `/reports/`, `/dashboard` routes on both HTTP muxes

## Phase 2 — Android: Device Info & Upload Client (US1)

- [x] T008 [P] [US1] Create `android/app/src/main/java/com/lenovo/mesh/ipv6diag/data/model/DeviceInfo.kt` — `@Serializable data class DeviceInfo(name, model, manufacturer, androidVersion, deviceId)`
- [x] T009 [P] [US1] Create `android/app/src/main/java/com/lenovo/mesh/ipv6diag/diagnostic/DeviceInfoCollector.kt` — collects `Build.MODEL`, `Build.MANUFACTURER`, `Build.VERSION.RELEASE`, `Settings.Secure.ANDROID_ID`; derives display name from `Build.MODEL`
- [x] T010 [US1] Create `android/app/src/main/java/com/lenovo/mesh/ipv6diag/upload/UploadStatus.kt` — `sealed class UploadStatus { Idle, Uploading, Success, Failed(reason) }`
- [x] T011 [US1] Create `android/app/src/main/java/com/lenovo/mesh/ipv6diag/upload/CloudUploader.kt` — `suspend fun upload(session, device, xlatSummary?, serverUrl): UploadStatus`; serialises `CloudUploadRequest` via kotlinx.serialization; POSTs to `POST {serverUrl}/reports`; returns `Success` or `Failed`
- [x] T012 [US1] Modify `android/app/src/main/java/com/lenovo/mesh/ipv6diag/upload/CloudUploader.kt` — add `CloudUploadRequest @Serializable` data class (`session_id`, `device`, `network`, `test_results`, `xlat_summary?`, `pass_count`, `total_count`, `run_timestamp`)
- [x] T013 [US1] Modify `android/app/src/main/java/com/lenovo/mesh/ipv6diag/IPv6DiagApplication.kt` — instantiate `CloudUploader`; expose `uploadStatus: MutableStateFlow<Map<String, UploadStatus>>`
- [x] T014 [US1] Modify `android/app/src/main/java/com/lenovo/mesh/ipv6diag/ui/home/HomeScreen.kt` — after `runner.runTests()` succeeds, launch coroutine to call `CloudUploader.upload()` and update `app.uploadStatus`
- [x] T015 [US1] Modify `android/app/src/main/java/com/lenovo/mesh/ipv6diag/ui/results/ResultsScreen.kt` — collect `app.uploadStatus[sessionId]` as state; show upload chip: "Uploading…" / "Uploaded ✓" / "Upload failed"

## Phase 3 — Build Verification

- [x] T016 Run `cd server && go build ./...` and confirm no errors
- [x] T017 Run `mise exec -- sh -c "cd android && ./gradlew assembleDebug"` and confirm BUILD SUCCESSFUL
