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
ENV APP_HTTP_PORT=8080
EXPOSE 8080 3478/tcp 3478/udp
HEALTHCHECK --interval=10s --timeout=3s --start-period=5s \
    CMD curl -sf http://localhost:8080/health || exit 1
ENTRYPOINT ["/server"]
# Production default: store DB on the mounted volume.
# APP_HTTP_PORT controls HTTP bind port (IPv4 + IPv6 defaults).
CMD ["--db", "/data/reports.db"]
