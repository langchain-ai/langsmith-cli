#!/bin/sh
# Install script for the langsmith CLI.
#
# Usage:
#   curl -fsSL https://cli.langsmith.com/install.sh | sh
#
# Environment variables:
#   INSTALL_DIR   directory to install into (default: /usr/local/bin when writable,
#                 otherwise $HOME/.local/bin)
#   VERSION       release tag to install, e.g. v0.4.1 (default: latest release)
#   GITHUB_TOKEN  token for the release lookup, to avoid anonymous rate limits
#
# Layout: pure helpers first (functional core), then everything that touches the
# network or filesystem (imperative shell), then main. scripts/install.ps1
# mirrors this structure function for function; keep the two in step.

set -eu

# ---------------------------------------------------------------------------
# Configuration
# ---------------------------------------------------------------------------

REPO="langchain-ai/langsmith-cli"
CLI_NAME="langsmith"
BINARY_NAME="langsmith"
ARCHIVE_EXT="tar.gz"
CHECKSUMS_NAME="checksums.txt"
RELEASE_API_URL="https://api.github.com/repos/${REPO}/releases/latest"
DOWNLOAD_BASE_URL="https://github.com/${REPO}/releases/download"
SYSTEM_INSTALL_DIR="/usr/local/bin"
USER_INSTALL_SUBDIR=".local/bin"
FETCH_ATTEMPTS=3

# ---------------------------------------------------------------------------
# Functional core
#
# Pure helpers: every input arrives as an argument or on stdin, every result
# goes to stdout, and nothing here reads the environment or touches disk. The
# resolve_* helpers print nothing and return non-zero when a value is
# unsupported, leaving the error message to the caller.
# ---------------------------------------------------------------------------

# Map `uname -s` onto a goreleaser OS name.
resolve_os() {
  case "$(printf '%s' "$1" | tr '[:upper:]' '[:lower:]')" in
    linux) printf 'linux' ;;
    darwin) printf 'darwin' ;;
    *) return 1 ;;
  esac
}

# Map `uname -m` onto a goreleaser arch name.
resolve_arch() {
  case "$1" in
    x86_64 | amd64) printf 'amd64' ;;
    arm64 | aarch64) printf 'arm64' ;;
    *) return 1 ;;
  esac
}

# resolve_archive_name <os> <arch>
resolve_archive_name() {
  printf '%s_%s_%s.%s' "$CLI_NAME" "$1" "$2" "$ARCHIVE_EXT"
}

# resolve_install_dir <override> <system_dir_writable: yes|no> <home>
#
# An explicit override wins, then the system directory when the caller found it
# writable, then the per-user fallback.
resolve_install_dir() {
  if [ -n "$1" ]; then
    printf '%s' "$1"
  elif [ "$2" = yes ]; then
    printf '%s' "$SYSTEM_INSTALL_DIR"
  else
    printf '%s/%s' "$3" "$USER_INSTALL_SUBDIR"
  fi
}

# Read release JSON on stdin, print its tag_name.
resolve_tag_name() {
  sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -n 1
}

# resolve_expected_checksum <archive_name>
#
# Reads checksums.txt on stdin and prints the hash recorded for the archive.
# Matches the whole file-name field: a bare prefix match would accept the hash
# of a different, longer asset name.
resolve_expected_checksum() {
  awk -v want="$1" '
    {
      name = $2
      sub(/^\*/, "", name)  # sha256sum marks binary mode with a leading *
      if (name == want) { print $1; exit }
    }
  '
}

# path_contains_dir <dir> <path_value>
path_contains_dir() {
  case ":$2:" in
    *":$1:"*) return 0 ;;
    *) return 1 ;;
  esac
}

# format_path_instructions <dir>
format_path_instructions() {
  printf '%s\n' \
    "" \
    "Add $1 to your PATH:" \
    "  export PATH=\"$1:\$PATH\"" \
    "" \
    "To make it permanent, add that line to your ~/.bashrc or ~/.zshrc"
}

# ---------------------------------------------------------------------------
# Imperative shell
#
# Everything below reads the environment, talks to the network, or writes to
# disk.
# ---------------------------------------------------------------------------

log() { printf '%s\n' "$*"; }
warn() { printf '%s\n' "$*" >&2; }
die() {
  warn "$*"
  exit 1
}

get_platform_name() { uname -s; }
get_machine_name() { uname -m; }

