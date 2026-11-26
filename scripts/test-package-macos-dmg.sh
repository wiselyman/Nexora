#!/usr/bin/env bash
set -euo pipefail
if [[ $# -ne 2 ]]; then
  echo "usage: test-package-macos-dmg.sh VERSION DIST_DIR" >&2
  exit 2
fi
version="$1"
dist="$(cd "$2" && pwd)"
for arch in amd64 arm64; do
  image="$dist/Nexora-${version}-darwin-${arch}.dmg"
  [[ -s "$image" ]] || { echo "missing DMG: $image" >&2; exit 1; }
  mount="$(mktemp -d "${TMPDIR:-/tmp}/nexora-image.XXXXXX")"
  device=""
  cleanup() {
    if [[ -n "$device" ]]; then diskutil unmount "$mount" >/dev/null 2>&1 || true; diskutil eject "$device" >/dev/null 2>&1 || true; fi
    rmdir "$mount" 2>/dev/null || true
  }
  trap cleanup EXIT
  output="$(diskutil image attach --readOnly --mountOptions nobrowse --mountPoint "$mount" "$image")"
  device="$(printf '%s\n' "$output" | awk '/^\/dev\/disk[0-9]+[[:space:]]/ {print $1; exit}')"
  [[ -n "$device" ]]
  test -x "$mount/Nexora.app/Contents/MacOS/Nexora"
  [[ -L "$mount/Applications" ]] || { echo "DMG is missing the Applications shortcut: $image" >&2; exit 1; }
  [[ "$(readlink "$mount/Applications")" == "/Applications" ]] || { echo "DMG Applications shortcut points to the wrong location: $image" >&2; exit 1; }
  architectures="$(lipo -archs "$mount/Nexora.app/Contents/MacOS/Nexora")"
  case "$arch:$architectures" in
    amd64:*x86_64*|arm64:*arm64*) ;;
    *) echo "wrong executable architecture in $image: $architectures" >&2; exit 1 ;;
  esac
  /usr/libexec/PlistBuddy -c 'Print :CFBundleIdentifier' "$mount/Nexora.app/Contents/Info.plist" | grep -Fx 'com.wiselyman.Nexora'
  grep -Fq 'http://localhost:18080' "$mount/README.txt"
  diskutil unmount "$mount" >/dev/null
  diskutil eject "$device" >/dev/null
  device=""
  rmdir "$mount"
  trap - EXIT
done
printf 'macOS DMG contents and install guidance checks passed.\n'
