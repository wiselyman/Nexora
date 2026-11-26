#!/usr/bin/env bash
set -euo pipefail
if [[ $# -ne 2 ]]; then
  echo "usage: test-linux-packages.sh VERSION PACKAGE_DIR" >&2
  exit 2
fi
version="$1"
packages="$(cd "$2" && pwd)"
for arch in amd64 arm64; do
  deb="$packages/Nexora-${version}-linux-${arch}.deb"
  rpm="$packages/Nexora-${version}-linux-${arch}.rpm"
  [[ -s "$deb" ]] || { echo "missing Debian package: $deb" >&2; exit 1; }
  [[ -s "$rpm" ]] || { echo "missing RPM package: $rpm" >&2; exit 1; }
  deb_arch="$(dpkg-deb -f "$deb" Architecture)"
  [[ "$deb_arch" == "$arch" ]] || { echo "$deb architecture is $deb_arch, expected $arch" >&2; exit 1; }
  rpm_arch="$(rpm -qp --qf '%{ARCH}' "$rpm")"
  expected_rpm_arch=x86_64
  [[ "$arch" == arm64 ]] && expected_rpm_arch=aarch64
  [[ "$rpm_arch" == "$expected_rpm_arch" ]] || { echo "$rpm architecture is $rpm_arch, expected $expected_rpm_arch" >&2; exit 1; }
  deb_contents="$(dpkg-deb -c "$deb")"
  grep -Fq './usr/bin/nexora' <<<"$deb_contents"
  grep -Fq './usr/share/applications/nexora.desktop' <<<"$deb_contents"
  rpm_contents="$(rpm -qpl "$rpm")"
  grep -Fxq '/usr/bin/nexora' <<<"$rpm_contents"
  grep -Fxq '/usr/share/applications/nexora.desktop' <<<"$rpm_contents"
  rpm -qp --scripts "$rpm" | grep -Fq 'http://localhost:18080'
  deb_control="$packages/control-$arch"
  mkdir -p "$deb_control"
  dpkg-deb -e "$deb" "$deb_control"
  grep -Fq 'http://localhost:18080' "$deb_control/postinst"
  rm -rf "$deb_control"
done
printf 'Debian/RPM package metadata, architecture, payload, and setup prompt checks passed.\n'
