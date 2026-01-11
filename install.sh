#!/bin/sh
set -e

# Orc installer script
# Usage: curl -fsSL https://raw.githubusercontent.com/randalmurphal/orc/main/install.sh | sh

REPO="randalmurphal/orc"
INSTALL_DIR="${INSTALL_DIR:-$HOME/.local/bin}"

# Detect OS and architecture
detect_platform() {
    OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
    ARCH="$(uname -m)"

    case "$ARCH" in
        x86_64|amd64) ARCH="amd64" ;;
        aarch64|arm64) ARCH="arm64" ;;
        *) echo "Unsupported architecture: $ARCH"; exit 1 ;;
    esac

    case "$OS" in
        linux) OS="linux" ;;
        darwin) OS="darwin" ;;
        *) echo "Unsupported OS: $OS"; exit 1 ;;
    esac

    PLATFORM="${OS}-${ARCH}"
}

# Get latest version from GitHub
get_latest_version() {
    if command -v curl >/dev/null 2>&1; then
        VERSION=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | sed -E 's/.*"([^"]+)".*/\1/')
    elif command -v wget >/dev/null 2>&1; then
        VERSION=$(wget -qO- "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | sed -E 's/.*"([^"]+)".*/\1/')
    else
        echo "Error: curl or wget required"
        exit 1
    fi

    if [ -z "$VERSION" ]; then
        echo "Error: Could not determine latest version"
        exit 1
    fi
}

# Download and install
install() {
    FILENAME="orc-${VERSION}-${PLATFORM}.tar.gz"
    URL="https://github.com/${REPO}/releases/download/${VERSION}/${FILENAME}"

    echo "Installing orc ${VERSION} for ${PLATFORM}..."

    # Create install directory
    mkdir -p "$INSTALL_DIR"

    # Download and extract
    TMP_DIR=$(mktemp -d)
    trap 'rm -rf "$TMP_DIR"' EXIT

    echo "Downloading ${URL}..."
    if command -v curl >/dev/null 2>&1; then
        curl -fsSL "$URL" -o "$TMP_DIR/$FILENAME"
    else
        wget -q "$URL" -O "$TMP_DIR/$FILENAME"
    fi

    echo "Extracting..."
    tar -xzf "$TMP_DIR/$FILENAME" -C "$TMP_DIR"

    # Install binary
    mv "$TMP_DIR/orc-${VERSION}-${PLATFORM}" "$INSTALL_DIR/orc"
    chmod +x "$INSTALL_DIR/orc"

    echo ""
    echo "orc ${VERSION} installed to ${INSTALL_DIR}/orc"

    # Check if in PATH
    if ! echo "$PATH" | grep -q "$INSTALL_DIR"; then
        echo ""
        echo "Add to your PATH:"
        echo "  export PATH=\"\$PATH:${INSTALL_DIR}\""
        echo ""
        echo "Add this to your ~/.bashrc, ~/.zshrc, or shell config."
    fi

    echo ""
    echo "Run 'orc --help' to get started."
}

main() {
    detect_platform
    get_latest_version
    install
}

main
