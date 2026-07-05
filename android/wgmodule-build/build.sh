#!/usr/bin/env bash
# Build wglib.aar from wgmodule/ using gomobile bind.
# Output: android/app/libs/wglib.aar
#
# Prerequisites:
#   - Go 1.25+  (must match wgmodule/go.mod minimum)
#   - Android NDK r25c+ (or via Android Studio SDK Manager)
#   - ANDROID_HOME set (or android-ndk-dir argument)
#
# Usage:
#   cd <repo-root>
#   bash android/wgmodule-build/build.sh
#
# First-time setup:
#   go install golang.org/x/mobile/cmd/gomobile@latest
#   gomobile init

set -euo pipefail

REPO_ROOT="$(git -C "$(dirname "$0")" rev-parse --show-toplevel)"
OUTPUT_AAR="$REPO_ROOT/android/app/libs/wglib.aar"
MODULE_PATH="$REPO_ROOT/wgmodule"

echo "[wgmodule-build] Building wglib.aar from $MODULE_PATH"
echo "[wgmodule-build] Output: $OUTPUT_AAR"

mkdir -p "$(dirname "$OUTPUT_AAR")"

cd "$MODULE_PATH"

gomobile bind \
  -target android \
  -androidapi 26 \
  -o "$OUTPUT_AAR" \
  .

echo "[wgmodule-build] Done: $OUTPUT_AAR"
ls -lh "$OUTPUT_AAR"
