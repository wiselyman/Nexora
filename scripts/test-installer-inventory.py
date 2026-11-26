#!/usr/bin/env python3
"""Verify installer filenames and the user-facing first-run guidance."""

from pathlib import Path
import runpy

ROOT = Path(__file__).resolve().parent.parent
VERSION = "0.2.0"
expected_names = runpy.run_path(str(ROOT / "scripts/verify-release-inventory.py"))["expected_names"]

expected_installers = {
    f"Nexora-{VERSION}-darwin-amd64.dmg",
    f"Nexora-{VERSION}-darwin-arm64.dmg",
    f"Nexora-{VERSION}-linux-amd64.deb",
    f"Nexora-{VERSION}-linux-amd64.rpm",
    f"Nexora-{VERSION}-linux-arm64.deb",
    f"Nexora-{VERSION}-linux-arm64.rpm",
    f"Nexora-{VERSION}-windows-amd64-setup.exe",
    f"Nexora-{VERSION}-windows-arm64-setup.exe",
}
assets, checksummed = expected_names(VERSION)
assert expected_installers <= assets, f"release is missing installers: {sorted(expected_installers - assets)}"
assert all(name.endswith(".spdx.json") for name in assets - expected_installers - {"SHA256SUMS"})
assert checksummed == assets - {"SHA256SUMS"}, "all published files must be checksummed"

required_prompt = "http://localhost:18080"
prompt_files = (
    ROOT / "packaging/linux/postinstall.sh",
    ROOT / "packaging/windows/Nexora.iss",
    ROOT / "scripts/package-macos-dmg.sh",
    ROOT / "README.md",
)
for path in prompt_files:
    content = path.read_text(encoding="utf-8")
    assert required_prompt in content, f"{path.relative_to(ROOT)} does not show the first-run address"

print("Native installer inventory and setup guidance checks passed.")
