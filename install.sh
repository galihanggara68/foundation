#!/bin/sh
set -e

# Foundation CLI installer
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/galihanggara68/foundation/main/install.sh | sh
# Options (via environment variables):
#   FOUNDATION_VERSION: release tag to install (defaults to latest)
#   FOUNDATION_INSTALL_DIR: directory to install binary into (defaults to /usr/local/bin or ~/.local/bin)

REPO="galihanggara68/foundation"

echo "Installing Foundation CLI..."

# Detect OS
OS="$(uname -s)"
case "${OS}" in
  Linux*)  OS="linux" ;;
  Darwin*) OS="darwin" ;;
  CYGWIN*|MINGW*|MSYS*) OS="windows" ;;
  *)
    echo "Error: Unsupported operating system: ${OS}" >&2
    exit 1
    ;;
esac

# Detect Architecture
ARCH="$(uname -m)"
case "${ARCH}" in
  x86_64|amd64) ARCH="amd64" ;;
  arm64|aarch64) ARCH="arm64" ;;
  *)
    echo "Error: Unsupported CPU architecture: ${ARCH}" >&2
    exit 1
    ;;
esac

# Determine version
TAG="${FOUNDATION_VERSION:-}"
if [ -z "${TAG}" ]; then
  # Fetch latest release tag from GitHub API
  if command -v curl >/dev/null 2>&1; then
    LATEST_JSON="$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" 2>/dev/null || true)"
  elif command -v wget >/dev/null 2>&1; then
    LATEST_JSON="$(wget -qO- "https://api.github.com/repos/${REPO}/releases/latest" 2>/dev/null || true)"
  else
    echo "Error: Neither curl nor wget is available." >&2
    exit 1
  fi

  TAG="$(echo "${LATEST_JSON}" | grep '"tag_name":' | head -n 1 | sed -E 's/.*"tag_name":[[:space:]]*"([^"]+)".*/\1/' || true)"
  if [ -z "${TAG}" ]; then
    # Fallback default release
    TAG="v0.7.0"
  fi
fi

echo "Selected version: ${TAG} (${OS}/${ARCH})"

EXT="tar.gz"
BIN_NAME="foundation"
if [ "${OS}" = "windows" ]; then
  EXT="zip"
  BIN_NAME="foundation.exe"
fi

ASSET_NAME="foundation-${TAG}-${OS}-${ARCH}.${EXT}"
DOWNLOAD_URL="https://github.com/${REPO}/releases/download/${TAG}/${ASSET_NAME}"

# Determine installation directory
INSTALL_DIR="${FOUNDATION_INSTALL_DIR:-}"
if [ -z "${INSTALL_DIR}" ]; then
  if [ -w "/usr/local/bin" ]; then
    INSTALL_DIR="/usr/local/bin"
  elif [ "$(id -u 2>/dev/null || echo 1)" -eq 0 ]; then
    INSTALL_DIR="/usr/local/bin"
  else
    INSTALL_DIR="${HOME}/.local/bin"
  fi
fi

mkdir -p "${INSTALL_DIR}"

TMP_DIR="$(mktemp -d 2>/dev/null || mktemp -d -t 'foundation')"
trap 'rm -rf "${TMP_DIR}"' EXIT

echo "Downloading ${DOWNLOAD_URL}..."
if command -v curl >/dev/null 2>&1; then
  curl -fsSL "${DOWNLOAD_URL}" -o "${TMP_DIR}/${ASSET_NAME}"
elif command -v wget >/dev/null 2>&1; then
  wget -qO "${TMP_DIR}/${ASSET_NAME}" "${DOWNLOAD_URL}"
fi

echo "Extracting binary..."
if [ "${EXT}" = "zip" ]; then
  unzip -q -o "${TMP_DIR}/${ASSET_NAME}" -d "${TMP_DIR}"
else
  tar -xzf "${TMP_DIR}/${ASSET_NAME}" -C "${TMP_DIR}"
fi

if [ ! -f "${TMP_DIR}/${BIN_NAME}" ]; then
  echo "Error: Binary '${BIN_NAME}' not found in downloaded archive." >&2
  exit 1
fi

chmod +x "${TMP_DIR}/${BIN_NAME}"
mv "${TMP_DIR}/${BIN_NAME}" "${INSTALL_DIR}/${BIN_NAME}"

echo "Foundation CLI ${TAG} installed successfully to ${INSTALL_DIR}/${BIN_NAME}!"

case ":${PATH}:" in
  *:"${INSTALL_DIR}":*) ;;
  *)
    echo ""
    echo "Notice: ${INSTALL_DIR} is not in your PATH."
    echo "Add it to your shell configuration (e.g., ~/.bashrc or ~/.zshrc):"
    echo "  export PATH=\"${INSTALL_DIR}:\$PATH\""
    ;;
esac

"${INSTALL_DIR}/${BIN_NAME}" version || true
