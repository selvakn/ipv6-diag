# CLI Command Contract: ipv6diag

## Invocation

```
ipv6diag [flags]
```

## Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--ipv4` | bool | false | Force IPv4 stack only |
| `--ipv6` | bool | false | Force IPv6 stack only |
| `--both` | bool | false | Run both stacks (default when no stack flag given) |
| `--server` | string | `https://ipv6-diag.selvakn.in` | Base URL of the diagnostic server |
| `--tests` | string | `http,https,icmp,stun,turn` | Comma-separated test subset |
| `--timeout` | int | `15000` | Per-test timeout in milliseconds |
| `--turn-token` | string | `""` | Bearer token for `/turn/credentials` (or `TURN_TOKEN` env var) |
| `--upload` | bool | false | POST results to `/api/reports` after run |
| `--insecure` | bool | false | Skip TLS certificate verification (prints warning to stderr) |
| `--insecure-upload` | bool | false | Allow `--upload` when `--insecure` is active |
| `--json` | bool | false | Emit results as JSON to stdout instead of human-readable text |
| `--version` | bool | false | Print version and exit |

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | All executed tests passed |
| 1 | One or more tests failed or errored |
| 2 | Invalid flags or configuration error |

## Human-readable Output Format

```
ipv6diag v1.1.x — targeting https://ipv6-diag.selvakn.in

── IPv4 ──────────────────────────────────────────────────────────
  HTTP    passed    112ms   http://ipv6-diag.selvakn.in/diag
  HTTPS   passed     98ms   https://ipv6-diag.selvakn.in/diag
  ICMP    passed    101ms   https://ipv6-diag.selvakn.in/diag
  STUN    passed     45ms   stun:ipv6-diag.selvakn.in:3478
  TURN    passed   10.2s    turn:ipv6-diag.selvakn.in:3478  342 kbps  rtt=18ms  quality=0.97

── IPv6 ──────────────────────────────────────────────────────────
  HTTP    passed    108ms   http://ipv6-diag.selvakn.in/diag
  HTTPS   passed     94ms   https://ipv6-diag.selvakn.in/diag
  ICMP    passed     99ms   https://ipv6-diag.selvakn.in/diag
  STUN    passed     41ms   stun:ipv6-diag.selvakn.in:3478
  TURN    passed   10.1s    turn:ipv6-diag.selvakn.in:3478  398 kbps  rtt=14ms  quality=0.99

── Summary ───────────────────────────────────────────────────────
  10/10 passed
```

Live progress line during TURN (written to stderr, replaced when done):
```
  TURN  ⠸  7s / 10s…
```

## JSON Output Format (--json)

```json
{
  "version": "1.1.x",
  "server": "https://ipv6-diag.selvakn.in",
  "session_id": "<uuid>",
  "started_at": "<iso8601>",
  "finished_at": "<iso8601>",
  "results": [
    {
      "test_type": "HTTP",
      "address_family": "IPv4",
      "target": "http://ipv6-diag.selvakn.in/diag",
      "status": "passed",
      "duration_ms": 112,
      "latency_ms": 112,
      "failure_reason": null,
      "transfer_rate_kbps": null,
      "delivery_quality_ratio": null
    }
  ],
  "pass_count": 10,
  "total_count": 10
}
```

## Stderr Warnings

- `WARNING: TLS verification disabled — do not use in production` (when `--insecure`)
- `WARNING: uploading to unverified endpoint requires --insecure-upload` (when `--upload --insecure` without `--insecure-upload`)
