# Quickstart: Go CLI Diagnostic Client

## Build locally

```bash
cd cli
go build -o ipv6diag .
```

## Run all tests (both stacks, default server)

```bash
./ipv6diag
```

## Force IPv4 only

```bash
./ipv6diag --ipv4
```

## Force IPv6 only

```bash
./ipv6diag --ipv6
```

## Run against a custom server

```bash
./ipv6diag --server https://staging.example.com
```

## Run against local dev server (self-signed cert)

```bash
./ipv6diag --server https://localhost:8443 --insecure
```

## Run only STUN and TURN tests

```bash
./ipv6diag --tests stun,turn
```

## Upload results to the dashboard

```bash
./ipv6diag --upload
```

## JSON output for CI / scripting

```bash
./ipv6diag --json | jq '.results[] | select(.status != "passed")'
```

## Use in CI — fail build if any test fails

```bash
./ipv6diag --ipv4 --tests http,https,stun --json --timeout 10000
echo "Exit: $?"
```

## Cross-compile manually

```bash
cd cli
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -o ipv6diag-windows-amd64.exe .
GOOS=linux   GOARCH=arm64 CGO_ENABLED=0 go build -o ipv6diag-linux-arm64 .
GOOS=darwin  GOARCH=arm64 CGO_ENABLED=0 go build -o ipv6diag-darwin-arm64 .
```

## Integration test scenarios (manual)

1. **IPv4-only host**: Run `--ipv6` → all tests should fail with clear "no route" / connection error
2. **IPv6-only host**: Run `--ipv4` → all tests should fail
3. **Dual-stack**: Run `--both` → both blocks should pass
4. **TURN disabled server**: Run `--tests turn` → status shows `skipped` (credential mode: none)
5. **--json + exit code**: Assert exit code 1 when any test fails
