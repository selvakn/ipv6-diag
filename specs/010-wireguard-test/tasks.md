# Tasks: WireGuard Protocol Diagnostic Test

**Input**: Design documents from `/specs/010-wireguard-test/`
**Prerequisites**: plan.md ✓, spec.md ✓, research.md ✓, data-model.md ✓, contracts/ ✓

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: New `wgmodule/` Go module and server package scaffolding

- [X] T001 Create `wgmodule/` directory with `go.mod` (`module github.com/selvakn/ipv6diag-wg; go 1.25.0`) and add `golang.zx2c4.com/wireguard` dependency in `wgmodule/go.mod`
- [X] T002 [P] Create `server/internal/wireguard/` package directory (empty, scaffolded for Phase 2)
- [X] T003 [P] Create `android/wgmodule-build/` directory with `README.md` explaining `build.sh` prerequisites (Go, gomobile)

---

## Phase 2: Foundational — Server WireGuard Service (User Story 3, P1)

**Purpose**: Server credential issuance and WireGuard echo peer — gates all client tests

**⚠️ CRITICAL**: CLI and Android work cannot be meaningfully tested until this phase is complete

**Goal**: GET `/wireguard/credentials` returns valid WireGuard session config; server WireGuard peer echoes UDP datagrams on tunnel IP port 7000.

**Independent Test**: `curl -H "Authorization: Bearer $TOKEN" http://localhost:8080/wireguard/credentials` returns JSON with `client_private_key`, `client_ip`, `server_public_key`, `server_endpoint`, `ttl_seconds`, `expires_at`. A `wg` client (or test harness) can connect and exchange UDP with the echo service.

- [X] T004 Implement `server/internal/wireguard/config.go` — `WireGuardConfig` struct with fields: `Enabled bool`, `Port int` (default 51820), `Subnet string` (default `"10.0.0.0/24"`), `MaxSessions int` (default 50), `SessionTTL time.Duration` (default 2m), `EchoPort int` (default 7000), `PublicEndpoint string`; add `HasValidConfig() bool` helper
- [X] T005 Implement `server/internal/wireguard/credentials.go` — `SessionManager` struct with `Issue(clientID string) (SessionLease, error)` (generate Curve25519 client keypair, allocate next free `/32` IP from pool, store session), `ActiveCount() int`, `pruneLocked(now time.Time)`, `PeerConfigs() []PeerConfig` (returns current valid peers for device sync); `SessionLease` type with `ClientPrivateKey [32]byte`, `ClientPublicKey [32]byte`, `ClientIP net.IP`, `ExpiresAt time.Time`, `SessionID string`
- [X] T006 Implement `server/internal/wireguard/service.go` — `Service` struct: generates server Curve25519 keypair at startup; creates `device.Device` via `tun.CreateNetTUN([]netip.Addr{serverTunnelIP}, nil, 1420)`; configures device with server private key and initial peer list via `device.IpcSet(uapiConfig)`; starts UDP echo goroutine listening on `netstack.Net.ListenPacket("udp", serverTunnelIP:echoPort)` echoing datagrams; starts background session pruner (ticker 60s); exposes `Start() error`, `Stop()`, `ServerPublicKey() [32]byte`, `PublicEndpoint(host string) string`
- [X] T007 Implement `server/internal/handler/wireguard_credentials.go` — `WireGuardCredentialsHandler` struct (fields: `Token string`, `Sessions *wireguard.SessionManager`, `Service *wireguard.Service`); `ServeHTTP` validates Bearer token, checks `Sessions != nil && Service != nil` (503 if not), calls `Sessions.Issue(clientIP)`, syncs new peer into device via `Service.AddPeer(clientPubKey, clientIP)`, returns JSON matching the contract schema; 503 on capacity exceeded; structured zap log on each session create/fail
- [X] T008 Wire WireGuard service in `server/cmd/server/main.go` — read `WG_ENABLED`, `WG_PORT`, `WG_SUBNET`, `WG_MAX_SESSIONS`, `WG_SESSION_TTL`, `WG_PUBLIC_ENDPOINT` env vars; construct `wgsvc.Service` and `wgsvc.SessionManager`; call `wgsvc.Start()`; register handler at `httpMux.Handle("/wireguard/credentials", &handler.WireGuardCredentialsHandler{...})`; add `wg_active_sessions` field to existing health handler JSON response
- [X] T009 Add `golang.zx2c4.com/wireguard` and `golang.zx2c4.com/wireguard/tun/netstack` to `server/go.mod` via `go get`

