#!/usr/bin/env bash
set -euo pipefail

REPO="selvakn/ipv6-diag"
BIN="ipv6diag"

os=$(uname -s | tr '[:upper:]' '[:lower:]')
arch=$(uname -m)

case "$os" in
  linux)  goos="linux" ;;
  darwin) goos="darwin" ;;
  *)      echo "Unsupported OS: $os" >&2; exit 1 ;;
esac

case "$arch" in
  x86_64|amd64) goarch="amd64" ;;
  arm64|aarch64) goarch="arm64" ;;
  *) echo "Unsupported architecture: $arch" >&2; exit 1 ;;
esac

asset="${BIN}-${goos}-${goarch}"
url="https://github.com/${REPO}/releases/latest/download/${asset}"

dest="${INSTALL_DIR:-/usr/local/bin}/${BIN}"

echo "Downloading ${asset} from ${REPO} ..."
curl -fsSL "$url" -o "$dest"
chmod +x "$dest"
echo "Installed to $dest"
$dest --version
