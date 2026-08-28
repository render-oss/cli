#!/bin/sh
# This script installs the latest version of the Render CLI
# You can run it directly:
#   curl -fsSL https://raw.githubusercontent.com/render-oss/cli/bin/install.sh | sh

set -e

# Prevent running with partial download
{ # this ensures the entire script is downloaded

  # Function to get latest release info using GitHub API
  get_latest_release() {
    curl --silent "https://api.github.com/repos/render-oss/cli/releases/latest" \
      | sed -n 's/.*"tag_name": "\([^"]*\)".*/\1/p'
  }

  # Function to output error message and exit
  error() {
    echo "Error: $1" >&2
    exit 1
  }

  # Return whether this is a sudo install running as root for another user.
  is_sudo_install() {
    [ "$(id -u)" -eq 0 ] && [ -n "${SUDO_USER:-}" ] && [ "${SUDO_USER:-}" != "root" ]
  }

  # Make text bold and Render purple when stdout is a terminal. Piped or logged
  # output stays plain text.
  bold_render_purple() {
    # File descriptor 1 is stdout; -t reports whether it is connected to a
    # terminal. Without one, %s inserts the first argument unchanged and printf
    # adds no newline.
    if [ ! -t 1 ]; then
      printf '%s' "$1"
      return
    fi

    # In the formats below, \033[ starts an ANSI style sequence, semicolons
    # separate its parameters, m applies them, and \033[0m resets the style.
    case "${COLORTERM:-}" in
      truecolor | 24bit)
        # Bold Render purple clears the contrast threshold against both a white
        # and a near-black terminal: 1 means bold, 38;2 selects a 24-bit
        # foreground color, and 155;82;251 is Render purple.
        printf '\033[1;38;2;155;82;251m%s\033[0m' "$1"
        ;;
      *)
        # No truecolor: fall back to the terminal's own magenta, which the
        # user's theme has already made legible against their background. Here,
        # 1 means bold and 35 selects magenta.
        printf '\033[1;35m%s\033[0m' "$1"
        ;;
    esac
  }

  # Check for required commands
  command -v curl > /dev/null 2>&1 || error "curl is required but not installed"
  command -v sed > /dev/null 2>&1 || error "sed is required but not installed"
  command -v unzip > /dev/null 2>&1 || error "unzip is required but not installed"

  # Detect OS
  OS="$(uname -s)"
  case "${OS}" in
    Linux*) OS_NAME=linux ;;
    Darwin*) OS_NAME=darwin ;;
    *) error "Unsupported operating system: ${OS}" ;;
  esac

  # Detect architecture
  ARCH="$(uname -m)"
  case "${ARCH}" in
    x86_64*) ARCH_NAME=amd64 ;;
    arm64*) ARCH_NAME=arm64 ;;
    aarch64*) ARCH_NAME=arm64 ;;
    *) error "Unsupported architecture: ${ARCH}" ;;
  esac

  # Get the latest release version
  VERSION=$(get_latest_release)
  if [ -z "$VERSION" ]; then
    error "Failed to get latest release version"
  fi

  # Remove 'v' prefix from version if present
  VERSION_NUM="${VERSION#v}"

  echo "Installing Render CLI version ${VERSION}..."

  # Construct download URL
  BINARY_NAME="cli_${VERSION_NUM}_${OS_NAME}_${ARCH_NAME}.zip"
  DOWNLOAD_URL="https://github.com/render-oss/cli/releases/download/${VERSION}/${BINARY_NAME}"

  # Create temporary directory
  TMP_DIR=$(mktemp -d)
  trap 'rm -rf "$TMP_DIR"' EXIT

  # Download and install
  echo "Downloading from ${DOWNLOAD_URL}..."
  curl -fsSL "$DOWNLOAD_URL" -o "${TMP_DIR}/${BINARY_NAME}"

  # Determine install location
  if [ "$(id -u)" -eq 0 ]; then
    INSTALL_DIR="/usr/local/bin"
  else
    INSTALL_DIR="$HOME/.local/bin"
    mkdir -p "$INSTALL_DIR"
  fi

  # Unzip in temporary directory
  unzip -o "${TMP_DIR}/${BINARY_NAME}" -d "${TMP_DIR}" > /dev/null 2>&1

  # Find and move the binary
  RENDER_BINARY=$(find "${TMP_DIR}" -type f -name "cli_v*" | head -n 1)
  if [ -z "$RENDER_BINARY" ]; then
    error "Could not find CLI binary in the archive"
  fi

  mv "${RENDER_BINARY}" "${INSTALL_DIR}/render"
  chmod +x "${INSTALL_DIR}/render"

  # Verify installation by checking the binary directly
  if [ -x "${INSTALL_DIR}/render" ]; then
    echo "✨ Successfully installed Render CLI to ${INSTALL_DIR}/render"
    echo
    # If the installer is running under sudo, defer the notice until the invoking
    # user's first CLI command so its state is not created as root.
    if ! is_sudo_install; then
      # Tell the user what the CLI collects. Best-effort: this must never turn a
      # successful install into a failed one, and the CLI shows the notice on
      # the first command anyway if this does not run. Keep stderr attached to
      # the terminal because the notice does not display otherwise.
      "${INSTALL_DIR}/render" analytics notice || true
    fi
    if ! command -v render > /dev/null 2>&1; then
      echo "NOTE: Make sure ${INSTALL_DIR} is in your PATH by adding this to your shell's rc file:"
      bold_render_purple "  export PATH=\$PATH:${INSTALL_DIR}"
      echo
      echo
      echo "To use render CLI immediately, run:"
      bold_render_purple "  export PATH=\$PATH:${INSTALL_DIR}"
      echo
      echo "  ${INSTALL_DIR}/render --version"
    else
      "${INSTALL_DIR}/render" --version
    fi
  else
    error "Installation failed: Could not install binary to ${INSTALL_DIR}/render"
  fi

}
