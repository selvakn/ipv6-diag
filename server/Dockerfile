FROM golang:1.25-alpine AS builder
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w -X main.version=$(git describe --tags --always 2>/dev/null || echo dev)" \
    -o server ./cmd/server

FROM alpine:3.21
RUN apk add --no-cache ca-certificates curl
COPY --from=builder /build/server /server
EXPOSE 80 443 3478/tcp 3478/udp
HEALTHCHECK --interval=10s --timeout=3s --start-period=5s \
    CMD curl -sf http://localhost:80/health || exit 1
ENTRYPOINT ["/server"]
# Production default: store DB on the mounted volume.
# Override via docker-compose `command:` for local dev.
CMD ["--db", "/data/reports.db"]
