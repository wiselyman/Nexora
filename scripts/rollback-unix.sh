#!/usr/bin/env bash
set -euo pipefail

dest_dir="${DESTDIR:-$HOME/.local/bin}"
target="$dest_dir/Nexora"
previous="$dest_dir/Nexora.previous"
if [[ ! -f "$previous" ]]; then
  printf 'No previous Nexora version is available at %s\n' "$previous" >&2
  exit 1
fi
staged="$(mktemp "$dest_dir/.Nexora.rollback.XXXXXX")"
rm -f -- "$staged"
trap 'rm -f -- "$staged"' EXIT
if [[ -e "$target" ]]; then cp -p "$target" "$staged"; fi
mv -f -- "$previous" "$target"
if [[ -e "$staged" ]]; then mv -f -- "$staged" "$previous"; fi
printf 'Restored the previous version to %s\n' "$target"
