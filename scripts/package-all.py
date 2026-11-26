#!/usr/bin/env python3
"""Create headless-friendly Nexora server archives."""
from pathlib import Path
import io
import sys
import tarfile
import zipfile


def main() -> None:
    if len(sys.argv) != 3:
        raise SystemExit("usage: package-all.py VERSION DIST_DIR")
    version, dist_name = sys.argv[1:]
    dist = Path(dist_name).resolve()
    root = Path(__file__).resolve().parent.parent
    for os_name, arch in (("darwin", "amd64"), ("darwin", "arm64"), ("linux", "amd64"), ("linux", "arm64"), ("windows", "amd64"), ("windows", "arm64")):
        stem = f"Nexora-{version}-{os_name}-{arch}"
        binary = dist / (stem + (".exe" if os_name == "windows" else ""))
        if not binary.is_file():
            raise SystemExit(f"missing build artifact: {binary}")
        if os_name == "windows":
            archive = dist / f"{stem}.zip"
            with zipfile.ZipFile(archive, "w", compression=zipfile.ZIP_DEFLATED) as bundle:
                bundle.write(binary, "Nexora.exe")
                bundle.write(root / "scripts/install-windows.ps1", "install-windows.ps1")
                bundle.write(root / "scripts/rollback-windows.ps1", "rollback-windows.ps1")
                bundle.write(root / "scripts/uninstall-windows.ps1", "uninstall-windows.ps1")
                bundle.write(root / "README.md", "README.md")
                bundle.write(root / "docs/privacy-data-flow.md", "privacy-data-flow.md")
                bundle.writestr("INSTALL.txt", windows_instructions())
        else:
            archive = dist / f"{stem}.tar.gz"
            with tarfile.open(archive, "w:gz") as bundle:
                bundle.add(binary, arcname="Nexora", recursive=False)
                bundle.add(root / "scripts/install-unix.sh", arcname="install-unix.sh", recursive=False)
                bundle.add(root / "scripts/rollback-unix.sh", arcname="rollback-unix.sh", recursive=False)
                bundle.add(root / "scripts/uninstall-unix.sh", arcname="uninstall-unix.sh", recursive=False)
                if os_name == "linux":
                    bundle.add(root / "assets/5060.png", arcname="Nexora.png", recursive=False)
                bundle.add(root / "README.md", arcname="README.md", recursive=False)
                bundle.add(root / "docs/privacy-data-flow.md", arcname="privacy-data-flow.md", recursive=False)
                info = tarfile.TarInfo("INSTALL.txt")
                content = unix_instructions().encode()
                info.size = len(content)
                bundle.addfile(info, io.BytesIO(content))
        print(f"Packaged {archive.name}")


def unix_instructions() -> str:
    return ("Nexora server installation\n\n"
        "1. Extract this archive.\n2. Run: chmod +x install-unix.sh && ./install-unix.sh ./Nexora\n"
        "3. Start with: ~/.local/bin/Nexora -no-open. The setup URL is printed in the terminal.\n"
        "If an upgrade needs to be undone, run: bash ./rollback-unix.sh\n"
        "For a headless remote host, forward the UI with: ssh -L 18080:127.0.0.1:18080 user@server\n"
        "Then open the printed URL locally. The service stays bound to loopback; it exposes no public web port.\n")


def windows_instructions() -> str:
    return ("Nexora server installation\r\n\r\n"
        "In PowerShell run: .\\install-windows.ps1 -Source .\\Nexora.exe\r\n"
        'Then start with: & "$env:LOCALAPPDATA\\Programs\\Nexora\\Nexora.exe" -no-open\r\n'
        "The setup URL is printed in the terminal. For remote headless use, forward 127.0.0.1:18080 over SSH.\r\n")



if __name__ == "__main__":
    main()
