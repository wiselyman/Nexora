#!/usr/bin/env bash
set -euo pipefail
if [[ $# -ne 3 ]]; then
  echo "usage: package-linux.sh VERSION BINARY_DIR OUTPUT_DIR" >&2
  exit 2
fi
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
version="$1"
binary_dir="$(cd "$2" && pwd)"
output_dir="$3"
[[ "$output_dir" == /* ]] || output_dir="$root/$output_dir"
mkdir -p "$output_dir"
cd "$root"
temp_dir="$(mktemp -d "${TMPDIR:-/tmp}/nexora-nfpm.XXXXXX")"
trap 'rm -rf "$temp_dir"' EXIT
for arch in amd64 arm64; do
  binary="$binary_dir/Nexora-${version}-linux-${arch}"
  [[ -f "$binary" ]] || { echo "missing Linux binary: $binary" >&2; exit 1; }
  config="$temp_dir/nfpm-${arch}.json"
  python3 "$root/scripts/render-nfpm-config.py" "$version" "$arch" "$binary" \
    "$root/packaging/linux/postinstall.sh" "$config"
  for packager in deb rpm; do
    if [[ -n "${NFPM_BIN:-}" ]]; then
      "$NFPM_BIN" package --config "$config" --packager "$packager" \
        --target "$output_dir/Nexora-${version}-linux-${arch}.${packager}"
    else
      go run github.com/goreleaser/nfpm/v2/cmd/nfpm@v2.48.0 package \
        --config "$config" --packager "$packager" \
        --target "$output_dir/Nexora-${version}-linux-${arch}.${packager}"
    fi
  done
done
