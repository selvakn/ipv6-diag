#!/usr/bin/env bash
# .devcontainer/post-create.sh
# Runs once after the container is created.
# Pre-downloads Go module dependencies so the first build is fast.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$REPO_ROOT"

echo ""
echo "╔══════════════════════════════════════════════════╗"
echo "║   IPv6 Diagnostic Tool — Dev Container Ready    ║"
echo "╚══════════════════════════════════════════════════╝"
echo ""
echo "Pre-downloading Go module dependencies..."

(cd server   && go mod download && echo "  ✓ server")
(cd wgmodule && go mod download && echo "  ✓ wgmodule")
(cd cli      && go mod download && echo "  ✓ cli")

echo ""
echo "Environment:"
echo "  Go      $(go version)"
echo "  Java    $(java -version 2>&1 | head -1)"
echo "  gomobile $(gomobile version 2>&1 || echo 'not in PATH?')"
echo "  NDK     ${ANDROID_HOME}/ndk/$(ls ${ANDROID_HOME}/ndk 2>/dev/null | head -1)"
echo ""
echo "Run 'make help' to see all build targets."
echo ""
