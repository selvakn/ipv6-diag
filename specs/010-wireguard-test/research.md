# Research: WireGuard Protocol Diagnostic Test

**Feature**: `010-wireguard-test` | **Phase**: 0 — Research | **Date**: 2026-07-05

---

## Decision 1: WireGuard Go Library

**Decision**: Use `golang.zx2c4.com/wireguard` (wireguard-go) as the sole WireGuard library.

**Rationale**:
- Pure Go — no CGO, cross-compiles cleanly to Linux, macOS, Windows, and Android.
- Maintained by the WireGuard project. Used in production (Tailscale, Mullvad, etc.).
- Provides both the full WireGuard protocol stack AND an in-process TUN via the `tun/netstack` subpackage.
- The `tun/netstack` package embeds gVisor's TCP/IP stack — creates a virtual network interface inside the process without kernel TUN, CAP_NET_ADMIN, or root.
- All existing Go modules already use pion libraries (also pure Go); wireguard-go fits the same pattern.

**Alternatives considered**:
- `wireguard-tools` (kernel module CLI) — requires kernel WireGuard, root, and `CAP_NET_ADMIN`. Ruled out.
- CGO bindings to WireGuard C library — introduces CGO, breaking cross-compilation for Android without NDK setup.

---

## Decision 2: In-Process TUN via netstack (No Root Anywhere)

**Decision**: Use `golang.zx2c4.com/wireguard/tun/netstack` for both server and CLI WireGuard devices.

**Rationale**:
- `tun.CreateNetTUN(localAddresses, dns, mtu)` creates an in-memory virtual network device backed by gVisor's netstack. Returns a `*netstack.Net` implementing `DialContext`, `ListenPacket`, `Listen` — standard Go net interfaces.
- The WireGuard device (`device.NewDevice`) wraps this TUN and handles all WireGuard protocol operations (handshake, encryption, keepalives).
- The resulting `*netstack.Net` exposes the tunnel as an ordinary Go network — you can bind a UDP/TCP echo server on the tunnel IP using standard library calls.
- No `CAP_NET_ADMIN`, no `/dev/net/tun`, no kernel modules required.
- The Docker server container currently has no special Linux capabilities — netstack preserves this.
- Confirmed working on Android: gVisor's netstack is compiled purely in Go with architecture-specific Go assembly (not CGO), which the Android NDK Go toolchain supports.

**Alternatives considered**:
- Kernel WireGuard (`wg` + TUN fd): Requires root and CAP_NET_ADMIN — ruled out for server and impossible without VPN Service on Android.
- `tun.CreateTUN()` (OS TUN device): Requires root on Linux and VPN Service on Android — ruled out.

---

## Decision 3: Credential Model — Server Generates Full Client Config

**Decision**: The WireGuard credential endpoint generates a complete client WireGuard config server-side and returns it as JSON. The client private key is included in the response.

**Rationale**:
- Mirrors the TURN credential pattern (GET endpoint, Bearer token, JSON response with all fields needed to connect).
- Eliminates the need for a client-to-server public key exchange step, keeping the API a simple idempotent GET — consistent with the rest of the diagnostic API.
- The credentials are transmitted over HTTPS (same as TURN passwords), so sending a private key in the response body is an acceptable and common pattern (Tailscale, Netbird, etc. use variations of this).
- Server generates a fresh Curve25519 keypair per session. Client uses the provided private key. Session expires with the TTL and the server's peer config is cleaned up.

**Credential response shape** (mirrors `turnCredentialsResponse`):
```json
{
  "client_private_key": "<base64-encoded 32-byte Curve25519 key>",
  "client_ip":          "10.0.0.2/24",
  "server_public_key":  "<base64-encoded 32-byte Curve25519 key>",
  "server_endpoint":    "1.2.3.4:51820",
  "ttl_seconds":        120,
  "expires_at":         "2026-07-05T17:00:00Z"
}
```

**Alternatives considered**:
- Client-generated keypair, POST request: More cryptographically correct but breaks the TURN GET pattern and requires request body parsing.
- Pre-shared keys: Simpler but static — cannot expire per session.

---

## Decision 4: Test Transfer Model — Echo Server on Tunnel IP

**Decision**: The server runs a UDP echo service on its WireGuard tunnel IP (`10.0.0.1:7000`). The CLI creates two independent in-process WireGuard peers (A and B), each with its own credential set. Both peers concurrently send data to the server echo service and measure throughput and RTT. Total transfer target: 1 MB outbound (combined A+B), 1 MB echoed back.

