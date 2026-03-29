#!/bin/sh
# chagg installer — downloads the latest (or a pinned) release binary from GitHub.
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/codested/chagg/main/install.sh | sh
#   curl -fsSL https://raw.githubusercontent.com/codested/chagg/main/install.sh | sh -s -- --version v1.2.3
#   curl -fsSL https://raw.githubusercontent.com/codested/chagg/main/install.sh | sh -s -- --dir ~/.local/bin
#
set -e

REPO="codested/chagg"
INSTALL_DIR="/usr/local/bin"
VERSION=""

# ── Parse flags ──────────────────────────────────────────────────────────────

while [ $# -gt 0 ]; do
  case "$1" in
    --version)  VERSION="$2";     shift 2 ;;
    --dir)      INSTALL_DIR="$2"; shift 2 ;;
    --help|-h)
      echo "Usage: install.sh [--version VERSION] [--dir INSTALL_DIR]"
      echo ""
      echo "  --version   Pin a specific release (e.g. v1.2.3). Default: latest."
      echo "  --dir       Installation directory. Default: /usr/local/bin"
      exit 0
      ;;
    *) echo "Unknown flag: $1"; exit 1 ;;
  esac
done

# ── Detect OS and architecture ───────────────────────────────────────────────

OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"

case "$OS" in
  linux)  OS="linux"  ;;
  darwin) OS="darwin" ;;
  mingw*|msys*|cygwin*) OS="windows" ;;
  *)
    echo "Error: unsupported operating system: $OS"
    exit 1
    ;;
esac

case "$ARCH" in
  x86_64|amd64)  ARCH="amd64" ;;
  aarch64|arm64)  ARCH="arm64" ;;
  *)
    echo "Error: unsupported architecture: $ARCH"
    exit 1
    ;;
esac

BINARY="chagg-${OS}-${ARCH}"
if [ "$OS" = "windows" ]; then
  BINARY="${BINARY}.exe"
fi

# ── Resolve version ─────────────────────────────────────────────────────────

if [ -z "$VERSION" ]; then
  echo "Fetching latest release..."
  VERSION="$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
    | grep '"tag_name"' | head -1 | sed 's/.*"tag_name": *"\([^"]*\)".*/\1/')"
  if [ -z "$VERSION" ]; then
    echo "Error: could not determine latest version."
    exit 1
  fi
fi

echo "Installing chagg ${VERSION} (${OS}/${ARCH})..."

# ── Download ─────────────────────────────────────────────────────────────────

DOWNLOAD_URL="https://github.com/${REPO}/releases/download/${VERSION}/${BINARY}"
TMP_DIR="$(mktemp -d)"
TMP_FILE="${TMP_DIR}/${BINARY}"

cleanup() { rm -rf "$TMP_DIR"; }
trap cleanup EXIT

echo "Downloading ${DOWNLOAD_URL}..."
if ! curl -fsSL -o "$TMP_FILE" "$DOWNLOAD_URL"; then
  echo "Error: download failed. Check that version ${VERSION} exists and includes a ${BINARY} asset."
  echo "Available releases: https://github.com/${REPO}/releases"
  exit 1
fi

# ── Verify checksum ─────────────────────────────────────────────────────────

CHECKSUMS_URL="https://github.com/${REPO}/releases/download/${VERSION}/checksums.txt"
CHECKSUMS_FILE="${TMP_DIR}/checksums.txt"

if curl -fsSL -o "$CHECKSUMS_FILE" "$CHECKSUMS_URL" 2>/dev/null; then
  EXPECTED="$(grep "${BINARY}" "$CHECKSUMS_FILE" | awk '{print $1}')"
  if [ -n "$EXPECTED" ]; then
    if command -v sha256sum >/dev/null 2>&1; then
      ACTUAL="$(sha256sum "$TMP_FILE" | awk '{print $1}')"
    elif command -v shasum >/dev/null 2>&1; then
      ACTUAL="$(shasum -a 256 "$TMP_FILE" | awk '{print $1}')"
    else
      ACTUAL=""
      echo "Warning: no sha256sum or shasum found, skipping checksum verification."
    fi

    if [ -n "$ACTUAL" ]; then
      if [ "$ACTUAL" != "$EXPECTED" ]; then
        echo "Error: checksum mismatch!"
        echo "  Expected: ${EXPECTED}"
        echo "  Actual:   ${ACTUAL}"
        exit 1
      fi
      echo "Checksum verified."
    fi
  fi
else
  echo "Warning: could not download checksums, skipping verification."
fi

# ── Install ──────────────────────────────────────────────────────────────────

chmod +x "$TMP_FILE"

DEST="${INSTALL_DIR}/chagg"
if [ "$OS" = "windows" ]; then
  DEST="${INSTALL_DIR}/chagg.exe"
fi

# Try direct copy; fall back to sudo if permission denied.
if ! cp "$TMP_FILE" "$DEST" 2>/dev/null; then
  echo "Permission denied — retrying with sudo..."
  sudo cp "$TMP_FILE" "$DEST"
fi

echo ""
echo "chagg ${VERSION} installed to ${DEST}"

# Verify it runs.
if command -v chagg >/dev/null 2>&1; then
  echo ""
  chagg --version 2>/dev/null || true
else
  echo ""
  echo "Note: ${INSTALL_DIR} is not in your PATH."
  echo "Add it with:  export PATH=\"${INSTALL_DIR}:\$PATH\""
fi
