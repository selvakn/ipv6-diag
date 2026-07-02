# Android IPv6 Diag Server

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
