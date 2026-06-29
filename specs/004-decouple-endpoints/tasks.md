# Tasks: Decouple Test and Reporting Endpoints

**Input**: Design documents from `/specs/004-decouple-endpoints/`  
**Prerequisites**: `plan.md`, `spec.md`, `research.md`, `data-model.md`, `contracts/`, `quickstart.md`

**Tests**: No explicit TDD/test-first requirement in spec; this task list focuses on implementation + build/test verification.

**Organization**: Tasks are grouped by user story so each story remains independently deliverable.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: User story label (`[US1]`, `[US2]`, `[US3]`)
- Paths are repository-relative

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Establish feature docs and planning baseline.

- [x] T001 Confirm planning artifacts (`research.md`, `data-model.md`, `quickstart.md`, `contracts/endpoint-decoupling-api.md`) under `specs/004-decouple-endpoints/`
- [x] T002 Update plan context pointer in `CLAUDE.md` to `specs/004-decouple-endpoints/plan.md`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Shared model/storage changes required by all user stories.

- [x] T003 Add `testEndpointHost` to session model in `android/app/src/main/java/selvakn/ipv6diag/data/model/DiagnosticSession.kt`
- [x] T004 Persist `test_endpoint_host` in Room entity mappings in `android/app/src/main/java/selvakn/ipv6diag/data/db/Entities.kt`
- [x] T005 Update session reconstruction to preserve endpoint context in `android/app/src/main/java/selvakn/ipv6diag/data/repository/SessionRepository.kt`

**Checkpoint**: Android session model and DB layer can carry endpoint context end-to-end.

---

## Phase 3: User Story 1 - Configure test target independently (Priority: P1) 🎯 MVP

**Goal**: Use configurable probe endpoint while keeping uploads fixed to reporting host.

**Independent Test**: Save custom test endpoint, run diagnostics, and verify probes target custom endpoint while upload target remains fixed.

- [x] T006 [US1] Add app-level fixed reporting base URL helper in `android/app/src/main/java/selvakn/ipv6diag/IPv6DiagApplication.kt`
- [x] T007 [US1] Capture effective test endpoint on run session creation in `android/app/src/main/java/selvakn/ipv6diag/diagnostic/DiagnosticRunner.kt`
- [x] T008 [US1] Switch upload target to fixed reporting host in `android/app/src/main/java/selvakn/ipv6diag/ui/home/HomeScreen.kt`
- [x] T009 [US1] Add host or host:port validation for test endpoint input in `android/app/src/main/java/selvakn/ipv6diag/ui/settings/SettingsScreen.kt`

**Checkpoint**: Test endpoint config affects probes only; upload endpoint remains fixed.

---

## Phase 4: User Story 2 - Keep report-viewed client IP meaningful (Priority: P2)

**Goal**: Preserve endpoint context in uploaded reports and dashboard/API detail.

**Independent Test**: Run with a custom test endpoint and verify report detail includes that endpoint context.

- [x] T010 [US2] Extend upload payload with `test_endpoint` in `android/app/src/main/java/selvakn/ipv6diag/upload/CloudUploader.kt`
- [x] T011 [US2] Add `test_endpoint` to server upload/request/detail structs in `server/internal/store/report_store.go`
- [x] T012 [US2] Persist and read `test_endpoint` in SQLite queries in `server/internal/store/report_store.go`
- [x] T013 [US2] Add DB migration for `test_endpoint` column in `server/internal/store/db.go`
- [x] T014 [US2] Show test endpoint context in dashboard detail view in `server/web/dashboard.html`

**Checkpoint**: Dashboard/API consumers can see exact probe endpoint used per report.

---

## Phase 5: User Story 3 - Safe fallback behavior (Priority: P3)

**Goal**: Keep deterministic behavior with default endpoint and no silent probe fallback.

**Independent Test**: Reset endpoint, run diagnostics, verify default endpoint is used and no fallback retries are introduced.

- [x] T015 [US3] Ensure default test endpoint assumption is reflected in settings save/reset UX in `android/app/src/main/java/selvakn/ipv6diag/ui/settings/SettingsScreen.kt`
- [x] T016 [US3] Confirm no probe fallback logic is introduced and upload continues on failures in `android/app/src/main/java/selvakn/ipv6diag/ui/home/HomeScreen.kt` and `android/app/src/main/java/selvakn/ipv6diag/upload/CloudUploader.kt`

**Checkpoint**: Default/failure semantics align with clarified requirements.

---

## Phase 6: Polish & Cross-Cutting Concerns

- [x] T017 [P] Update feature docs consistency between spec/plan/contracts in `specs/004-decouple-endpoints/`
- [x] T018 Run server test verification with `go test ./...` from `server/`
- [x] T019 Run Android build verification with `mise exec -- ./gradlew assembleDebug` from `android/`
- [x] T020 Mark completed tasks and summarize implementation outcomes in `specs/004-decouple-endpoints/tasks.md`

---

## Dependencies & Execution Order

### Phase Dependencies

- Phase 1 → Phase 2 → Phase 3 → Phase 4 → Phase 5 → Phase 6
- User story phases depend on foundational model/persistence updates from Phase 2.

### User Story Dependencies

- **US1 (P1)**: Starts after foundational updates.
- **US2 (P2)**: Depends on US1 session/upload behavior for endpoint context propagation.
- **US3 (P3)**: Depends on US1 settings/run behavior and confirms non-fallback semantics.

### Parallel Opportunities

- `T017` can run in parallel with verification prep.
- Verification tasks (`T018`, `T019`) can be executed independently after implementation completes.

---

## Implementation Strategy

### MVP First (US1)

1. Complete Phases 1-2.
2. Complete US1 tasks (`T006`-`T009`).
3. Validate custom test endpoint + fixed reporting upload behavior.

### Incremental Delivery

1. Add US2 server/dashboard context propagation.
2. Confirm US3 default/non-fallback semantics.
3. Execute final verification and update task tracking.
