#!/bin/sh
# Installs django-filerouter. Usage:
#   curl -fsSL https://raw.githubusercontent.com/AlmostWorkingSystem/django-filerouter/main/install.sh | sh
#
# Env vars:
#   DJANGO_FILEROUTER_VERSION  a tag to install (default: latest release)
#   BINDIR                     install directory (default: /usr/local/bin)
set -eu

REPO="AlmostWorkingSystem/django-filerouter"
VERSION="${DJANGO_FILEROUTER_VERSION:-latest}"
BINDIR="${BINDIR:-/usr/local/bin}"

OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
case "$OS" in
    linux | darwin) ;;
    *)
        echo "django-filerouter: unsupported OS: $OS" >&2
        exit 1
        ;;
esac

ARCH="$(uname -m)"
case "$ARCH" in
    x86_64) ARCH=amd64 ;;
    aarch64 | arm64) ARCH=arm64 ;;
    *)
        echo "django-filerouter: unsupported architecture: $ARCH" >&2
        exit 1
        ;;
esac

if [ "$OS" = "darwin" ] && [ "$ARCH" = "amd64" ]; then
    echo "django-filerouter: no darwin/amd64 release is published (only darwin/arm64, linux/amd64, linux/arm64)" >&2
    exit 1
fi

ASSET="django-filerouter_${OS}_${ARCH}.tar.gz"
if [ "$VERSION" = "latest" ]; then
    URL="https://github.com/${REPO}/releases/latest/download/${ASSET}"
else
    URL="https://github.com/${REPO}/releases/download/${VERSION}/${ASSET}"
fi

TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

echo "django-filerouter: downloading ${ASSET} (${VERSION}) from ${REPO}..."
curl -fsSL "$URL" -o "${TMP_DIR}/django-filerouter.tar.gz"
tar -xzf "${TMP_DIR}/django-filerouter.tar.gz" -C "$TMP_DIR" django-filerouter
chmod +x "${TMP_DIR}/django-filerouter"

if [ -w "$BINDIR" ]; then
    mv "${TMP_DIR}/django-filerouter" "${BINDIR}/django-filerouter"
else
    echo "django-filerouter: ${BINDIR} isn't writable, retrying with sudo..."
    sudo mv "${TMP_DIR}/django-filerouter" "${BINDIR}/django-filerouter"
fi

echo "django-filerouter: installed $("${BINDIR}/django-filerouter" version) to ${BINDIR}/django-filerouter"
