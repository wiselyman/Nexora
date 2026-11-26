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
    "dist/Nexora-${{ env.VERSION }}-*",
    "python3 ./scripts/test-release-inventory.py",
    'python3 ./scripts/verify-release-inventory.py "$VERSION" dist',
    "dist/SHA256SUMS",
)
missing = [item for item in required if item not in source]
if missing:
    raise SystemExit(f"release workflow is missing inventory/version checks: {missing}")
if source.count("dist/Nexora-${{ env.VERSION }}-*") != 3:
    raise SystemExit("upload, attestation, and publishing must select the normalized version")
if "path: dist/*" in source:
    raise SystemExit("release upload must use the explicit versioned inventory")
if "VERSION: ${{ github.ref_name }}" in source:
    raise SystemExit("release jobs must use the normalized build version, not the raw tag")
ci_source = pathlib.Path(sys.argv[1]).with_name("ci.yml").read_text(encoding="utf-8")
if "python3 ./scripts/test-release-inventory.py" not in ci_source:
    raise SystemExit("CI must run the release inventory gate regression test")
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
