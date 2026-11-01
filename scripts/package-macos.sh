#!/usr/bin/env bash
set -euo pipefail

# wgit macOS packaging.
#
# Single source of truth for the desktop artifact: CI calls this script and so
# does a local release, so a green pipeline means the shipped bundle really
# builds. Order matters: build -> bundle -> sign/notarize/staple the .app ->
# create the DMG from the stapled .app -> sign/notarize/staple the DMG.
#
# Environment:
#   ARCHS            space separated targets, default "arm64 amd64" (universal)
#   VERSION          release version; defaults to the tag or config.yml + sha
#   OUT_DIR          artifact directory, default "dist"
#   SKIP_FRONTEND    set to 1 to reuse an existing frontend/dist
#   APPLE_SIGNING_IDENTITY / notary credentials -> see sign-and-notarize.sh
#   NOTARIZE         auto (default) | true | false

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
cd "${REPO_ROOT}"

APP_NAME="wgit"
BUNDLE_ID="com.wgit.app"
BIN_DIR="bin"
OUT_DIR="${OUT_DIR:-dist}"
ARCHS="${ARCHS:-arm64 amd64}"
DEPLOYMENT_TARGET="12.0"
ENTITLEMENTS="${ENTITLEMENTS:-build/darwin/entitlements.plist}"

log() { echo "==> $*"; }
die() { echo "Error: $*" >&2; exit 1; }

for tool in go hdiutil plutil; do
  command -v "${tool}" >/dev/null 2>&1 || die "${tool} is required (install Xcode command line tools)"
done

COMMIT_HASH="$(git rev-parse --short HEAD 2>/dev/null || echo unknown)"
BUILD_DATE="${BUILD_DATE:-$(date -u +"%Y-%m-%dT%H:%M:%SZ")}"
BASE_VERSION="$(awk '/^  version:/ { gsub(/[\"'"'"']/, "", $2); print $2; exit }' build/config.yml)"
BASE_VERSION="${BASE_VERSION:-0.0.0}"
if [ -z "${VERSION:-}" ]; then
  if VERSION_TAG="$(git describe --tags --exact-match HEAD 2>/dev/null)"; then
    VERSION="${VERSION_TAG#v}"
  else
    VERSION="${BASE_VERSION}-dev.${COMMIT_HASH}"
  fi
fi

ARCH_LIST="$(echo "${ARCHS}" | tr -s ' ' | sed -e 's/^ *//' -e 's/ *$//')"
[ -n "${ARCH_LIST}" ] || die "ARCHS is empty"
for arch in ${ARCH_LIST}; do
  case "${arch}" in arm64 | amd64) ;; *) die "unsupported ARCHS entry '${arch}' (use arm64 and/or amd64)" ;; esac
done
case "${ARCH_LIST}" in
  *" "*) UNIARCH=true; ARCH_LABEL="universal" ;;
  *) UNIARCH=false; ARCH_LABEL="${ARCH_LIST}" ;;
esac
DMG_NAME="${APP_NAME}-${VERSION}-darwin-${ARCH_LABEL}.dmg"
DMG_PATH="${OUT_DIR}/${DMG_NAME}"
APP_DIR="${BIN_DIR}/${APP_NAME}.app"

log "Packaging ${APP_NAME} ${VERSION} (${ARCH_LABEL})"
log "Commit ${COMMIT_HASH}  built ${BUILD_DATE}"

if [ "${SKIP_FRONTEND:-0}" != "1" ]; then
  log "Building the frontend bundle"
  (cd frontend && bun run build)
fi
[ -n "$(ls -A frontend/dist 2>/dev/null)" ] || die "frontend/dist is empty; run 'bun run build' in frontend/ or unset SKIP_FRONTEND"

mkdir -p "${BIN_DIR}" "${OUT_DIR}"
LDFLAGS="-s -w -X main.AppVersion=${VERSION} -X main.CommitHash=${COMMIT_HASH} -X main.BuildDate=${BUILD_DATE}"

for arch in ${ARCH_LIST}; do
  log "Compiling darwin/${arch} (-tags production embeds frontend/dist)"
  # CGO is required by the WebKit shell; MACOSX_DEPLOYMENT_TARGET keeps the
  # minimum OS identical to LSMinimumSystemVersion in Info.plist.
  CGO_ENABLED=1 GOOS=darwin GOARCH="${arch}" \
    CGO_CFLAGS="-mmacosx-version-min=${DEPLOYMENT_TARGET}" \
    CGO_LDFLAGS="-mmacosx-version-min=${DEPLOYMENT_TARGET}" \
    MACOSX_DEPLOYMENT_TARGET="${DEPLOYMENT_TARGET}" \
    go build -tags production -trimpath -buildvcs=false -ldflags "${LDFLAGS}" \
      -o "${BIN_DIR}/${APP_NAME}-${arch}" .
done

