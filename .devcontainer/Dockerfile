# .devcontainer/Dockerfile
# Full development environment for IPv6 Diagnostic Tool.
#
# Provides:
#   • Go 1.24 (via golang:1.24-bookworm base)
#   • OpenJDK 21 (for Gradle / Android builds)
#   • Android SDK command-line tools + NDK r25c + platform-tools
#   • gomobile (pre-initialised against the NDK)
#   • Common dev utilities: git, make, curl, jq, docker CLI
#
# This image is intentionally built once and cached by VS Code / Cursor;
# rebuilds only happen when this file or devcontainer.json changes.
# ─────────────────────────────────────────────────────────────────────────────

FROM golang:1.24-bookworm

ARG CMDLINE_TOOLS_BUILD=11076708
ARG ANDROID_NDK_VERSION=25.2.9519653
ARG ANDROID_PLATFORM=35
ARG ANDROID_BUILD_TOOLS=35.0.0

ENV ANDROID_HOME=/opt/android-sdk \
    JAVA_HOME=/usr/lib/jvm/java-21-openjdk-amd64 \
    GOTOOLCHAIN=local \
    DEBIAN_FRONTEND=noninteractive
ENV PATH=$PATH:$ANDROID_HOME/cmdline-tools/latest/bin:$ANDROID_HOME/platform-tools

# ── System packages ───────────────────────────────────────────────────────────
RUN apt-get update && apt-get install -y --no-install-recommends \
        openjdk-21-jdk \
        wget unzip zip \
        sudo make curl jq \
        build-essential \
        ca-certificates \
        less vim-tiny \
    && rm -rf /var/lib/apt/lists/*

# ── Non-root vscode user (standard devcontainer convention) ───────────────────
RUN useradd -ms /bin/bash -u 1000 vscode \
    && echo "vscode ALL=(ALL) NOPASSWD:ALL" >> /etc/sudoers.d/vscode \
    && chmod 0440 /etc/sudoers.d/vscode

# ── Android command-line tools ────────────────────────────────────────────────
RUN mkdir -p "$ANDROID_HOME/cmdline-tools" \
    && wget -q \
        "https://dl.google.com/android/repository/commandlinetools-linux-${CMDLINE_TOOLS_BUILD}_latest.zip" \
        -O /tmp/ct.zip \
    && unzip -q /tmp/ct.zip -d /tmp/ \
    && mv /tmp/cmdline-tools "$ANDROID_HOME/cmdline-tools/latest" \
    && rm /tmp/ct.zip \
    && chown -R vscode:vscode "$ANDROID_HOME"

# ── Android SDK packages (run as vscode so sdkmanager respects $ANDROID_HOME) ─
USER vscode
ENV GOPATH=/home/vscode/go
ENV PATH=$PATH:$GOPATH/bin

RUN yes | sdkmanager --licenses > /dev/null 2>&1 || true \
    && sdkmanager \
        "ndk;${ANDROID_NDK_VERSION}" \
        "platforms;android-${ANDROID_PLATFORM}" \
        "build-tools;${ANDROID_BUILD_TOOLS}" \
        "platform-tools"

# ── gomobile (pre-initialised against the NDK) ────────────────────────────────
RUN go install golang.org/x/mobile/cmd/gomobile@latest \
    && ANDROID_NDK_HOME="$ANDROID_HOME/ndk/${ANDROID_NDK_VERSION}" gomobile init

USER root
WORKDIR /workspaces
