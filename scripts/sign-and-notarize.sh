#!/usr/bin/env bash
set -euo pipefail

# wgit code signing and notarization for a .app bundle or a .dmg.
#
# Called by package-macos.sh between bundling and DMG creation (and again for
# the DMG), and usable on its own for a rebuild of an existing artifact.
#
# Environment:
#   APPLE_SIGNING_IDENTITY   "Developer ID Application: Name (TEAMID)"
#   APPLE_CERT_P12_BASE64    optional .p12 imported into a throwaway keychain
#   APPLE_CERT_PASSWORD      password for the .p12 above
#   APPLE_KEYCHAIN_PASSWORD  password for that keychain (default: random)
#   APPLE_API_KEY_BASE64     App Store Connect .p8, preferred over a profile
#   APPLE_API_KEY_FILE       path to an existing .p8
#   APPLE_API_KEY_ID / APPLE_API_ISSUER   required with either .p8 form
#   APPLE_NOTARY_PROFILE     keychain profile for notarytool (macOS workstations)
#   ENTITLEMENTS             default build/darwin/entitlements.plist
#   NOTARIZE                 auto (default) | true | false

TARGET="${1:?usage: sign-and-notarize.sh <path/to/App.app|path/to/file.dmg>}"
ENTITLEMENTS="${ENTITLEMENTS:-build/darwin/entitlements.plist}"
IDENTITY="${APPLE_SIGNING_IDENTITY:-}"
NOTARIZE="${NOTARIZE:-auto}"

log() { echo "==> $*"; }
die() { echo "Error: $*" >&2; exit 1; }

[ -e "${TARGET}" ] || die "target not found: ${TARGET}"
case "${TARGET}" in
  *.app)
    TARGET_KIND=app
    [ -f "${ENTITLEMENTS}" ] || die "entitlements file missing: ${ENTITLEMENTS}"
    ;;
  *.dmg) TARGET_KIND=dmg ;;
  *) die "unsupported target type (expected .app or .dmg): ${TARGET}" ;;
esac

WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/wgit-sign.XXXXXX")"
SUBMIT_TARGET=""
KEYCHAIN=""
P8_FILE=""
EXISTING_KEYCHAINS=()
while IFS= read -r line; do
  [ -n "${line}" ] && EXISTING_KEYCHAINS+=("$(printf '%s' "${line}" | tr -d '\"')")
done < <(security list-keychains -d user 2>/dev/null || true)

