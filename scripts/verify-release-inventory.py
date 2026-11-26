#!/usr/bin/env python3
"""Validate the exact public release asset set, SBOMs, and checksum manifest."""

from hashlib import sha256
import json
from pathlib import Path
import re
import sys


TARGETS = (
    ("darwin", "amd64"),
    ("darwin", "arm64"),
    ("linux", "amd64"),
    ("linux", "arm64"),
    ("windows", "amd64"),
    ("windows", "arm64"),
)


def expected_names(version: str) -> tuple[set[str], set[str]]:
    public_assets = {"SHA256SUMS"}
    checksum_assets: set[str] = set()
    for os_name, arch in TARGETS:
        binary = f"Nexora-{version}-{os_name}-{arch}"
        if os_name == "windows":
            binary += ".exe"
            archive = binary.removesuffix(".exe") + ".zip"
        else:
            archive = binary + ".tar.gz"
        checksum_assets.update((binary, archive))
        public_assets.update((binary, archive, binary + ".spdx.json"))
    return public_assets, checksum_assets


def validate(dist: Path, version: str) -> None:
    if not re.fullmatch(r"[A-Za-z0-9][A-Za-z0-9._+-]*", version):
        raise ValueError("invalid release version")
    public_assets, checksum_assets = expected_names(version)
    entries = list(dist.iterdir())
    actual = {path.name for path in entries}
    if actual != public_assets:
        raise ValueError(
            f"release inventory mismatch; missing={sorted(public_assets - actual)}, "
            f"unexpected={sorted(actual - public_assets)}"
        )
    non_regular = sorted(path.name for path in entries if not path.is_file() or path.is_symlink())
    if non_regular:
        raise ValueError(f"release assets must be regular files: {non_regular}")

    manifest: dict[str, str] = {}
    for line_number, line in enumerate((dist / "SHA256SUMS").read_text(encoding="utf-8").splitlines(), 1):
        if len(line) < 67 or line[64:66] != "  ":
            raise ValueError(f"invalid SHA256SUMS line {line_number}")
        digest, name = line[:64], line[66:]
        if not re.fullmatch(r"[0-9a-fA-F]{64}", digest):
            raise ValueError(f"invalid SHA256 digest on line {line_number}")
        if name in manifest:
            raise ValueError(f"duplicate SHA256SUMS entry: {name}")
        manifest[name] = digest.lower()
    if set(manifest) != checksum_assets:
        raise ValueError(
            f"checksum inventory mismatch; missing={sorted(checksum_assets - set(manifest))}, "
            f"unexpected={sorted(set(manifest) - checksum_assets)}"
        )
    for name, expected_digest in manifest.items():
        actual_digest = sha256((dist / name).read_bytes()).hexdigest()
        if actual_digest != expected_digest:
            raise ValueError(f"SHA256 mismatch: {name}")

    for name in public_assets:
        if not name.endswith(".spdx.json"):
            continue
        try:
            document = json.loads((dist / name).read_text(encoding="utf-8"))
        except (UnicodeDecodeError, json.JSONDecodeError) as exc:
            raise ValueError(f"invalid SPDX JSON: {name}: {exc}") from exc
        if not isinstance(document, dict) or not document.get("spdxVersion") or not isinstance(document.get("packages"), list):
            raise ValueError(f"incomplete SPDX document: {name}")


def main() -> None:
    if len(sys.argv) != 3:
        raise SystemExit("usage: verify-release-inventory.py VERSION DIST_DIR")
    version, dist = sys.argv[1], Path(sys.argv[2]).resolve()
    if not dist.is_dir():
        raise SystemExit(f"release directory does not exist: {dist}")
    try:
        validate(dist, version)
    except (OSError, ValueError) as exc:
        raise SystemExit(str(exc)) from exc
    print("Release inventory, SBOMs, and SHA256 manifest verified.")


if __name__ == "__main__":
    main()
