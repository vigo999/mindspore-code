#!/usr/bin/env bash
set -euo pipefail

REPO="vigo999/ms-cli"
INSTALL_DIR="$HOME/.ms-cli/bin"
BINARY_NAME="mscli"

# Detect OS.
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
case "$OS" in
  linux)  OS="linux" ;;
  darwin) OS="darwin" ;;
  *)
    echo "Error: unsupported OS: $OS" >&2
    exit 1
    ;;
esac

# Detect architecture.
ARCH="$(uname -m)"
case "$ARCH" in
  x86_64|amd64) ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *)
    echo "Error: unsupported architecture: $ARCH" >&2
    exit 1
    ;;
esac

echo "Detected: ${OS}/${ARCH}"

# Fetch latest release tag.
echo "Fetching latest release..."
LATEST="$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | sed -E 's/.*"tag_name":\s*"([^"]+)".*/\1/')"

if [ -z "$LATEST" ]; then
  echo "Error: could not determine latest release" >&2
  exit 1
fi

echo "Latest release: ${LATEST}"

# Build download URL.
ASSET="ms-cli-${OS}-${ARCH}"
URL="https://github.com/${REPO}/releases/download/${LATEST}/${ASSET}"

# Download binary.
echo "Downloading ${URL}..."
mkdir -p "$INSTALL_DIR"
curl -fSL -o "${INSTALL_DIR}/${BINARY_NAME}" "$URL"
chmod +x "${INSTALL_DIR}/${BINARY_NAME}"

echo ""
echo "Installed ms-cli ${LATEST} to ${INSTALL_DIR}/${BINARY_NAME}"

# PATH hint.
if ! echo "$PATH" | tr ':' '\n' | grep -qx "$INSTALL_DIR"; then
  echo ""
  echo "Add ms-cli to your PATH by adding this to your shell profile:"
  echo ""
  echo "  export PATH=\"${INSTALL_DIR}:\$PATH\""
fi
