#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
tmp="$(mktemp -d)"
trap 'rm -rf -- "$tmp"' EXIT
version="package-smoke"

for target in windows/amd64 windows/arm64 darwin/amd64 darwin/arm64 linux/amd64 linux/arm64; do
	os="${target%/*}"
	arch="${target#*/}"
	ext=""
	[[ "$os" == "windows" ]] && ext=".exe"
	: > "$tmp/Nexora-${version}-${os}-${arch}${ext}"
done

python3 "$root/scripts/package-all.py" "$version" "$tmp"
for target in darwin/amd64 darwin/arm64 linux/amd64 linux/arm64; do
	os="${target%/*}"
	arch="${target#*/}"
	test -f "$tmp/Nexora-${version}-${os}-${arch}.tar.gz"
done
for arch in amd64 arm64; do
	test -f "$tmp/Nexora-${version}-windows-${arch}.zip"
done
test "$(find "$tmp" -maxdepth 1 \( -name '*.tar.gz' -o -name '*.zip' \) | wc -l | tr -d ' ')" -eq 6
python3 - "$tmp" <<'PY'
from pathlib import Path
import sys
import tarfile
import zipfile

dist = Path(sys.argv[1])
for archive in dist.glob("*.tar.gz"):
    with tarfile.open(archive, "r:gz") as bundle:
        names = set(bundle.getnames())
    required = {"Nexora", "install-unix.sh", "rollback-unix.sh", "uninstall-unix.sh", "README.md", "privacy-data-flow.md", "INSTALL.txt"}
    if "linux" in archive.name:
        required.add("Nexora.png")
    if not required <= names:
        raise SystemExit(f"{archive.name} missing {sorted(required - names)}")
for archive in dist.glob("*.zip"):
    with zipfile.ZipFile(archive) as bundle:
        names = set(bundle.namelist())
    required = {"Nexora.exe", "install-windows.ps1", "rollback-windows.ps1", "uninstall-windows.ps1", "README.md", "privacy-data-flow.md", "INSTALL.txt"}
    if not required <= names:
        raise SystemExit(f"{archive.name} missing {sorted(required - names)}")
PY
printf 'Six platform archives passed content and instruction validation.\n'
