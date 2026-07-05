# wgmodule-build — WireGuard Android AAR Builder

This directory contains the script that compiles `wgmodule/` (the shared Go WireGuard library) into `android/app/libs/wglib.aar` using **gomobile bind**.

## Prerequisites

| Tool | Version | Install |
|------|---------|---------|
| Go | 1.25+ | [go.dev/dl](https://go.dev/dl/) |
| Android NDK | r25c+ | Via Android Studio → SDK Manager → SDK Tools |
| gomobile | latest | `go install golang.org/x/mobile/cmd/gomobile@latest` |

## First-Time Setup

```bash
# 1. Install gomobile
go install golang.org/x/mobile/cmd/gomobile@latest

# 2. Initialise gomobile (downloads Android NDK metadata if needed)
gomobile init

# 3. Set ANDROID_HOME (if not already in your shell profile)
export ANDROID_HOME=$HOME/Android/Sdk   # adjust to your SDK path
```

## Building

```bash
# From repo root:
bash android/wgmodule-build/build.sh
```

The generated `android/app/libs/wglib.aar` is committed to git so that Android developers without the Go toolchain can build the app without re-running this script.

## When to Rebuild

Rebuild the `.aar` whenever files in `wgmodule/` change:

```bash
# Quick check: any wgmodule changes since last aar build?
git diff --name-only HEAD -- wgmodule/
```

## Gradle Integration

`android/app/build.gradle.kts` includes the `.aar` via:

```kotlin
dependencies {
    implementation(fileTree(mapOf("dir" to "libs", "include" to listOf("*.aar"))))
    // ...
}
```

The generated Java/Kotlin package name is `wgmodule` (lowercase of the Go module base name).
Exported types: `WireGuardResult`, `WireGuardCallback` (interface), `Wgmodule.runWireGuardTestAsync(...)`.