**Rationale**:
- Mirrors the TURN test methodology (two data paths, throughput + RTT measured).
- UDP echo is the simplest reliable echo protocol over a WireGuard tunnel — no TCP handshake overhead.
- Two credential sets → two independent WireGuard sessions → tests that the server handles concurrent sessions.
- The server does NOT need to route traffic between PeerA and PeerB (no inter-peer routing). Each peer communicates independently with the server echo service. This avoids complex netstack routing table configuration.
- Echo latency + throughput on two parallel tunnels provides a meaningful "WireGuard reachability" signal analogous to the two-PeerConnection TURN test.

**Alternatives considered**:
- Peer-to-peer through server relay (A → B through server routing): Requires server-side netstack routing between two tunnel IPs. More complex and fragile — deferred to a future improvement.
- Single peer, single credential: Simpler but doesn't test concurrent session capacity at all.

---

## Decision 5: Android Binding via gomobile bind

**Decision**: Use `golang.org/x/mobile/cmd/gomobile bind` to generate an Android Archive (`.aar`) from `wgmodule/`.

**Rationale**:
- `gomobile bind` generates Java/Kotlin-callable wrappers for exported Go types without manual JNI boilerplate.
- Supports callback interfaces: a Go interface implemented in Kotlin is passed across the JNI boundary — perfect for the agreed async callback pattern.
- wireguard-go is pure Go, so `gomobile bind` (which uses `GOARCH=arm64 CGO_ENABLED=1`) works without additional C dependencies.
- The generated `.aar` is dropped into `android/app/libs/`, declared as a local `fileTree` dependency in `build.gradle.kts`, and used directly from Kotlin.
- gomobile supports `android/arm64` and `android/amd64` (for emulator) — matches SC-006.

**gomobile callback interface**:
```go
// Defined in wgmodule/, exported for gomobile
type WireGuardCallback interface {
    OnResult(result *WireGuardResult, errMsg string)
}

func RunWireGuardTestAsync(credsJSON string, callback WireGuardCallback) {
    go func() {
        result, err := runWireGuardTest(credsJSON)
        errMsg := ""
        if err != nil { errMsg = err.Error() }
        callback.OnResult(result, errMsg)
    }()
}
```

**Alternatives considered**:
- Manual JNI (CGO + JNI header): Full control but significant boilerplate and error-prone type marshaling.
- Exposed as a CLI binary called from Android via `Runtime.exec()`: Not suitable for an in-process library call.

---

## Decision 6: Monorepo Module Layout

**Decision**: New module at `wgmodule/` with `module github.com/selvakn/ipv6diag-wg`, referenced via `replace` directive in `cli/go.mod`. The `android/wgmodule-build/build.sh` script runs `gomobile bind` pointing at `wgmodule/`.

**File layout**:
```
wgmodule/
├── go.mod       # module github.com/selvakn/ipv6diag-wg; go 1.25.0
├── peer.go      # WireGuardPeer: setup, send/receive, teardown (via netstack)
├── creds.go     # WireGuardCredential type, JSON parsing, base64 key decode
├── transfer.go  # timed transfer window, throughput + RTT measurement
└── callback.go  # //go:build android — WireGuardCallback + RunWireGuardTestAsync
```

`cli/go.mod` addition:
```
require github.com/selvakn/ipv6diag-wg v0.0.0
replace github.com/selvakn/ipv6diag-wg => ../wgmodule
```

**Rationale**: Consistent with the existing CLI module pattern. No external publishing needed for v1.

---

## Decision 7: Server WireGuard Config / Wiring

**Decision**: WireGuard service is opt-in via environment variables (`WG_ENABLED=true`, `WG_PORT=51820`, `WG_SUBNET=10.0.0.0/24`). When disabled, the credential endpoint returns 503 with "wireguard service unavailable" — consistent with TURN's disabled behavior.

**Server internal package**: `server/internal/wireguard/` — mirrors `server/internal/turn/` structure:
- `config.go`: `WireGuardConfig` with Enable flag, port, subnet, max sessions, TTL
- `credentials.go`: `SessionManager` — issues sessions (keypairs + IP allocation), prunes expired
- `service.go`: `Service` — wraps wireguard-go device + netstack, starts UDP echo server on tunnel IP

**Wiring in main.go**: Same pattern as TURN — check `WG_ENABLED`, construct `wgsvc.Service`, register `wgsvc.CredentialHandler` at `/wireguard/credentials`.

**Observability**: `zap.Logger` (already in server) used for structured log events. Active session count exposed via `/health` JSON (existing health endpoint extended with `wg_active_sessions` field).
