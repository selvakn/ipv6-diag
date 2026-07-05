# Root Makefile — orchestrates server, CLI, wglib.aar, Android APK, and Docker Compose.
# Run 'make help' to see all available targets.

.PHONY: help \
        server-build server-run \
        cli-build \
        wglib-build wglib-build-local \
        android-build \
        build-all \
        up up-server down logs test-server apk-install \
        clean

WGLIB_AAR           := android/app/libs/wglib.aar
WGLIB_BUILDER_IMAGE ?= ipv6diag-wglib-builder
ANDROID_SDK_IMAGE   ?= thyrlian/android-sdk:latest

# ─────────────────────────────────────────────────────────────────────────────
help:
	@echo ""
	@echo "  IPv6 Diagnostic Tool — Build Targets"
	@echo ""
	@echo "  Server (Go):"
	@echo "    make server-build        Build static binary → server/bin/server"
	@echo "    make server-run          Build and run server locally (HTTP only)"
	@echo ""
	@echo "  CLI (Go):"
	@echo "    make cli-build           Build CLI binary → cli/ipv6diag"
	@echo "                             (uses local Go if present, else Docker)"
	@echo ""
	@echo "  WireGuard Android library (gomobile → AAR):"
	@echo "    make wglib-build         Build wglib.aar via Docker  ← recommended"
	@echo "                             (no local NDK needed; ~3 GB NDK cached after first run)"
	@echo "    make wglib-build-local   Build wglib.aar with local gomobile install"
	@echo ""
	@echo "  Android APK:"
	@echo "    make android-build       Build debug APK"
	@echo "                             (runs wglib-build automatically if AAR is missing)"
	@echo ""
	@echo "  Everything:"
	@echo "    make build-all           server + CLI + wglib.aar + Android APK"
	@echo ""
	@echo "  Docker Compose:"
	@echo "    make up                  Start server + Android emulator"
	@echo "    make up-server           Start server only"
	@echo "    make down                Stop all containers"
	@echo "    make logs                Tail all container logs"
	@echo "    make test-server         Run curl smoke tests against the server container"
	@echo "    make apk-install         Build APK and install into the running emulator"
	@echo ""
	@echo "  Ports:"
	@echo "    http://localhost:8080/diag    Diagnostic server (HTTP)"
	@echo "    http://localhost:6080         Android emulator noVNC UI"
	@echo ""

# ── Server ───────────────────────────────────────────────────────────────────
server-build:
	$(MAKE) -C server build

server-run:
	$(MAKE) -C server run

# ── CLI ──────────────────────────────────────────────────────────────────────
# Uses local Go when available; falls back to the Docker builder so CI
# machines without Go installed can still produce the binary.
cli-build:
	@if command -v go >/dev/null 2>&1; then \
		echo "Building CLI with local Go..."; \
		cd cli && GOTOOLCHAIN=local CGO_ENABLED=0 go build -ldflags="-s -w" -o ipv6diag .; \
		echo "  Built: cli/ipv6diag"; \
	else \
		echo "go not found — building CLI inside Docker..."; \
		DOCKER_BUILDKIT=1 docker build \
			--file docker/Dockerfile.cli \
			--target cli-export \
			--output "type=local,dest=cli" \
			.; \
		echo "  Built: cli/ipv6diag"; \
	fi

# ── WireGuard AAR ─────────────────────────────────────────────────────────────
# wglib-build: always uses Docker so no local NDK is required.
# The NDK layer (~3 GB) is cached after the first build.
wglib-build:
	@echo "Building wglib.aar in Docker..."
	@echo "(First run downloads the Android NDK — this may take several minutes.)"
	mkdir -p android/app/libs
	DOCKER_BUILDKIT=1 docker build \
		--file docker/Dockerfile.wglib \
		--target wglib-export \
		--output "type=local,dest=android/app/libs" \
		.
	@echo "  Built: $(WGLIB_AAR)"

# wglib-build-local: convenience target for developers with gomobile installed.
wglib-build-local:
	bash android/wgmodule-build/build.sh

# Make target for dependency tracking: rebuild AAR if any wgmodule source changes.
$(WGLIB_AAR): $(wildcard wgmodule/*.go) wgmodule/go.mod wgmodule/go.sum
	$(MAKE) wglib-build

# ── Android APK ──────────────────────────────────────────────────────────────
# Depends on $(WGLIB_AAR) so Make rebuilds the AAR when wgmodule changes.
# If the AAR already exists and wgmodule hasn't changed, this step is skipped.
android-build: $(WGLIB_AAR)
	@if [ -n "$$ANDROID_HOME" ] || [ -d "$$HOME/Android/Sdk" ]; then \
		echo "Building APK with local Android SDK..."; \
		cd android && ./gradlew assembleDebug; \
	else \
		echo "ANDROID_HOME not set — building APK inside Docker..."; \
		docker run --rm \
			-v $(CURDIR)/android:/project \
			-v android-gradle-cache:/root/.gradle \
			-w /project \
			$(ANDROID_SDK_IMAGE) \
			bash -c "./gradlew assembleDebug"; \
	fi
	@echo "  Built: android/app/build/outputs/apk/debug/app-debug.apk"

# ── All artifacts ─────────────────────────────────────────────────────────────
build-all: server-build cli-build wglib-build android-build
	@echo ""
	@echo "All artifacts built:"
	@echo "  server/bin/server"
	@echo "  cli/ipv6diag"
	@echo "  $(WGLIB_AAR)"
	@echo "  android/app/build/outputs/apk/debug/app-debug.apk"

# ── Docker Compose ────────────────────────────────────────────────────────────
up:
	docker compose up -d server android
	@echo ""
	@echo "  Server:  http://localhost:8080/diag"
	@echo "  Android: http://localhost:6080  (emulator takes ~2 min to boot)"
	@echo ""
	@echo "  To install APK after android-build:  make apk-install"

up-server:
	docker compose up -d server
	@echo ""
	@echo "  Server: http://localhost:8080/diag"
	@echo "  Run smoke tests: make test-server"

down:
	docker compose down

logs:
	docker compose logs -f

test-server:
	docker compose run --rm test-server

apk-install: android-build
	docker compose --profile install run --rm apk-install

clean:
	docker compose down -v --remove-orphans
	$(MAKE) -C server clean
	rm -f cli/ipv6diag
	rm -f $(WGLIB_AAR)
