#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
tmp="$(mktemp -d)"
trap 'rm -rf -- "$tmp"' EXIT

export HOME="$tmp/home"
export XDG_CONFIG_HOME="$HOME/.config"
export XDG_DATA_HOME="$HOME/.local/share"
unset DISPLAY WAYLAND_DISPLAY
if [[ "$(uname -s)" == "Darwin" ]]; then
	config_dir="$HOME/Library/Application Support/Nexora"
else
	config_dir="$XDG_CONFIG_HOME/Nexora"
fi
mkdir -p "$HOME" "$tmp/bin" "$config_dir"
export DESTDIR="$tmp/bin"

make_fixture() {
	local path="$1" version="$2"
	cat > "$path" <<EOF
#!/bin/sh
printf '%s\\n' '$version'
EOF
	chmod 0755 "$path"
}

expect_version() {
	local path="$1" expected="$2" actual
	actual="$("$path")"
	if [[ "$actual" != "$expected" ]]; then
		printf 'Expected %s to report %s, got %s\n' "$path" "$expected" "$actual" >&2
		exit 1
	fi
}

make_fixture "$tmp/Nexora-v1" "version-one"
make_fixture "$tmp/Nexora-v2" "version-two"
printf 'keep-me\n' > "$config_dir/config.json"
printf 'secret-fallback\n' > "$config_dir/secrets.json"

bash "$root/scripts/install-unix.sh" "$tmp/Nexora-v1"
expect_version "$DESTDIR/Nexora" "version-one"
test -x "$DESTDIR/Nexora"
test ! -e "$DESTDIR/Nexora.previous"

bash "$root/scripts/install-unix.sh" "$tmp/Nexora-v2"
expect_version "$DESTDIR/Nexora" "version-two"
expect_version "$DESTDIR/Nexora.previous" "version-one"

bash "$root/scripts/rollback-unix.sh"
expect_version "$DESTDIR/Nexora" "version-one"
expect_version "$DESTDIR/Nexora.previous" "version-two"

bash "$root/scripts/uninstall-unix.sh"
test ! -e "$DESTDIR/Nexora"
expect_version "$DESTDIR/Nexora.previous" "version-two"
test -f "$config_dir/config.json"
test -f "$config_dir/secrets.json"
if find "$DESTDIR" -maxdepth 1 -name '.5060.*' -print -quit | grep -q .; then
	printf 'temporary installer files were left behind\n' >&2
	exit 1
fi
printf 'Unix install/upgrade/rollback/uninstall lifecycle checks passed.\n'
