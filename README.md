# ipv6-diag

A set of diagnostic tools for testing IPv6 connectivity and network path quality. Run tests from a browser, an Android device, or a command-line binary.

A hosted server is available at **https://ipv6-diag.selvakn.in**.

---

## What it tests

Each client runs the same five diagnostic tests against a target server:

| Test | What it checks |
|------|---------------|
| HTTP | Plain HTTP reachability to the target |
| HTTPS | TLS reachability and certificate validity |
| ICMP-equiv | Basic host reachability via HTTP HEAD |
| STUN | UDP hole-punching and external address discovery via the embedded STUN server |
| TURN | Relay throughput and round-trip latency via the embedded TURN server, using a full two-peer data channel transfer |

Tests run over IPv4, IPv6, or both stacks, depending on the client and flags used. The TURN test is the most informative: it measures actual relay throughput (kbps), packet delivery ratio, and round-trip latency by transferring data between two peer connections through the TURN relay.

---

## Clients

### Web

Open **https://ipv6-diag.selvakn.in** in any browser. The page detects your IPv4 and IPv6 addresses using split-DNS probes, runs all five tests, and displays results inline.

### Android

Download the latest APK from the [Releases](https://github.com/selvakn/ipv6-diag/releases) page.

The Android client runs the same test suite and reports the device model, OS version, and network interface details alongside the test results, which makes it useful for comparing cellular vs. Wi-Fi connectivity on the same device.

Minimum Android version: depends on the release APK (check the release notes).

### CLI

Download a pre-built binary for your platform from the [Releases](https://github.com/selvakn/ipv6-diag/releases) page. The binary is statically linked with no runtime dependencies.

Available platforms:

- `ipv6diag-linux-amd64`
- `ipv6diag-linux-arm64`
- `ipv6diag-darwin-amd64`
- `ipv6diag-darwin-arm64`
- `ipv6diag-windows-amd64.exe`
- `ipv6diag-windows-arm64.exe`

**Run all tests against the hosted server:**

```
./ipv6diag
```

**Force IPv4 or IPv6 only:**

```
./ipv6diag --ipv4
./ipv6diag --ipv6
```

**Run a specific subset of tests:**

```
./ipv6diag --tests http,https,stun
```

**Test against a custom server:**

```
./ipv6diag --server https://your-server.example.com
```

**Upload results:**

```
./ipv6diag --upload
```

**Machine-readable JSON output (useful in CI):**

```
./ipv6diag --json
```

Exit code is `0` if all tests pass, `1` if any test fails or times out, `2` for invalid flags.

**Full flag reference:**

| Flag | Default | Description |
|------|---------|-------------|
| `--ipv4` | off | Force IPv4 stack only |
| `--ipv6` | off | Force IPv6 stack only |
| `--both` | on | Run both stacks (default) |
| `--server` | `https://ipv6-diag.selvakn.in` | Target server base URL |
| `--tests` | `http,https,icmp,stun,turn` | Comma-separated test subset |
| `--timeout` | `15000` | Per-test timeout in milliseconds |
| `--turn-token` | — | Bearer token for TURN credentials (or `TURN_TOKEN` env var) |
| `--upload` | off | POST results to the server |
| `--insecure` | off | Skip TLS verification (for local/staging servers) |
| `--json` | off | Emit results as JSON instead of human-readable text |
| `--version` | — | Print version and exit |

---

## Self-hosting

The server is a single Go binary. It embeds a STUN/TURN relay, serves the web client, issues TURN credentials, stores diagnostic reports in a local SQLite database, and manages TLS certificates automatically via Let's Encrypt.

**Environment variables:**

| Variable | Description |
|----------|-------------|
| `HTTPS_HOST` | Comma-separated hostnames for TLS certificate provisioning (e.g. `ipv6-diag.example.com,4.ipv6-diag.example.com,6.ipv6-diag.example.com`) |
| `HTTP_PORT` | HTTP listen port (default `80`) |
| `HTTPS_PORT` | HTTPS listen port (default `443`) |
| `TURN_PORT` | TURN/STUN UDP listen port (default `3478`) |
| `TURN_SECRET` | Shared secret for TURN credential HMAC |
| `TURN_REALM` | TURN realm (default: derived from hostname) |
| `TURN_PUBLIC_IP` | Public IP to advertise in TURN relay candidates |
| `DB_PATH` | SQLite database file path (default `reports.db`) |

The server binds a TURN listener on `TURN_PORT` and serves TURN credentials at `/turn/credentials`. The web client fetches these credentials directly and uses them for the TURN test, so no separate TURN configuration is needed on the client side.

For split-DNS IPv4/IPv6 detection, add DNS records that resolve to only A or only AAAA:

```
4.ipv6-diag.example.com  →  A record only  (forces IPv4 connections)
6.ipv6-diag.example.com  →  AAAA record only  (forces IPv6 connections)
ipv6-diag.example.com    →  both A and AAAA
```

Then set `HTTPS_HOST=ipv6-diag.example.com,4.ipv6-diag.example.com,6.ipv6-diag.example.com` to obtain TLS certificates for all three names.

**Build from source:**

```
cd server
go build ./cmd/server/
./server
```

Requires Go 1.25 or later.

---

## Repository layout

```
android/          Android client (Kotlin)
server/           Go server (HTTP, STUN/TURN, SQLite, TLS)
  cmd/server/     Server entry point
  internal/       Handlers, TURN service, credential manager
  web/            Web client (HTML + vanilla JS, served by the server)
cli/              Go CLI client
  main.go         Entry point and flag definitions
  diag/           HTTP, STUN, and TURN test implementations
  output/         Human-readable and JSON output
  upload/         Result upload
.github/workflows/
  release-apk.yml    Builds and publishes the Android APK on version tags
  release-cli.yml    Builds and publishes CLI binaries on version tags
specs/            Feature specifications and implementation plans
```
