#!/usr/bin/env python3
"""Exercise the release inventory gate with isolated temporary artifacts."""

import hashlib
import json
from pathlib import Path
import runpy
import subprocess
import sys
import tempfile

VERSION = "inventory-test"
VERIFIER = Path(__file__).with_name("verify-release-inventory.py")
expected_names = runpy.run_path(str(VERIFIER))["expected_names"]


def make_inventory(directory: Path) -> None:
    public_assets, checksum_assets = expected_names(VERSION)
    for name in public_assets:
        if name == "SHA256SUMS":
            continue
        if name.endswith(".spdx.json"):
            (directory / name).write_text(
                json.dumps({"spdxVersion": "SPDX-2.3", "packages": []}), encoding="utf-8"
            )
        else:
            (directory / name).write_bytes(name.encode("utf-8"))
    lines = [
        f"{hashlib.sha256((directory / name).read_bytes()).hexdigest()}  {name}"
        for name in sorted(checksum_assets)
    ]
    (directory / "SHA256SUMS").write_text("\n".join(lines) + "\n", encoding="utf-8")


def run_verifier(directory: Path) -> subprocess.CompletedProcess[str]:
    return subprocess.run(
        [sys.executable, str(VERIFIER), VERSION, str(directory)],
        text=True,
        capture_output=True,
        check=False,
    )


def main() -> None:
    with tempfile.TemporaryDirectory(prefix="Nexora-release-inventory-") as temp:
        root = Path(temp)
        valid = root / "valid"
        valid.mkdir()
        make_inventory(valid)
        result = run_verifier(valid)
        if result.returncode != 0:
            raise SystemExit(f"valid inventory was rejected: {result.stderr}")

        extra = root / "extra"
        extra.mkdir()
        make_inventory(extra)
        (extra / "stale-asset.zip").write_text("stale", encoding="utf-8")
        if run_verifier(extra).returncode == 0:
            raise SystemExit("unexpected stale asset was accepted")

        missing_sbom = root / "missing-sbom"
        missing_sbom.mkdir()
        make_inventory(missing_sbom)
        sbom = next(missing_sbom.glob("*.spdx.json"))
        sbom.unlink()
        if run_verifier(missing_sbom).returncode == 0:
            raise SystemExit("missing SBOM was accepted")

        invalid_sbom = root / "invalid-sbom"
        invalid_sbom.mkdir()
        make_inventory(invalid_sbom)
        sbom = next(invalid_sbom.glob("*.spdx.json"))
        sbom.write_text("not JSON", encoding="utf-8")
        if run_verifier(invalid_sbom).returncode == 0:
            raise SystemExit("invalid SPDX document was accepted")

        tampered = root / "tampered"
        tampered.mkdir()
        make_inventory(tampered)
        binary = next(path for path in tampered.iterdir() if path.name.endswith("-darwin-amd64"))
        binary.write_text("modified after checksumming", encoding="utf-8")
        if run_verifier(tampered).returncode == 0:
            raise SystemExit("artifact with a mismatched checksum was accepted")

    print("Release inventory gate tests passed.")


if __name__ == "__main__":
    main()