if [ "${UNIARCH}" = true ]; then
  log "Creating the universal binary (${ARCH_LIST})"
  lipo -create -output "${BIN_DIR}/${APP_NAME}" ${BIN_DIR}/${APP_NAME}-amd64 ${BIN_DIR}/${APP_NAME}-arm64
  rm -f ${BIN_DIR}/${APP_NAME}-amd64 ${BIN_DIR}/${APP_NAME}-arm64
else
  mv "${BIN_DIR}/${APP_NAME}-${ARCH_LIST}" "${BIN_DIR}/${APP_NAME}"
fi

log "Assembling ${APP_DIR}"
rm -rf "${APP_DIR}"
mkdir -p "${APP_DIR}/Contents/MacOS" "${APP_DIR}/Contents/Resources"
cp "${BIN_DIR}/${APP_NAME}" "${APP_DIR}/Contents/MacOS/${APP_NAME}"
chmod +x "${APP_DIR}/Contents/MacOS/${APP_NAME}"
cp build/darwin/Info.plist "${APP_DIR}/Contents/Info.plist"
cp build/darwin/icons.icns "${APP_DIR}/Contents/Resources/icons.icns"
if [ -f build/darwin/Assets.car ]; then
  cp build/darwin/Assets.car "${APP_DIR}/Contents/Resources/Assets.car"
fi

# Info.plist is the version the Finder and Gatekeeper show, so it has to carry
# the same string the binary was stamped with.
/usr/libexec/PlistBuddy -c "Set :CFBundleShortVersionString ${VERSION}" "${APP_DIR}/Contents/Info.plist"
/usr/libexec/PlistBuddy -c "Set :CFBundleVersion ${COMMIT_HASH}" "${APP_DIR}/Contents/Info.plist"
plutil -lint "${APP_DIR}/Contents/Info.plist" >/dev/null

VERSION_JSON="${APP_DIR}/Contents/Resources/build-info.json"
printf '{\n  "version": "%s",\n  "commit": "%s",\n  "buildDate": "%s",\n  "arch": "%s",\n  "bundleId": "%s"\n}\n' \
  "${VERSION}" "${COMMIT_HASH}" "${BUILD_DATE}" "${ARCH_LABEL}" "${BUNDLE_ID}" > "${VERSION_JSON}"

log "Signing${APPLE_SIGNING_IDENTITY:+ (Developer ID)} and notarizing the app bundle"
"${SCRIPT_DIR}/sign-and-notarize.sh" "${APP_DIR}"

STAGING="$(mktemp -d "${TMPDIR:-/tmp}/wgit-dmg-staging.XXXXXX")"
trap 'rm -rf "${STAGING}"' EXIT

log "Creating ${DMG_PATH} from the signed bundle"
cp -R "${APP_DIR}" "${STAGING}/${APP_NAME}.app"
ln -s /Applications "${STAGING}/Applications"
rm -f "${DMG_PATH}"

if [ -x ".venv/bin/python" ] && ".venv/bin/python" -c "import dmgbuild" 2>/dev/null; then
  # dmgbuild pins the icon coordinates and the retina background that the
  # Taskfile's styled DMG uses.
  ".venv/bin/python" build/darwin/scripts/build_dmg.py \
    --app "${APP_DIR}" \
    --bg build/darwin/dmg-background@2x.png \
    --name "${APP_NAME}" \
    --output "${DMG_PATH}" \
    --win-w 540 --win-h 380 \
    --app-x 150 --app-y 175 --apps-x 390 --apps-y 175 \
    --icon-size 100 --text-size 12
else
  echo "    dmgbuild unavailable (python3 -m venv .venv && .venv/bin/pip install dmgbuild); using hdiutil"
  hdiutil create -volname "${APP_NAME}" -srcfolder "${STAGING}" -ov -format UDZO "${DMG_PATH}"
fi

# Checksum the image before signing it: codesign appends to the file afterwards.
hdiutil verify "${DMG_PATH}" >/dev/null

log "Signing and notarizing the disk image"
"${SCRIPT_DIR}/sign-and-notarize.sh" "${DMG_PATH}"

(
  cd "${OUT_DIR}"
  # Truncate: a local rebuild in the same OUT_DIR must not keep checksums for
  # a DMG that has since been replaced.
  shasum -a 256 "${DMG_NAME}" | tee sha256sums.txt
)

log "Done"
ls -lh "${DMG_PATH}"
echo "    SHA-256: $(shasum -a 256 "${DMG_PATH}" | cut -d' ' -f1)"
codesign -dvv "${APP_DIR}" 2>&1 | grep -E "^Identifier|^TeamIdentifier" || true
xcrun stapler validate "${DMG_PATH}" 2>/dev/null \
  && echo "    Notarization ticket is stapled to the DMG" \
  || echo "    WARNING: no stapled notarization ticket - the DMG is not distributable"
