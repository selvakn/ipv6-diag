# Tasks: Embedded TURN Relay Service

**Input**: Design documents from `/specs/006-turn-dualstack-vps/`  
**Prerequisites**: `plan.md`, `spec.md`, `research.md`, `data-model.md`, `contracts/`

**Tests**: Include focused Go tests for TURN credential lifecycle and endpoint behavior.  
**Organization**: Tasks are grouped by user story to keep delivery independently testable.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no direct dependencies)
- **[Story]**: Maps to user stories in `spec.md` (`US1`, `US2`, `US3`)

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Add dependencies and shared configuration scaffolding.

- [X] T001 Add TURN dependency in `server/go.mod` via `go get github.com/pion/turn/v4`
- [X] T002 Create TURN package skeleton in `server/internal/turn/config.go`, `server/internal/turn/credentials.go`, and `server/internal/turn/service.go`
- [X] T003 Add TURN env var documentation placeholders in `server/README.md`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core components required by all user stories.

- [X] T004 Implement TURN configuration parsing and defaults in `server/internal/turn/config.go`
- [X] T005 Implement in-memory 5-minute credential leasing logic in `server/internal/turn/credentials.go`
- [X] T006 [P] Implement credential endpoint contract types and validation in `server/internal/handler/turn_credentials.go`
- [X] T007 Wire TURN config and credential manager initialization in `server/cmd/server/main.go`

**Checkpoint**: Foundation ready for story work.

---

## Phase 3: User Story 1 - Run TURN relay from one server package (Priority: P1) 🎯 MVP

**Goal**: Run diagnostics and TURN from a single server process with working credential issuance.

**Independent Test**: Start server with TURN enabled and verify `/health`, `/diag`, and `/turn/credentials` in one process.

### Tests for User Story 1

- [X] T008 [P] [US1] Add credential manager unit tests in `server/internal/turn/credentials_test.go`
- [X] T009 [P] [US1] Add credential endpoint handler tests in `server/internal/handler/turn_credentials_test.go`

### Implementation for User Story 1

- [X] T010 [US1] Implement TURN credential HTTP handler in `server/internal/handler/turn_credentials.go`
- [X] T011 [US1] Register `/turn/credentials` route on HTTP and HTTPS muxes in `server/cmd/server/main.go`
- [X] T012 [US1] Implement embedded TURN service startup/stop lifecycle in `server/internal/turn/service.go`
- [X] T013 [US1] Start TURN service from server bootstrap and graceful shutdown path in `server/cmd/server/main.go`

**Checkpoint**: Single-process diagnostics + TURN + credential endpoint works.

---

## Phase 4: User Story 2 - Support IPv4/IPv6 and both transports (Priority: P2)

**Goal**: Support UDP/TCP listeners on IPv4/IPv6 with degraded behavior and IPv6-first preference.

**Independent Test**: Enable all four listener modes and confirm state logs reflect active/degraded per listener.

### Tests for User Story 2

- [X] T014 [P] [US2] Add TURN listener status tests in `server/internal/turn/service_test.go`

### Implementation for User Story 2

- [X] T015 [US2] Implement multi-listener startup tracking (`udp4`, `udp6`, `tcp4`, `tcp6`) in `server/internal/turn/service.go`
- [X] T016 [US2] Implement degraded startup behavior and status reporting in `server/internal/turn/service.go`
- [X] T017 [US2] Add startup/allocation logging with protocol and IP family in `server/internal/turn/service.go`
- [X] T018 [US2] Apply IPv6-first with IPv4 fallback URI ordering in `server/internal/handler/turn_credentials.go`

**Checkpoint**: Dual-stack + dual-transport behavior is available and observable.

---

## Phase 5: User Story 3 - Deploy reliably outside Fly.io (Priority: P3)

**Goal**: Ensure Docker image and docs are VPS-ready for TURN + diagnostics.

**Independent Test**: Build image and verify required TURN/DX ports and env config are documented and exposed.

### Implementation for User Story 3

- [X] T019 [US3] Update container port exposure and health assumptions in `server/Dockerfile`
- [X] T020 [US3] Document VPS deployment env vars and port mappings in `server/README.md`
- [X] T021 [US3] Update runtime examples for TURN-enabled launch in `specs/006-turn-dualstack-vps/quickstart.md`

**Checkpoint**: VPS deployment path is documented and aligned to runtime behavior.

---

## Phase 6: Polish & Cross-Cutting Concerns

- [X] T022 Run `go test ./...` in `server/` and fix failures
- [X] T023 Run `go test ./...` from repo root for final validation (module-scoped final run executed in `server/`)
- [X] T024 Ensure task completion markers and final notes are updated in `specs/006-turn-dualstack-vps/tasks.md`

---

## Dependencies & Execution Order

- Phase 1 → Phase 2 → Phase 3 → Phase 4 → Phase 5 → Phase 6
- `T008`/`T009` precede `T010`–`T013`
- `T014` precedes `T015`–`T018`
- `T022` and `T023` must pass before completion

## Parallel Opportunities

- Phase 2: `T006` can run parallel to `T004`/`T005` after skeleton exists
- Phase 3: `T008` and `T009` parallel
- Phase 4: `T014` can run before and independently from `T015` implementation details

## Implementation Strategy

### MVP First (US1)
1. Complete setup and foundational phases
2. Deliver US1 with tests and integrated lifecycle
3. Validate one-process runtime before dual-stack expansion

### Incremental Delivery
1. Add dual-stack/transport resiliency (US2)
2. Finalize VPS deployability and docs (US3)
3. Execute full validation and close task list
