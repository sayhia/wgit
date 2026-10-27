#!/usr/bin/env bash
set -euo pipefail

# wgit macOS Code Signing & Notarization Script (P0.3)
# Validates environment and signs/notarizes the application bundle.

APP_PATH="${1:-bin/wgit.app}"
IDENTITY="${APPLE_SIGNING_IDENTITY:-}"
NOTARY_PROFILE="${APPLE_NOTARY_PROFILE:-wgit-notary}"

echo "=================================================="
echo " wgit Code Signing & Notarization"
echo " Target App:   ${APP_PATH}"
echo "=================================================="

if [ ! -d "${APP_PATH}" ]; then
  echo "Error: Target application bundle not found at ${APP_PATH}" >&2
  exit 1
fi

if [ -z "${IDENTITY}" ]; then
  echo "Warning: APPLE_SIGNING_IDENTITY is not set."
  echo "Running in DRY-RUN verification mode..."
  echo ""
  echo "Validating Info.plist bundle integrity..."
  plutil -lint "${APP_PATH}/Contents/Info.plist"
  echo "Checking binary executable..."
  file "${APP_PATH}/Contents/MacOS/wgit"
  echo ""
  echo "[DRY-RUN SUCCESS] Bundle structure is valid for signing."
  echo "To sign for real, export APPLE_SIGNING_IDENTITY=\"Developer ID Application: Your Name (ID)\""
  exit 0
fi

echo "==> Signing app bundle with identity: ${IDENTITY}..."
codesign --deep --force --options runtime \
  --sign "${IDENTITY}" \
  "${APP_PATH}"

echo "==> Verifying signature..."
codesign --verify --deep --strict --verbose=2 "${APP_PATH}"
spctl -a -vvv -t install "${APP_PATH}" || true

ZIP_PATH="/tmp/wgit-notarize.zip"
echo "==> Creating zip archive for notary submission..."
ditto -c -k --keepParent "${APP_PATH}" "${ZIP_PATH}"

echo "==> Submitting to Apple Notary Service..."
xcrun notarytool submit "${ZIP_PATH}" --keychain-profile "${NOTARY_PROFILE}" --wait

echo "==> Stapling notarization ticket to app bundle..."
xcrun stapler staple "${APP_PATH}"

echo "==> Final gatekeeper validation..."
spctl -a -vvv -t install "${APP_PATH}"

rm -f "${ZIP_PATH}"
echo "=================================================="
echo " Successfully signed and notarized: ${APP_PATH}"
echo "=================================================="