**Checkpoint**: `WG_ENABLED=true go run ./server/cmd/server` starts without error; credential endpoint returns valid JSON; a manual wireguard-go client can connect and echo UDP.

---

## Phase 3: User Story 1 — CLI WireGuard Connectivity Test (P1) 🎯 MVP

**Goal**: `ipv6diag` runs WireGuard test and reports `PASS`/`FAIL`/`SKIPPED` with throughput and latency.

**Independent Test**: `./ipv6diag --server http://localhost:8080 --tests wireguard` prints a WireGuard result row. Against a WireGuard-enabled server it shows `PASS` with `kbps` and `ms` values; against a server without WireGuard it shows `SKIPPED`.

### Implementation for User Story 1

- [X] T010 Implement `wgmodule/creds.go` — `WireGuardCredential` struct (`ClientPrivateKey [32]byte`, `ClientPublicKey [32]byte` derived, `ClientIP *net.IPNet`, `ServerPublicKey [32]byte`, `ServerEndpoint *net.UDPAddr`); `ParseCredential(jsonBytes []byte) (*WireGuardCredential, error)` decoding base64 keys and CIDR/addr; `FetchCredential(serverURL, token string, transport *http.Transport) (*WireGuardCredential, error)` calling `GET /wireguard/credentials`
- [X] T011 Implement `wgmodule/peer.go` — `Peer` struct wrapping a `device.Device` + `*netstack.Net` + `net.PacketConn` (bound to echo port on tunnel IP); `NewPeer(cred *WireGuardCredential) (*Peer, error)` — creates netstack TUN via `tun.CreateNetTUN`, creates `device.Device`, applies UAPI config (private key, peer public key, allowed IPs 0.0.0.0/0, endpoint), calls `device.Up()`; `WaitHandshake(ctx context.Context) error` — polls device peer stats until `LastHandshakeTime` non-zero or ctx expires; `EchoConn() net.PacketConn` — listens on netstack for UDP; `Close()`
- [X] T012 Implement `wgmodule/transfer.go` — `TransferResult` struct (`BytesSent, BytesReceived int64`, `AvgRTTMs float64`, `RateKbps float64`, `Elapsed time.Duration`); `RunEchoTransfer(ctx context.Context, conn net.PacketConn, serverAddr *net.UDPAddr, windowSec int, payloadBytes int) TransferResult` — timed loop sending UDP datagrams to serverAddr:7000, receiving echoes, measuring RTT per round-trip, accumulating bytes
- [X] T013 Add `require github.com/selvakn/ipv6diag-wg v0.0.0` and `replace github.com/selvakn/ipv6diag-wg => ../wgmodule` to `cli/go.mod`; run `go mod tidy` in `cli/`
- [X] T014 Add `TestWireGuard TestType = "wireguard"` constant to `cli/diag/types.go`; add `WireGuardCredentials` struct mirroring the JSON response; add `TestWireGuard` to `AllTests` slice; add `WireGuardEnabled bool`, `WireGuardEchoPort int` fields to `ServerConfig`
- [X] T015 Add `FetchWireGuardCredentials(serverURL, token string, transport *http.Transport) (*WireGuardCredentials, error)` to `cli/diag/config_fetch.go` — GET `/wireguard/credentials`, returns nil + skipped reason on 404/503
- [X] T016 Implement `cli/diag/wireguard.go` — `RunWireGuard(cfg *ServerConfig, stack string, timeout time.Duration, spinner *output.Spinner) TestResult`: fetch two credential sets (A and B) in parallel; create two `wgmodule.Peer` instances in parallel; wait handshake for both (5s timeout); run `RunEchoTransfer` concurrently for A and B (each 512 KB target, 10s window); aggregate metrics into `TestResult{TestType: TestWireGuard, ...}` with throughput and RTT; return `StatusSkipped` on nil creds (503/404 from server); return `StatusFailed` on handshake timeout

