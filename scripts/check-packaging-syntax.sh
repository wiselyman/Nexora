#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
while IFS= read -r -d '' script; do
  bash -n "$script"
done < <(find "$root/scripts" -maxdepth 1 -type f -name '*.sh' -print0)

python3 - "$root/scripts" <<'PY'
import ast
import pathlib
import sys

for source in pathlib.Path(sys.argv[1]).glob("*.py"):
    ast.parse(source.read_text(encoding="utf-8"), filename=str(source))
PY

python3 - "$root/.github/workflows/release.yml" <<'PY'
import pathlib
import sys

source = pathlib.Path(sys.argv[1]).read_text(encoding="utf-8")
required = (
    "version: ${{ steps.version.outputs.version }}",
    'echo "version=$version" >> "$GITHUB_OUTPUT"',
    "VERSION: ${{ needs.build.outputs.version }}",
    "package-linux:",
    "package-macos:",
    "package-windows:",
    "assemble:",
    "attest:",
    "publish:",
    "./scripts/package-linux.sh",
    "./scripts/package-macos-dmg.sh",
    "./scripts/test-package-macos-dmg.sh",
    "./scripts/package-windows.ps1",
    "./scripts/test-linux-packages.sh",
    "python3 ./scripts/test-installer-inventory.py",
    "python3 ./scripts/test-release-inventory.py",
    'python3 ./scripts/verify-release-inventory.py "$VERSION" dist',
    "Nexora-<version>-linux-<amd64-or-arm64>.deb",
    "Nexora-<version>-linux-<amd64-or-arm64>.rpm",
    "-setup.exe",
    "http://localhost:18080",
    "dist/SHA256SUMS",
)
missing = [item for item in required if item not in source]
if missing:
    raise SystemExit(f"release workflow is missing native installer gates: {missing}")
if source.count("path: dist/*") != 1:
    raise SystemExit("only the post-verification assembled inventory may be uploaded as dist/*")
if source.index('python3 ./scripts/verify-release-inventory.py "$VERSION" dist') > source.index("name: release-dist"):
    raise SystemExit("the exact asset inventory must pass before upload")
if "VERSION: ${{ github.ref_name }}" in source:
    raise SystemExit("release jobs must use the normalized build version, not the raw tag")
ci_source = pathlib.Path(sys.argv[1]).with_name("ci.yml").read_text(encoding="utf-8")
for test in ("python3 ./scripts/test-release-inventory.py", "python3 ./scripts/test-installer-inventory.py"):
    if test not in ci_source:
        raise SystemExit(f"CI must run {test}")
PY

while IFS='|' read -r wrapper target; do
  wrapper_path="$root/scripts/$wrapper"
  target_path="$root/scripts/$target"
  [[ -f "$wrapper_path" ]] || { printf 'Missing Windows launcher: %s\n' "$wrapper" >&2; exit 1; }
  [[ -f "$target_path" ]] || { printf 'Missing PowerShell target for %s: %s\n' "$wrapper" "$target" >&2; exit 1; }
  rg -q --fixed-strings "$target" "$wrapper_path" || {
    printf 'Windows launcher %s does not reference %s\n' "$wrapper" "$target" >&2
    exit 1
  }
done <<'LAUNCHERS'
install-windows.cmd|install-windows.ps1
rollback-windows.cmd|rollback-windows.ps1
uninstall-windows.cmd|uninstall-windows.ps1
LAUNCHERS

printf 'Packaging script syntax and launcher checks passed.\n'
