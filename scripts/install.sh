#!/bin/sh
# Install script for langsmith CLI
# Usage: curl -sSL https://raw.githubusercontent.com/langchain-ai/langsmith-cli/main/scripts/install.sh | sh
#
# Environment variables:
#   INSTALL_DIR   — directory to install to (default: auto-detect)
#   VERSION       — specific version to install (default: latest)
#   GITHUB_TOKEN  — token for the release lookup, to avoid anonymous rate limits

set -e

REPO="langchain-ai/langsmith-cli"
BINARY="langsmith"

# Detect OS
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
case "$OS" in
  linux)  OS="linux" ;;
  darwin) OS="darwin" ;;
  *)      echo "Unsupported OS: $OS" >&2; exit 1 ;;
esac

# Detect architecture
ARCH="$(uname -m)"
case "$ARCH" in
  x86_64|amd64)  ARCH="amd64" ;;
  arm64|aarch64) ARCH="arm64" ;;
  *)             echo "Unsupported architecture: $ARCH" >&2; exit 1 ;;
esac

# Determine install directory
if [ -z "$INSTALL_DIR" ]; then
  # Try /usr/local/bin first, fall back to ~/.local/bin
  if [ -w /usr/local/bin ]; then
    INSTALL_DIR="/usr/local/bin"
  else
    INSTALL_DIR="${HOME}/.local/bin"
  fi
fi

# Get version
if [ -z "$VERSION" ]; then
  API_URL="https://api.github.com/repos/${REPO}/releases/latest"

  # GITHUB_TOKEN, then an authenticated gh, then anonymous
  fetch_latest() {
    if [ -n "$GITHUB_TOKEN" ]; then
      curl -fsSL -H "Accept: application/vnd.github+json" \
        -H "Authorization: Bearer $GITHUB_TOKEN" "$API_URL"
    elif command -v gh >/dev/null 2>&1; then
      gh api -H "Accept: application/vnd.github+json" \
        "repos/${REPO}/releases/latest" 2>/dev/null \
        || curl -fsSL -H "Accept: application/vnd.github+json" "$API_URL"
    else
      curl -fsSL -H "Accept: application/vnd.github+json" "$API_URL"
    fi
  }

  VERSION=""
  i=1
  while [ "$i" -le 3 ]; do
    VERSION="$(fetch_latest | grep '"tag_name"' | sed -E 's/.*"tag_name": *"([^"]+)".*/\1/')"
    [ -n "$VERSION" ] && break
    [ "$i" -lt 3 ] && sleep "$i"
    i=$((i + 1))
  done

  if [ -z "$VERSION" ]; then
    echo "Failed to determine latest version (GitHub API unreachable or rate-limited)." >&2
    echo "Set GITHUB_TOKEN, run 'gh auth login', or set VERSION to install a specific release." >&2
    exit 1
  fi
fi

# Strip v prefix for filename
VERSION_NUM="${VERSION#v}"

# Build download URL
ARCHIVE="${BINARY}_${OS}_${ARCH}.tar.gz"
URL="https://github.com/${REPO}/releases/download/${VERSION}/${ARCHIVE}"
CHECKSUM_URL="https://github.com/${REPO}/releases/download/${VERSION}/checksums.txt"

echo "Installing ${BINARY} ${VERSION} (${OS}/${ARCH})..."

# Create temp directory
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

# Download archive and checksums
curl -sSL -o "${TMP_DIR}/${ARCHIVE}" "$URL"
curl -sSL -o "${TMP_DIR}/checksums.txt" "$CHECKSUM_URL"

# Verify checksum
cd "$TMP_DIR"
EXPECTED="$(grep "${ARCHIVE}" checksums.txt | awk '{print $1}')"
if [ -n "$EXPECTED" ]; then
  if command -v sha256sum >/dev/null 2>&1; then
    ACTUAL="$(sha256sum "${ARCHIVE}" | awk '{print $1}')"
  elif command -v shasum >/dev/null 2>&1; then
    ACTUAL="$(shasum -a 256 "${ARCHIVE}" | awk '{print $1}')"
  else
    echo "Warning: cannot verify checksum (no sha256sum or shasum found)" >&2
    ACTUAL="$EXPECTED"
  fi

  if [ "$ACTUAL" != "$EXPECTED" ]; then
    echo "Checksum verification failed!" >&2
    echo "  Expected: $EXPECTED" >&2
    echo "  Actual:   $ACTUAL" >&2
    exit 1
  fi
fi

# Extract
tar xzf "${ARCHIVE}"

# Install
mkdir -p "$INSTALL_DIR"
mv "${BINARY}" "${INSTALL_DIR}/${BINARY}"
chmod +x "${INSTALL_DIR}/${BINARY}"

echo "Installed ${BINARY} to ${INSTALL_DIR}/${BINARY}"

# Check if INSTALL_DIR is in PATH
case ":$PATH:" in
  *":${INSTALL_DIR}:"*) ;;
  *)
    echo ""
    echo "Add ${INSTALL_DIR} to your PATH:"
    echo "  export PATH=\"${INSTALL_DIR}:\$PATH\""
    echo ""
    echo "To make it permanent, add that line to your ~/.bashrc or ~/.zshrc"
    ;;
esac

echo ""
echo "Run: ${BINARY} --version"