**Checkpoint**: `go build ./...` in `cli/` succeeds; `go test ./...` in `wgmodule/` and `cli/` pass; CLI produces WireGuard result row in both text and JSON output modes.

---

## Phase 4: User Story 2 — Android WireGuard Diagnostic (P2)

**Goal**: Android app runs WireGuard test via gomobile-compiled native library; result appears in results list.

**Independent Test**: Build and install Android debug APK; run diagnostics against WireGuard-enabled server; verify `WIREGUARD` row appears in results screen with status, latency, and throughput values.

### Implementation for User Story 2

- [X] T017 [P] Add `WIREGUARD` to `TestType` enum in `android/app/src/main/java/selvakn/ipv6diag/data/model/TestResult.kt`
- [X] T018 [P] Implement `wgmodule/callback.go` (build tag `//go:build android`) — export `WireGuardResult` struct (`Status, FailureReason, AvgRTTMs, RateKbps, BytesSent, BytesReceived string`; all string for gomobile compatibility); export `WireGuardCallback` interface with `OnResult(result *WireGuardResult, errMsg string)`; export `RunWireGuardTestAsync(serverURL, token, stack string, callback WireGuardCallback)` — goroutine that fetches one credential, creates one peer, runs echo transfer, calls `callback.OnResult`
- [X] T019 Create `android/wgmodule-build/build.sh` — shell script running `gomobile bind -target android -androidapi 26 -o $(git rev-parse --show-toplevel)/android/app/libs/wglib.aar github.com/selvakn/ipv6diag-wg`; create `android/wgmodule-build/README.md` with setup instructions (install Go 1.25+, run `go install golang.org/x/mobile/cmd/gomobile@latest && gomobile init`)
- [X] T020 Run `android/wgmodule-build/build.sh` to generate `android/app/libs/wglib.aar`
- [X] T021 Add `fileTree(dir = "libs", include = ["*.aar"])` dependency to `android/app/build.gradle.kts` dependencies block; run `./gradlew assembleDebug` to verify AAR resolves
- [X] T022 Implement `android/app/src/main/java/selvakn/ipv6diag/diagnostic/WireGuardTest.kt` — `suspend fun runWireGuardTest(network: Network, sessionId: String, serverURL: String, token: String, addressFamily: AddressFamily): TestResult` using `withContext(Dispatchers.IO)`: call `Wgmodule.runWireGuardTestAsync()` via a `CompletableFuture`-backed adapter (since gomobile callback is not a coroutine); parse `WireGuardResult` into `TestResult(testType = TestType.WIREGUARD, ...)`; return `SKIPPED` if credential endpoint returns non-200
- [X] T023 Add `TestType.WIREGUARD` to `baseTypes` list in `DiagnosticRunner.kt` for `TestFilter.ALL`; add corresponding `when` branch calling `runWireGuardTest(network, sessionId, endpoint.url, endpoint.token, AddressFamily.IPv4)` and `IPv6` variants (same pattern as STUN/TURN); add `SKIPPED` fallback when result list is empty

**Checkpoint**: `./gradlew assembleDebug` succeeds; install on emulator/device; WIREGUARD row appears in results UI.

---

## Phase 5: Polish & Cross-Cutting Concerns

**Purpose**: Integration consistency, docker-compose, CI wiring

