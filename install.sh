#!/usr/bin/env bash
#
# Install tm (Tracemesh) from pre-built binaries — no Go toolchain required.
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/Varun136/trace-mesh-cli/master/install.sh | bash
#   curl -fsSL https://raw.githubusercontent.com/Varun136/trace-mesh-cli/master/install.sh | bash -s -- --version v0.1.0
#   curl -fsSL https://raw.githubusercontent.com/Varun136/trace-mesh-cli/master/install.sh | bash -s -- --prefix ~/.local/bin
#
# Options:
#   --version VERSION   Install a specific release tag (e.g. v0.1.0). Default: latest.
#   --prefix DIR        Install directory. Default: /usr/local/bin, or
#                       ~/.local/bin when /usr/local/bin is not writable.
#   -h, --help          Show this help.
#
# Environment overrides: TM_VERSION, TM_INSTALL_DIR.
set -euo pipefail

REPO="Varun136/trace-mesh-cli"
BIN="tm"
PROJECT="tracemesh"

VERSION="${TM_VERSION:-}"
PREFIX="${TM_INSTALL_DIR:-}"

usage() {
  awk 'NR == 1 {next} /^set -euo pipefail$/ {exit} {sub(/^# ?/, ""); print}' "$0"
}

log()  { printf '%s\n' "$*" >&2; }
fail() { log "install.sh: error: $*"; exit 1; }

while [ $# -gt 0 ]; do
  case "$1" in
    --version) VERSION="${2:?--version requires a value}"; shift 2 ;;
    --version=*) VERSION="${1#--version=}"; shift ;;
    --prefix|--dir) PREFIX="${2:?--prefix requires a value}"; shift 2 ;;
    --prefix=*|--dir=*) PREFIX="${1#*=}"; shift ;;
    -h|--help) usage; exit 0 ;;
    -*) fail "unknown option: $1 (see --help)" ;;
    *)  [ -z "$VERSION" ] || fail "unexpected argument: $1 (see --help)"
        VERSION="$1"; shift ;;
  esac
done

# Normalize "1.2.3" to "v1.2.3"; release tags are v-prefixed.
if [ -n "$VERSION" ] && [ "$VERSION" != "latest" ]; then
  case "$VERSION" in
    v*) ;;
    *) VERSION="v$VERSION" ;;
  esac
fi

detect_platform() {
  local os arch
  os="$(uname -s)"
  case "$os" in
    Linux)  OS="Linux" ;;
    Darwin) OS="Darwin" ;;
    MINGW*|MSYS*|CYGWIN*|Windows_NT) OS="Windows" ;;
    *) fail "unsupported operating system: $os" ;;
  esac
  arch="$(uname -m)"
  case "$arch" in
    x86_64|amd64) ARCH="x86_64" ;;
    arm64|aarch64) ARCH="arm64" ;;
    armv7l|armv6l|arm) ARCH="arm" ;;
    i386|i686) ARCH="i386" ;;
    *) fail "unsupported architecture: $arch" ;;
  esac
  if [ "$OS" = "Windows" ]; then
    EXT="zip"
    BIN_NAME="tm.exe"
  else
    EXT="tar.gz"
    BIN_NAME="tm"
  fi
  ASSET="${PROJECT}_${OS}_${ARCH}.${EXT}"
}

resolve_base_url() {
  # TM_BASE_URL overrides the download location (used for local testing and
  # mirrors); otherwise releases are fetched from GitHub.
  if [ -n "${TM_BASE_URL:-}" ]; then
    BASE_URL="$TM_BASE_URL"
    LABEL="${VERSION:-local}"
    return
  fi
  if [ -z "$VERSION" ] || [ "$VERSION" = "latest" ]; then
    BASE_URL="https://github.com/${REPO}/releases/latest/download"
    LABEL="latest"
  else
    BASE_URL="https://github.com/${REPO}/releases/download/${VERSION}"
    LABEL="$VERSION"
  fi
}

download() {
  local url="$1" dest="$2"
  if command -v curl >/dev/null 2>&1; then
    curl -fsSL --retry 3 -o "$dest" "$url" || return 1
  elif command -v wget >/dev/null 2>&1; then
    wget -qO "$dest" "$url" || return 1
  else
    fail "neither curl nor wget is installed"
  fi
}

sha256_of() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | awk '{print $1}'
  elif command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$1" | awk '{print $1}'
  else
    fail "neither sha256sum nor shasum is installed"
  fi
}

verify_checksum() {
  local archive="$1" sums="$2" expected actual
  # checksums.txt lines look like "<sha256>  <filename>".
  expected="$(grep -F "  ${ASSET}" "$sums" | awk '{print $1}' | head -n 1)"
  [ -n "$expected" ] || fail "no checksum entry for ${ASSET} in checksums.txt"
  actual="$(sha256_of "$archive")"
  [ "$expected" = "$actual" ] || fail "checksum mismatch for ${ASSET} (download may be corrupt)"
  log "Checksum verified."
}

extract() {
  local archive="$1" dest="$2"
  if [ "$EXT" = "zip" ]; then
    command -v unzip >/dev/null 2>&1 || fail "unzip is required to install the Windows archive"
    unzip -oq "$archive" -d "$dest"
  else
    tar -xzf "$archive" -C "$dest"
  fi
}

choose_prefix() {
  if [ -n "$PREFIX" ]; then
    return
  fi
  if [ "$OS" = "Windows" ]; then
    PREFIX="$HOME/bin"
  elif [ -w "/usr/local/bin" ] || [ "$(id -u)" = "0" ]; then
    PREFIX="/usr/local/bin"
  else
    PREFIX="$HOME/.local/bin"
  fi
}

main() {
  detect_platform
  resolve_base_url
  choose_prefix
  log "Installing tm (${LABEL}) for ${OS}/${ARCH} into ${PREFIX} ..."

  TMP_DIR="$(mktemp -d)"
  trap 'rm -rf "$TMP_DIR"' EXIT INT TERM

  download "${BASE_URL}/${ASSET}" "${TMP_DIR}/${ASSET}" \
    || fail "could not download ${BASE_URL}/${ASSET} — check that release ${LABEL} has a ${OS}/${ARCH} asset"
  download "${BASE_URL}/checksums.txt" "${TMP_DIR}/checksums.txt" \
    || fail "could not download ${BASE_URL}/checksums.txt"
  verify_checksum "${TMP_DIR}/${ASSET}" "${TMP_DIR}/checksums.txt"
  extract "${TMP_DIR}/${ASSET}" "$TMP_DIR"

  [ -f "${TMP_DIR}/${BIN_NAME}" ] || fail "archive did not contain ${BIN_NAME}"
  mkdir -p "$PREFIX"
  cp -f "${TMP_DIR}/${BIN_NAME}" "${PREFIX}/${BIN_NAME}"
  chmod +x "${PREFIX}/${BIN_NAME}"

  if ! "${PREFIX}/${BIN_NAME}" --version >/dev/null 2>&1; then
    log "Warning: installed binary did not respond to '${BIN} --version'."
  fi

  log "tm installed successfully at ${PREFIX}/${BIN_NAME}"
  case ":$PATH:" in
    *":${PREFIX}:"*) ;;
    *) log "Note: ${PREFIX} is not on your PATH. Add it, e.g.: export PATH=\"${PREFIX}:\$PATH\"" ;;
  esac
}

main
