# Quickstart: Embedded TURN Relay Service

## 1) Build server image

```bash
cd server
docker build -t androidipv6diag-server:turn .
```

## 2) Run on VPS with dual-stack TURN listeners

```bash
docker run --rm \
  -p 80:80 \
  -p 443:443 \
  -p 3478:3478/tcp \
  -p 3478:3478/udp \
  -e TURN_ENABLED=true \
  -e TURN_REALM=androidipv6diag \
  -e TURN_UDP4_ADDR=0.0.0.0:3478 \
  -e TURN_UDP6_ADDR=[::]:3478 \
  -e TURN_TCP4_ADDR=0.0.0.0:3478 \
  -e TURN_TCP6_ADDR=[::]:3478 \
  -e TURN_PUBLIC_IPV4=<public-ipv4> \
  -e TURN_PUBLIC_IPV6=<public-ipv6> \
  -e TURN_CREDENTIALS_TOKEN=<shared-token> \
  androidipv6diag-server:turn
```

## 3) Verify diagnostics endpoint

```bash
curl -fsS http://<host>/health
curl -fsS http://<host>/diag
```

## 4) Fetch TURN credentials

```bash
curl -fsS \
  -H "Authorization: Bearer <shared-token>" \
  http://<host>/turn/credentials
```

Expected: JSON payload with `username`, `password`, `ttl_seconds: 300`, `realm`, and TURN URIs.

## 5) Validate listener startup behavior

- Confirm logs report each listener state (`active` or `degraded`).
- Confirm service remains available if one listener family fails.

## 6) Validation matrix

- IPv4 + UDP allocation: success
- IPv4 + TCP allocation: success
- IPv6 + UDP allocation: success
- IPv6 + TCP allocation: success
- Expired credential (>5 minutes): allocation rejected
