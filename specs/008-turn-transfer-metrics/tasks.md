# Tasks: TURN Transfer Metrics Across Clients

**Input**: Design documents from `/specs/008-turn-transfer-metrics/`
**Prerequisites**: `plan.md`, `spec.md`, `research.md`, `data-model.md`, `contracts/turn-transfer-report-contract.md`, `quickstart.md`

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Define shared TURN transfer metric configuration and scaffolding.

- [X] T001 Add TURN transfer payload profile config fields in `server/internal/handler/browser_diagnostics.go`
- [X] T002 Add Android TURN transfer profile defaults in `android/app/src/main/res/values/config.xml`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Add shared data-model fields and persistence wiring needed by all user stories.

- [X] T003 Extend `TestResult` model with transfer metric fields in `android/app/src/main/java/selvakn/ipv6diag/data/model/TestResult.kt`
- [X] T004 Add Room entity columns for transfer metrics in `android/app/src/main/java/selvakn/ipv6diag/data/db/Entities.kt`
- [X] T005 Add DB migration for new test result columns in `android/app/src/main/java/selvakn/ipv6diag/data/db/AppDatabase.kt`
- [X] T006 Ensure report upload payload carries extended TURN metric fields in `android/app/src/main/java/selvakn/ipv6diag/upload/CloudUploader.kt`

**Checkpoint**: Shared metric fields persist locally and are upload-ready.

---

## Phase 3: User Story 1 - Measure TURN Transfer Metrics in Web Client (Priority: P1) 🎯 MVP

**Goal**: Implement two-client 10-second TURN transfer test with RTT and aggregate throughput in web diagnostics.

**Independent Test**: Run TURN test in browser diagnostics and verify two-client relay flow, 10-second transfer, metric output, and threshold-based status.

- [X] T007 [US1] Implement two-peer TURN transfer session orchestration in `server/web/browser_diagnostics.html`
- [X] T008 [US1] Implement fixed payload send/receive loop for 10-second window in `server/web/browser_diagnostics.html`
- [X] T009 [US1] Compute and render round-trip latency + aggregate transfer rate + quality ratio in `server/web/browser_diagnostics.html`
- [X] T010 [US1] Enforce delivery-quality threshold and final status labeling in `server/web/browser_diagnostics.html`

**Checkpoint**: Web TURN diagnostics produces valid transfer metrics and pass/fail decisions.

---

## Phase 4: User Story 2 - Measure TURN Transfer Metrics in Android Client (Priority: P2)

**Goal**: Implement equivalent two-client TURN transfer metric capture on Android and persist runs to backend reports.

**Independent Test**: Run Android TURN test and verify 10-second transfer metrics, threshold status, and successful report upload.

- [X] T011 [US2] Implement two-client TURN transfer execution loop in `android/app/src/main/java/selvakn/ipv6diag/diagnostic/StunTurnTest.kt`
- [X] T012 [US2] Integrate transfer metric capture into runner flow in `android/app/src/main/java/selvakn/ipv6diag/diagnostic/DiagnosticRunner.kt`
- [X] T013 [US2] Display transfer metrics in results UI in `android/app/src/main/java/selvakn/ipv6diag/ui/results/ResultsScreen.kt`
- [X] T014 [US2] Ensure TURN metric fields are included in backend-persisted reports via `android/app/src/main/java/selvakn/ipv6diag/upload/CloudUploader.kt`

**Checkpoint**: Android TURN diagnostics matches metric semantics with web and uploads report data.

---

## Phase 5: User Story 3 - Compare TURN Quality Across Runs (Priority: P3)

**Goal**: Enable reliable comparison across repeated runs from both web and Android clients.

**Independent Test**: Execute multiple runs and verify per-run metric history and backend report comparability.

- [X] T015 [US3] Persist and render web TURN run history entries with transfer metrics in `server/web/browser_diagnostics.html`
- [X] T016 [US3] Ensure backend report payload consistency checks for TURN metrics in `server/web/browser_diagnostics.html`
- [X] T017 [US3] Update Android run summaries for repeated-run comparison in `android/app/src/main/java/selvakn/ipv6diag/ui/results/ResultsScreen.kt`

**Checkpoint**: Both clients expose comparable run-to-run TURN quality data.

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Verification, docs, and task closure.

- [X] T018 [P] Add TURN transfer metric usage notes in `server/README.md`
- [X] T019 [P] Add Android/web validation notes in `specs/008-turn-transfer-metrics/quickstart.md`
- [X] T020 Run `GOCACHE=\"/home/selva/projects/Lenovo/Mesh/AndroidIPv6Diag/.gocache\" go test ./...` in `server/`
- [X] T021 Run `./gradlew testDebugUnitTest` in `android/`
- [X] T022 Mark completed tasks in `specs/008-turn-transfer-metrics/tasks.md`

---

## Dependencies & Execution Order

- Phase 1 → Phase 2 → Phases 3/4 → Phase 5 → Phase 6.
- US1 and US2 both depend on foundational data model updates.
- US3 depends on US1 and US2 metric semantics being finalized.

## Parallel Opportunities

- T018 and T019 can run in parallel with final validation.
- Within Phase 2, T004 and T006 can run after T003 without file conflicts.

## Parallel Example: User Story 2

```bash
Task: "Display transfer metrics in results UI in android/app/src/main/java/selvakn/ipv6diag/ui/results/ResultsScreen.kt"
Task: "Ensure TURN metric fields are included in backend-persisted reports via android/app/src/main/java/selvakn/ipv6diag/upload/CloudUploader.kt"
```

## Implementation Strategy

### MVP First (User Story 1 Only)
1. Complete setup + foundational model changes.
2. Implement web TURN transfer metrics (US1) and validate.

### Incremental Delivery
1. Add Android parity (US2) with backend persistence.
2. Finish run comparison and consistency improvements (US3).
3. Execute cross-cutting validation and documentation updates.
