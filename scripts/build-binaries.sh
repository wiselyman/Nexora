#!/usr/bin/env bash
set -euo pipefail
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
out="${DIST_DIR:-$root/dist}"
[[ "$out" == /* ]] || out="$root/$out"
binary_dir="$out/bin"
mkdir -p "$binary_dir"
version="${VERSION:-dev}"
if [[ ! "$version" =~ ^[A-Za-z0-9][A-Za-z0-9._+-]*$ ]]; then
  echo "Invalid VERSION: use letters, digits, dot, underscore, plus or hyphen only." >&2
  exit 2
fi
for target in windows/amd64 windows/arm64 darwin/amd64 darwin/arm64 linux/amd64 linux/arm64; do
  os="${target%/*}"; arch="${target#*/}"; ext=""
  [[ "$os" == windows ]] && ext=".exe"
  name="Nexora-${version}-${os}-${arch}"
  echo "Building Nexora server for $os/$arch"
  (cd "$root" && GOOS="$os" GOARCH="$arch" CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -X nexora/internal/app.Version=$version" -o "$binary_dir/${name}${ext}" ./cmd/nexora)
done
