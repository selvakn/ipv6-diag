# Android IPv6 Diag Server

## Browser diagnostics page

The server hosts a browser-run diagnostics page at `/browser-diagnostics` and a public config endpoint at `/browser-diagnostics/config`.

All tests execute from the browser client context. This release is intentionally public and does not enforce request throttling.

### Browser diagnostics environment variables

- `BROWSER_DIAG_ALLOW_CUSTOM_TARGETS` (`true`/`false`, default `true`)
- `BROWSER_DIAG_PER_TEST_TIMEOUT_MS` (default `15000`)
- `BROWSER_DIAG_HTTP_TARGET` (default `http://ipv6-diag.r.selvakn.in/diag`)
- `BROWSER_DIAG_HTTPS_TARGET` (default `https://ipv6-diag.r.selvakn.in/diag`)
- `BROWSER_DIAG_ICMP_TARGET` (default `https://ipv6-diag.r.selvakn.in/diag`)
- `BROWSER_DIAG_STUN_TARGET` (default `stun:ipv6-diag.r.selvakn.in:3478`)
- `BROWSER_DIAG_TURN_TARGET` (default `turn:ipv6-diag.r.selvakn.in:3478?transport=udp`)
- `BROWSER_DIAG_TURN_WINDOW_SECONDS` (default `10`)
- `BROWSER_DIAG_TURN_PAYLOAD_BYTES` (default `1024`)
- `BROWSER_DIAG_TURN_MESSAGES_PER_SEC` (default `20`)
- `BROWSER_DIAG_TURN_QUALITY_THRESHOLD_RATIO` (default `0.90`)

## TURN runtime configuration

The server can run diagnostics APIs and embedded TURN relay in one process.

### Environment variables

- `TURN_ENABLED` (`true`/`false`, default `false`)
- `TURN_REALM` (default `androidipv6diag`)
- `TURN_CREDENTIALS_TOKEN` (optional bearer token for `/turn/credentials`)
- `TURN_UDP4_ADDR` (default `0.0.0.0:3478`)
- `TURN_UDP6_ADDR` (default `[::]:3478`)
- `TURN_TCP4_ADDR` (default `0.0.0.0:3478`)
- `TURN_TCP6_ADDR` (default `[::]:3478`)
- `TURN_PUBLIC_IPV4` (optional, reserved for future relay address advertisement)
- `TURN_PUBLIC_IPV6` (optional, reserved for future relay address advertisement)
- `APP_HTTP_PORT` (default `8080` in container image; controls HTTP bind for both IPv4 and IPv6)

Credential leases are valid for 5 minutes and stored in memory only.

## VPS Docker example

```bash
docker run --rm \
  --network host \
  -e TURN_ENABLED=true \
  -e TURN_REALM=androidipv6diag \
  -e TURN_CREDENTIALS_TOKEN=changeme \
  ghcr.io/selvakn/androidipv6diag-server:latest
```

With host networking, the container defaults to HTTP on `:${APP_HTTP_PORT}` (`8080` by default), so Caddy (or another reverse proxy) can keep `:80/:443` and forward to `127.0.0.1:<APP_HTTP_PORT>`.
