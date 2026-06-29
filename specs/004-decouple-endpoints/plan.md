# Implementation Plan: Decouple Test and Reporting Endpoints

**Branch**: `004-decouple-endpoints` | **Date**: 2026-06-29 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/004-decouple-endpoints/spec.md`

## Summary

Keep report upload fixed to `androidipv6diag.fly.dev` while allowing runtime diagnostic probes (`ICMP`, `HTTP`, `HTTPS`, `DNS`, and 464XLAT helper checks) to use a configurable test endpoint. Persist and upload test-endpoint context with each session so dashboard consumers can interpret client-IP observations correctly.

## Technical Context

**Language/Version**: Go 1.24 (server), Kotlin/Android API 26+ (app)  
**Primary Dependencies**: Existing Go stdlib + `modernc.org/sqlite`; Android OkHttp3 + coroutines + Room  
**Storage**: Android Room database for local sessions/settings; server SQLite for uploaded reports  
**Testing**: `go test ./...` (server), `mise exec -- ./gradlew assembleDebug` (Android build verification)  
**Target Platform**: Linux Docker container (server); Android API 26+ devices  
**Project Type**: Existing web service + mobile app extension  
**Performance Goals**: No regression from current test runtime; report upload behavior unchanged; dashboard detail remains responsive (<3s list/detail load)  
**Constraints**: Reporting host must remain fixed to `androidipv6diag.fly.dev`; test endpoint input must be host or host:port only; no silent fallback of failed probes to another endpoint  
**Scale/Scope**: Internal diagnostics usage with current report volume (<500 list limit); backward compatibility for existing reports and existing app users

## Constitution Check

Constitution file is still a placeholder template with no enforceable gates. No violations before or after design.

## Project Structure

### Documentation (this feature)

```text
specs/004-decouple-endpoints/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   └── endpoint-decoupling-api.md
└── tasks.md
```

### Source Code (repository root)

```text
android/app/src/main/java/selvakn/ipv6diag/
├── ui/
│   ├── home/HomeScreen.kt                 ← MODIFIED: upload always uses fixed reporting endpoint
│   └── settings/SettingsScreen.kt         ← MODIFIED: test endpoint validation and save rules
├── upload/CloudUploader.kt                ← MODIFIED: include test endpoint context in upload payload
├── data/
│   ├── model/DiagnosticSession.kt         ← MODIFIED: persist test endpoint used for run
│   ├── db/Entities.kt                     ← MODIFIED: Room entity column for test endpoint
│   └── repository/SessionRepository.kt    ← MODIFIED: read/write test endpoint context
└── IPv6DiagApplication.kt                 ← MODIFIED: fixed reporting base URL helper

server/internal/store/
├── db.go                                  ← MODIFIED: migration add test_endpoint column
└── report_store.go                        ← MODIFIED: store and return test endpoint context

specs/004-decouple-endpoints/*             ← NEW/MODIFIED: design and task artifacts
CLAUDE.md                                  ← MODIFIED: point SPECKIT context to this plan
```

**Structure Decision**: Extend existing Android + server code paths in place; no new modules required.

## Complexity Tracking

No constitution violations to justify.
