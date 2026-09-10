#!/bin/sh
# Installs enigma-cli. Usage:
#   curl -fsSL https://raw.githubusercontent.com/AlmostWorkingSystem/enigma-cli/main/install.sh | sh
#
# Env vars:
#   ENIGMA_CLI_VERSION  a tag to install (default: latest release)
#   BINDIR              install directory (default: /usr/local/bin)
set -eu

REPO="AlmostWorkingSystem/enigma-cli"
VERSION="${ENIGMA_CLI_VERSION:-latest}"
BINDIR="${BINDIR:-/usr/local/bin}"

OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
case "$OS" in
    linux | darwin) ;;
    *)
        echo "enigma-cli: unsupported OS: $OS" >&2
        exit 1
        ;;
esac

ARCH="$(uname -m)"
case "$ARCH" in
    x86_64) ARCH=amd64 ;;
    aarch64 | arm64) ARCH=arm64 ;;
    *)
        echo "enigma-cli: unsupported architecture: $ARCH" >&2
        exit 1
        ;;
esac

if [ "$OS" = "darwin" ] && [ "$ARCH" = "amd64" ]; then
    echo "enigma-cli: no darwin/amd64 release is published (only darwin/arm64, linux/amd64, linux/arm64)" >&2
    exit 1
fi

ASSET="enigma-cli_${OS}_${ARCH}.tar.gz"
if [ "$VERSION" = "latest" ]; then
    URL="https://github.com/${REPO}/releases/latest/download/${ASSET}"
else
    URL="https://github.com/${REPO}/releases/download/${VERSION}/${ASSET}"
fi

TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

echo "enigma-cli: downloading ${ASSET} (${VERSION}) from ${REPO}..."
curl -fsSL "$URL" -o "${TMP_DIR}/enigma-cli.tar.gz"
tar -xzf "${TMP_DIR}/enigma-cli.tar.gz" -C "$TMP_DIR" enigma-cli
chmod +x "${TMP_DIR}/enigma-cli"

if [ -w "$BINDIR" ]; then
    mv "${TMP_DIR}/enigma-cli" "${BINDIR}/enigma-cli"
else
    echo "enigma-cli: ${BINDIR} isn't writable, retrying with sudo..."
    sudo mv "${TMP_DIR}/enigma-cli" "${BINDIR}/enigma-cli"
fi

echo "enigma-cli: installed $("${BINDIR}/enigma-cli" version) to ${BINDIR}/enigma-cli"
