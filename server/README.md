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

Credential leases are valid for 5 minutes and stored in memory only.

## VPS Docker example

```bash
docker run --rm \
  -p 80:80 \
  -p 443:443 \
  -p 3478:3478/tcp \
  -p 3478:3478/udp \
  -e TURN_ENABLED=true \
  -e TURN_REALM=androidipv6diag \
  -e TURN_CREDENTIALS_TOKEN=changeme \
  ghcr.io/selvakn/androidipv6diag-server:latest
```