# GITHUB_TOKEN, then an authenticated gh, then anonymous.
fetch_release_json() {
  if [ -n "${GITHUB_TOKEN:-}" ]; then
    curl -fsSL -H 'Accept: application/vnd.github+json' \
      -H "Authorization: Bearer ${GITHUB_TOKEN}" "$RELEASE_API_URL"
  elif command -v gh >/dev/null 2>&1; then
    gh api -H 'Accept: application/vnd.github+json' \
      "repos/${REPO}/releases/latest" 2>/dev/null ||
      curl -fsSL -H 'Accept: application/vnd.github+json' "$RELEASE_API_URL"
  else
    curl -fsSL -H 'Accept: application/vnd.github+json' "$RELEASE_API_URL"
  fi
}

# Print the latest release tag, or return non-zero once the retries are spent.
fetch_latest_version() {
  local attempt tag
  attempt=1
  while [ "$attempt" -le "$FETCH_ATTEMPTS" ]; do
    tag="$(fetch_release_json 2>/dev/null | resolve_tag_name || true)"
    if [ -n "$tag" ]; then
      printf '%s' "$tag"
      return 0
    fi
    [ "$attempt" -lt "$FETCH_ATTEMPTS" ] && sleep "$attempt"
    attempt=$((attempt + 1))
  done
  return 1
}

# download_file <url> <dest>
download_file() {
  # -f so an HTTP error page is never saved as if it were the archive.
  curl -fsSL -o "$2" "$1"
}

# file_sha256 <path>
file_sha256() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | awk '{print $1}'
  elif command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$1" | awk '{print $1}'
  else
    return 1
  fi
}

# verify_checksum <archive_path> <archive_name> <checksums_path>
verify_checksum() {
  local expected actual
  expected="$(resolve_expected_checksum "$2" <"$3")"
  if [ -z "$expected" ]; then
    warn "Warning: $2 is not listed in ${CHECKSUMS_NAME}; skipping verification"
    return 0
  fi

  if ! actual="$(file_sha256 "$1")"; then
    warn "Warning: cannot verify checksum (no sha256sum or shasum found)"
    return 0
  fi

  if [ "$actual" != "$expected" ]; then
    warn "Checksum verification failed for $2"
    warn "  Expected: ${expected}"
    warn "  Actual:   ${actual}"
    exit 1
  fi
}

# install_binary <source_path> <install_dir>
install_binary() {
  mkdir -p "$2"
  mv "$1" "$2/${BINARY_NAME}"
  chmod +x "$2/${BINARY_NAME}"
}

main() {
  platform="$(get_platform_name)"
  machine="$(get_machine_name)"

  os="$(resolve_os "$platform")" || die "Unsupported OS: ${platform}"
  arch="$(resolve_arch "$machine")" || die "Unsupported architecture: ${machine}"

  system_writable=no
  [ -w "$SYSTEM_INSTALL_DIR" ] && system_writable=yes
  install_dir="$(resolve_install_dir "${INSTALL_DIR:-}" "$system_writable" "${HOME:-}")"

  release_tag="${VERSION:-}"
  if [ -z "$release_tag" ]; then
    if ! release_tag="$(fetch_latest_version)"; then
      warn "Failed to determine latest version (GitHub API unreachable or rate-limited)."
      die "Set GITHUB_TOKEN, run 'gh auth login', or set VERSION to install a specific release."
    fi
  fi

  archive_name="$(resolve_archive_name "$os" "$arch")"
  archive_url="${DOWNLOAD_BASE_URL}/${release_tag}/${archive_name}"
  checksums_url="${DOWNLOAD_BASE_URL}/${release_tag}/${CHECKSUMS_NAME}"

  log "Installing ${CLI_NAME} ${release_tag} (${os}/${arch})..."

  # Not local: the EXIT trap expands TMP_DIR after main has returned.
  TMP_DIR="$(mktemp -d)"
  trap 'rm -rf "$TMP_DIR"' EXIT INT TERM

  archive_path="${TMP_DIR}/${archive_name}"
  checksums_path="${TMP_DIR}/${CHECKSUMS_NAME}"
  extract_dir="${TMP_DIR}/extract"

  download_file "$archive_url" "$archive_path"
  download_file "$checksums_url" "$checksums_path"
  verify_checksum "$archive_path" "$archive_name" "$checksums_path"

  mkdir -p "$extract_dir"
  tar -xzf "$archive_path" -C "$extract_dir"

  source_binary="${extract_dir}/${BINARY_NAME}"
  [ -f "$source_binary" ] || die "Archive did not contain ${BINARY_NAME}"

  install_binary "$source_binary" "$install_dir"
  log "Installed ${CLI_NAME} to ${install_dir}/${BINARY_NAME}"

  path_contains_dir "$install_dir" "${PATH:-}" || format_path_instructions "$install_dir"

  log ""
  log "Run: ${CLI_NAME} --version"
}

main "$@"
