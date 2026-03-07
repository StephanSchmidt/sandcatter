#!/bin/sh
set -e

# Sandcatter installer
# Usage: curl -sSfL https://raw.githubusercontent.com/StephanSchmidt/sandcatter/main/install.sh | bash

REPO="StephanSchmidt/sandcatter"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"

OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

case "$ARCH" in
  x86_64|amd64) ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *) echo "Unsupported architecture: $ARCH" >&2; exit 1 ;;
esac

case "$OS" in
  linux|darwin) ;;
  *) echo "Unsupported OS: $OS" >&2; exit 1 ;;
esac

# Get latest version
VERSION=$(curl -sSfL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | sed -E 's/.*"v([^"]+)".*/\1/')

if [ -z "$VERSION" ]; then
  echo "Failed to determine latest version" >&2
  exit 1
fi

FILENAME="sandcatter_${VERSION}_${OS}_${ARCH}.tar.gz"
URL="https://github.com/${REPO}/releases/download/v${VERSION}/${FILENAME}"

echo "Installing sandcatter v${VERSION} (${OS}/${ARCH})..."

TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT

curl -sSfL "$URL" -o "${TMP}/${FILENAME}"
tar -xzf "${TMP}/${FILENAME}" -C "$TMP"

if [ -w "$INSTALL_DIR" ]; then
  mv "${TMP}/sandcatter" "${INSTALL_DIR}/sandcatter"
else
  sudo mv "${TMP}/sandcatter" "${INSTALL_DIR}/sandcatter"
fi

chmod +x "${INSTALL_DIR}/sandcatter"

echo "sandcatter v${VERSION} installed to ${INSTALL_DIR}/sandcatter"