cleanup() {
  [ -n "${KEYCHAIN}" ] && rm -f "${KEYCHAIN}" 2>/dev/null || true
  [ -n "${P8_FILE}" ] && rm -f "${P8_FILE}" 2>/dev/null || true
  # Put the user's own search list back; a stray temp keychain in it is rude.
  if [ ${#EXISTING_KEYCHAINS[@]} -gt 0 ]; then
    security list-keychains -d user -s "${EXISTING_KEYCHAINS[@]}" 2>/dev/null || true
  fi
  rm -rf "${WORK_DIR}"
}
trap cleanup EXIT

if [ -n "${IDENTITY}" ] && [ -n "${APPLE_CERT_P12_BASE64:-}" ]; then
  log "Importing the Developer ID certificate into a temporary keychain"
  KEYCHAIN="${WORK_DIR}/wgit-sign.keychain-db"
  KEYCHAIN_PASSWORD="${APPLE_KEYCHAIN_PASSWORD:-$(openssl rand -hex 16)}"
  security create-keychain -p "${KEYCHAIN_PASSWORD}" "${KEYCHAIN}"
  security set-keychain-settings -lut 21600 "${KEYCHAIN}"
  security unlock-keychain -p "${KEYCHAIN_PASSWORD}" "${KEYCHAIN}"
  echo "${APPLE_CERT_P12_BASE64}" | base64 --decode > "${WORK_DIR}/developer.p12"
  security import "${WORK_DIR}/developer.p12" -P "${APPLE_CERT_PASSWORD:-}" \
    -A -f pkcs12 -k "${KEYCHAIN}" >/dev/null
  # Without this, codesign asks for an interactive ACL unlock on a headless runner.
  security set-key-partition-list -S apple-tool:,apple:,codesign: -s -k "${KEYCHAIN_PASSWORD}" "${KEYCHAIN}" >/dev/null
  security list-keychains -d user -s "${KEYCHAIN}" "${EXISTING_KEYCHAINS[@]}"
fi

if [ -z "${IDENTITY}" ]; then
  # arm64 macOS refuses to exec an unsigned Mach-O, so an unsigned build is
  # ad-hoc signed: it runs on the build machine but is not distributable.
  if [ "${TARGET_KIND}" = app ]; then
    log "No APPLE_SIGNING_IDENTITY: ad-hoc signing (local testing only)"
    codesign --force --sign - --entitlements "${ENTITLEMENTS}" "${TARGET}"
  else
    log "No APPLE_SIGNING_IDENTITY: leaving ${TARGET} unsigned"
  fi
  echo "[UNSIGNED] Set APPLE_SIGNING_IDENTITY (and notary credentials) for a distributable artifact."
  exit 0
fi

TEAM_ID="${APPLE_TEAM_ID:-$(printf '%s' "${IDENTITY}" | sed -nE 's/.*\(([A-Z0-9]+)\)$/\1/p')}"

if [ "${TARGET_KIND}" = app ]; then
  log "Signing the inner executable, then the bundle (hardened runtime)"
  # Signing the bundle after its executable beats the deprecated --deep walk.
  codesign --force --timestamp --options runtime --entitlements "${ENTITLEMENTS}" \
    --sign "${IDENTITY}" "${TARGET}/Contents/MacOS"/*
  codesign --force --timestamp --options runtime --entitlements "${ENTITLEMENTS}" \
    --sign "${IDENTITY}" "${TARGET}"
  SUBMIT_TARGET="${WORK_DIR}/$(basename "${TARGET}" .app).zip"
  ditto -c -k --keepParent "${TARGET}" "${SUBMIT_TARGET}"
else
  log "Signing the disk image"
  codesign --force --timestamp --sign "${IDENTITY}" "${TARGET}"
  SUBMIT_TARGET="${TARGET}"
fi

log "Verifying the signature"
codesign --verify --strict --verbose=2 "${TARGET}"

if [ "${NOTARIZE}" = false ]; then
  echo "WARNING: NOTARIZE=false - Gatekeeper will block this artifact outside the build machine."
  exit 0
fi

if [ -n "${APPLE_API_KEY_BASE64:-}" ]; then
  P8_FILE="${WORK_DIR}/AuthKey.p8"
  echo "${APPLE_API_KEY_BASE64}" | base64 --decode > "${P8_FILE}"
  chmod 600 "${P8_FILE}"
fi
P8_FILE="${P8_FILE:-${APPLE_API_KEY_FILE:-}}"

NOTARY_ARGS=()
if [ -n "${P8_FILE}" ] && [ -n "${APPLE_API_KEY_ID:-}" ] && [ -n "${APPLE_API_ISSUER:-}" ]; then
  NOTARY_ARGS=(--key "${P8_FILE}" --key-id "${APPLE_API_KEY_ID}" --issuer "${APPLE_API_ISSUER}")
elif [ -n "${APPLE_NOTARY_PROFILE:-}" ]; then
  NOTARY_ARGS=(--keychain-profile "${APPLE_NOTARY_PROFILE}")
elif [ "${NOTARIZE}" = true ]; then
  die "NOTARIZE=true but no notary credentials (APPLE_API_KEY_BASE64 + APPLE_API_KEY_ID + APPLE_API_ISSUER, or APPLE_NOTARY_PROFILE)"
else
  echo "WARNING: signed but not notarized (no notary credentials found)."
  exit 0
fi

log "Submitting to Apple notary service"
RESULT_JSON="${WORK_DIR}/notary.json"
# --wait exits non-zero on rejection; the JSON still carries the id we need to
# print Apple's diagnostic log, so keep going and judge by the status field.
xcrun notarytool submit "${SUBMIT_TARGET}" "${NOTARY_ARGS[@]}" --wait --output-format json > "${RESULT_JSON}" || true
read -r STATUS ID < <(python3 -c '
import json, sys
try:
    doc = json.load(open(sys.argv[1]))
except Exception:
    print("Unreadable unknown")
    sys.exit(0)
print(doc.get("status", "Unknown"), doc.get("id", ""))
' "${RESULT_JSON}")
if [ "${STATUS}" != "Accepted" ]; then
  echo "Notarization status: ${STATUS} (submission ${ID:-unknown})" >&2
  if [ -n "${ID:-}" ] && [ "${ID}" != "unknown" ]; then
    xcrun notarytool log "${ID}" "${NOTARY_ARGS[@]}" || true
  fi
  die "notarization failed for ${TARGET}"
fi

log "Stapling the notarization ticket"
xcrun stapler staple "${TARGET}"
xcrun stapler validate "${TARGET}"

log "Gatekeeper assessment"
if spctl -a -vvv -t install "${TARGET}"; then
  echo "    accepted"
else
  die "Gatekeeper rejected ${TARGET}"
fi

echo "=================================================="
echo " Signed and notarized: ${TARGET}"
echo " Team: ${TEAM_ID:-unknown}"
echo "=================================================="
