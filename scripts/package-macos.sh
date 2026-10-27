#!/usr/bin/env bash
set -euo pipefail

# wgit macOS Packaging Script (P0.2)
# Generates production DMG with version metadata injection and Applications symlink.

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

cd "${REPO_ROOT}"

VERSION="${VERSION:-$(git describe --tags --always --dirty 2>/dev/null || echo "1.0.0")}"
COMMIT_HASH="${COMMIT_HASH:-$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")}"
BUILD_DATE="${BUILD_DATE:-$(date -u +"%Y-%m-%dT%H:%M:%SZ")}"

echo "=================================================="
echo " Packaging wgit for macOS"
echo " Version:     ${VERSION}"
echo " Commit:      ${COMMIT_HASH}"
echo " Build Date:  ${BUILD_DATE}"
echo "=================================================="

# Ensure frontend is built
echo "==> Building frontend assets..."
(cd frontend && bun run build)

# Compile backend binary with ldflags injection
echo "==> Compiling backend binary with ldflags..."
mkdir -p bin
LDFLAGS="-s -w -X main.AppVersion=${VERSION} -X main.CommitHash=${COMMIT_HASH} -X main.BuildDate=${BUILD_DATE}"
go build -ldflags "${LDFLAGS}" -trimpath -o bin/wgit .

# Bundle .app structure if not using full wails3 build
APP_DIR="bin/wgit.app"
mkdir -p "${APP_DIR}/Contents/MacOS"
mkdir -p "${APP_DIR}/Contents/Resources"

cp bin/wgit "${APP_DIR}/Contents/MacOS/wgit"
chmod +x "${APP_DIR}/Contents/MacOS/wgit"

if [ -f "build/darwin/Info.plist" ]; then
  cp build/darwin/Info.plist "${APP_DIR}/Contents/Info.plist"
fi

if [ -f "build/darwin/icons.icns" ]; then
  cp build/darwin/icons.icns "${APP_DIR}/Contents/Resources/icons.icns"
fi

echo "==> App bundle prepared at ${APP_DIR}"

# Create DMG
DMG_DIR="dist"
mkdir -p "${DMG_DIR}"
DMG_NAME="wgit-${VERSION}-darwin-$(uname -m).dmg"
DMG_PATH="${DMG_DIR}/${DMG_NAME}"
TMP_DMG_STAGING="$(mktemp -d /tmp/wgit-dmg-staging.XXXXXX)"

trap 'rm -rf "${TMP_DMG_STAGING}"' EXIT

cp -R "${APP_DIR}" "${TMP_DMG_STAGING}/wgit.app"
ln -s /Applications "${TMP_DMG_STAGING}/Applications"

rm -f "${DMG_PATH}"

echo "==> Creating DMG file with hdiutil..."
hdiutil create \
  -volname "wgit" \
  -srcfolder "${TMP_DMG_STAGING}" \
  -ov \
  -format UDZO \
  "${DMG_PATH}"

echo "=================================================="
echo " Successfully created DMG package: ${DMG_PATH}"
echo " Size: $(du -h "${DMG_PATH}" | cut -f1)"
echo "=================================================="
