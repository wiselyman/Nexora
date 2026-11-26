#!/usr/bin/env python3
"""Render a concrete nfpm JSON config for one Nexora Linux package."""

import json
from pathlib import Path
import sys


def main() -> None:
    if len(sys.argv) != 6:
        raise SystemExit("usage: render-nfpm-config.py VERSION ARCH BINARY POSTINSTALL OUTPUT")
    version, arch, binary, postinstall, output = sys.argv[1:]
    if arch not in {"amd64", "arm64"}:
        raise SystemExit(f"unsupported Linux architecture: {arch}")
    config = {
        "name": "nexora",
        "arch": arch,
        "platform": "linux",
        "version": version,
        "release": "1",
        "section": "utils",
        "priority": "optional",
        "maintainer": "Yunfei Wang <wiselyman2008@gmail.com>",
        "description": "Nexora local service for configuring a private ChatGPT MCP connection.",
        "homepage": "https://github.com/wiselyman/Nexora",
        "vendor": "Nexora",
        "contents": [
            {"src": str(Path(binary).resolve()), "dst": "/usr/bin/nexora", "file_info": {"mode": int("0755", 8)}},
            {"src": "packaging/linux/nexora.desktop", "dst": "/usr/share/applications/nexora.desktop"},
            {"src": "assets/5060.png", "dst": "/usr/share/icons/hicolor/256x256/apps/nexora.png"},
            {"src": "README.md", "dst": "/usr/share/doc/nexora/README.md"},
            {"src": "docs/privacy-data-flow.md", "dst": "/usr/share/doc/nexora/privacy-data-flow.md"},
        ],
        "scripts": {"postinstall": str(Path(postinstall).resolve())},
    }
    Path(output).write_text(json.dumps(config, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")


if __name__ == "__main__":
    main()
