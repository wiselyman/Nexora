#!/usr/bin/env bash
set -euo pipefail
if [[ $# -ne 3 ]]; then
  echo "usage: package-macos-dmg.sh VERSION BINARY_DIR OUTPUT_DIR" >&2
  exit 2
fi
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
version="$1"
binary_dir="$(cd "$2" && pwd)"
output_dir="$3"
[[ "$output_dir" == /* ]] || output_dir="$root/$output_dir"
mkdir -p "$output_dir"

for arch in amd64 arm64; do
  binary="$binary_dir/Nexora-${version}-darwin-${arch}"
  [[ -f "$binary" ]] || { echo "missing macOS binary: $binary" >&2; exit 1; }
  temp_dir="$(mktemp -d "${TMPDIR:-/tmp}/nexora-dmg.XXXXXX")"
  trap 'rm -rf "$temp_dir"' EXIT
  app="$temp_dir/Nexora.app"
  contents="$app/Contents"
  mkdir -p "$contents/MacOS" "$contents/Resources"
  cp "$binary" "$contents/MacOS/Nexora"
  chmod 0755 "$contents/MacOS/Nexora"
  cat > "$contents/Info.plist" <<PLIST
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
<key>CFBundleName</key><string>Nexora</string>
<key>CFBundleDisplayName</key><string>Nexora</string>
<key>CFBundleIdentifier</key><string>com.wiselyman.Nexora</string>
<key>CFBundleExecutable</key><string>Nexora</string>
<key>CFBundlePackageType</key><string>APPL</string>
<key>CFBundleVersion</key><string>${version}</string>
<key>CFBundleShortVersionString</key><string>${version}</string>
<key>LSUIElement</key><true/>
<key>NSHighResolutionCapable</key><true/>
</dict></plist>
PLIST
  cat > "$temp_dir/README.txt" <<'README'
Nexora for macOS

Drag Nexora.app to Applications, then open it from Applications.
Nexora starts the local service and opens its setup page in your browser.
After the first launch, you can revisit http://localhost:18080.
Use the Exit Nexora button in the setup page to stop the local service.

The local setup page and MCP service only listen on this computer.
README
  ln -s /Applications "$temp_dir/Applications"
  hdiutil create -quiet -volname "Nexora ${version}" -srcfolder "$temp_dir" -ov -format UDZO \
    "$output_dir/Nexora-${version}-darwin-${arch}.dmg"
  rm -rf "$temp_dir"
  trap - EXIT
done