- [X] T024 [P] Add `WG_ENABLED`, `WG_PORT`, `WG_SUBNET`, `WG_MAX_SESSIONS`, `WG_SESSION_TTL`, `WG_PUBLIC_ENDPOINT` environment variable entries to `docker-compose.yml` under the `server` service (commented-out defaults, with explanatory comment mirroring the TURN env block)
- [X] T025 [P] Update `server/internal/handler/browser_diagnostics.go` config response to include `wireguard_enabled bool` field so CLI can detect WireGuard support from the config endpoint without making a separate credential request
- [X] T026 [P] Add `cli/diag/config_fetch.go` to parse `wireguard_enabled` from server config and skip credential fetch when false (returning SKIPPED result immediately)
- [X] T027 [P] Update `cli/diag/types.go` output formatting — add WireGuard row to columnar text output in `output/text.go` (reuse TURN row format: test name, status, RTT, throughput)
- [X] T028 [P] Update `android/app/src/main/java/selvakn/ipv6diag/ui/results/ResultsScreen.kt` to display WIREGUARD results — add row rendering for `TestType.WIREGUARD` (reuse TURN result card, showing throughput + RTT)
- [X] T029 Verify `go build -o /dev/null ./...` succeeds in `wgmodule/`, `cli/`, and `server/` modules; fix any import errors
- [X] T030 [P] Add `.github/workflows/` step (or note in existing CI) to rebuild `wglib.aar` when `wgmodule/` changes (document as a manual step if full CI integration is out of scope for v1)

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies — start immediately
- **Foundational (Phase 2)**: Depends on Phase 1 — T001 must complete before T004-T009
- **User Story 1 / CLI (Phase 3)**: Depends on Phase 2 complete (server credential endpoint needed for integration) — T010-T012 (wgmodule) can start in parallel with Phase 2; T013-T016 (CLI wiring) depend on Phase 2
- **User Story 2 / Android (Phase 4)**: Depends on Phase 2 (server) and T010-T012 (wgmodule core); T017-T018 can run in parallel with Phase 3
- **Polish (Phase 5)**: Depends on Phases 2-4 complete

### User Story Dependencies

- **US3 / Server (Phase 2)**: Foundational — gates both client stories
- **US1 / CLI (Phase 3)**: Depends on Phase 2 complete; wgmodule tasks T010-T012 can run concurrently
- **US2 / Android (Phase 4)**: Depends on wgmodule tasks T010-T012 and Phase 2; T017-T018 can start in parallel with Phase 3

### Parallel Opportunities

- T002 and T003 (Phase 1) can run in parallel with T001
- T010, T011, T012 (wgmodule core) can all run in parallel
- T017 (Android enum) and T018 (gomobile callback) can run in parallel once wgmodule exists
- T024-T028 (Phase 5) are all independently parallelizable

---

## Implementation Strategy

### MVP First (US3 + US1)

1. Complete Phase 1 (Setup)
2. Complete Phase 2 (Server — credential endpoint + echo service)
3. Complete Phase 3 (CLI — two in-process peers, transfer test)
4. **STOP and VALIDATE**: `ipv6diag --server http://localhost:8080` shows WIREGUARD row
5. Android (Phase 4) follows as second increment

### Incremental Delivery

1. Phase 1 → Phase 2 → Server credential endpoint works (`curl` test)
2. Phase 3 → CLI WireGuard test works end-to-end → MVP demonstrable
3. Phase 4 → Android WireGuard test works on device
4. Phase 5 → Docker, CI, UI polish

---

## Notes

- `wgmodule/` Go version must match `go.mod` minimum (1.25.0) for gomobile compatibility
- `wglib.aar` must be committed to git so Android builds work without Go toolchain
- WireGuard device UAPI config format: `private_key=<hex>\npublic_key=<hex>\nallowed_ip=<cidr>\nendpoint=<host:port>\n\n` (double newline terminates peer block)
- Curve25519 keys: WireGuard uses 32-byte keys; base64-standard encoding (not URL-safe) for JSON transport
- netstack `CreateNetTUN` signature: `func CreateNetTUN(localAddresses []netip.Addr, dnsServers []netip.Addr, mtu int) (tun.Device, *Net, error)`
- gomobile does not support Go channels, maps, or slices of interfaces — keep the exported API surface simple (use structs with string fields for `WireGuardResult`)
