#!/usr/bin/env bash
set -euo pipefail

source_bin="${1:-./Nexora}"
dest_dir="${DESTDIR:-$HOME/.local/bin}"
target="$dest_dir/Nexora"
previous="$dest_dir/Nexora.previous"
sha256_file() {
  if command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$1" | awk '{print $1}'
  elif command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | awk '{print $1}'
  else
    return 1
  fi
}
if command -v pgrep >/dev/null 2>&1 && pgrep -x Nexora >/dev/null 2>&1; then
  printf 'Close Nexora before installing or upgrading it. No files were changed.\n' >&2
  exit 1
fi
if [[ ! -f "$source_bin" ]]; then
  printf 'Source binary not found: %s\n' "$source_bin" >&2
  exit 1
fi
source_bin="$(cd "$(dirname "$source_bin")" && pwd)/$(basename "$source_bin")"
mkdir -p "$dest_dir"
staged="$(mktemp "$dest_dir/.Nexora.new.XXXXXX")"
backup_staged=""
icon_staged=""
desktop_staged=""
had_target=0
cleanup() {
  rm -f -- "$staged"
  if [[ -n "$backup_staged" ]]; then rm -f -- "$backup_staged"; fi
  if [[ -n "$icon_staged" ]]; then rm -f -- "$icon_staged"; fi
  if [[ -n "$desktop_staged" ]]; then rm -f -- "$desktop_staged"; fi
}
trap cleanup EXIT
install -m 0755 "$source_bin" "$staged"
if [[ -e "$target" ]]; then
  had_target=1
  backup_staged="$(mktemp "$dest_dir/.Nexora.previous.XXXXXX")"
  cp -p "$target" "$backup_staged"
  mv -f -- "$backup_staged" "$previous"
  backup_staged=""
fi
if ! mv -f -- "$staged" "$target"; then
  if [[ "$had_target" -eq 1 && -e "$previous" ]]; then
    cp -p "$previous" "$target"
  else
    rm -f -- "$target"
  fi
  exit 1
fi
printf 'Installed Nexora to %s\n' "$target"
if [[ -e "$previous" ]]; then
  printf 'Previous version saved to %s; run rollback-unix.sh to restore it.\n' "$previous"
fi
if [[ "$(uname -s)" == "Linux" && ( -n "${DISPLAY:-}" || -n "${WAYLAND_DISPLAY:-}" ) ]]; then
  source_dir="$(cd "$(dirname "$source_bin")" && pwd)"
  icon_source="$source_dir/Nexora.png"
  data_home="${XDG_DATA_HOME:-$HOME/.local/share}"
  [[ "$data_home" == /* ]] || data_home="$HOME/.local/share"
  app_data_dir="$data_home/Nexora"
  desktop_dir="$data_home/applications"
  icon_dir="$data_home/icons/hicolor/512x512/apps"
  managed_marker="$app_data_dir/desktop-launcher-managed"
  desktop_file="$desktop_dir/Nexora.desktop"
  icon_path="$icon_dir/Nexora.png"
  if [[ -f "$icon_source" ]]; then
    may_replace=1
    if [[ -e "$desktop_file" || -e "$icon_path" ]]; then
      if [[ ! -f "$managed_marker" ]]; then
        may_replace=0
      else
        old_desktop_digest="$(sed -n 's/^desktop-sha256=//p' "$managed_marker" | sed -n '1p')"
        old_icon_digest="$(sed -n 's/^icon-sha256=//p' "$managed_marker" | sed -n '1p')"
        if [[ -e "$desktop_file" ]] && { [[ ! "$old_desktop_digest" =~ ^[a-fA-F0-9]{64}$ ]] || [[ "$(sha256_file "$desktop_file")" != "$old_desktop_digest" ]]; }; then
          may_replace=0
        fi
        if [[ -e "$icon_path" ]] && { [[ ! "$old_icon_digest" =~ ^[a-fA-F0-9]{64}$ ]] || [[ "$(sha256_file "$icon_path")" != "$old_icon_digest" ]]; }; then
          may_replace=0
        fi
      fi
    fi
    if [[ "$may_replace" -ne 1 ]]; then
      printf 'The existing Nexora desktop entry or icon changed or cannot be verified; left them unchanged.\n' >&2
    else
      mkdir -p "$app_data_dir" "$desktop_dir" "$icon_dir"
      icon_staged="$(mktemp "$icon_dir/.Nexora.png.XXXXXX")"
      desktop_staged="$(mktemp "$desktop_dir/.Nexora.desktop.XXXXXX")"
      install -m 0644 "$icon_source" "$icon_staged"
      cat > "$desktop_staged" <<'DESKTOP'
[Desktop Entry]
Type=Application
Name=Nexora
Comment=Connect this computer to ChatGPT
Exec=/bin/sh -c "exec \"$HOME/.local/bin/Nexora\""
Icon=Nexora
Terminal=false
Categories=Utility;
StartupNotify=false
DESKTOP
      mv -f -- "$icon_staged" "$icon_path"
      icon_staged=""
      mv -f -- "$desktop_staged" "$desktop_file"
      desktop_staged=""
      desktop_digest="$(sha256_file "$desktop_file")"
      icon_digest="$(sha256_file "$icon_path")"
      printf 'managed-by=Nexora-user-installer\ndesktop-sha256=%s\nicon-sha256=%s\n' \
        "$desktop_digest" "$icon_digest" > "$managed_marker"
      printf 'Added Nexora to the desktop application menu. Use the Exit Nexora button in the setup page to stop it.\n'
    fi
  fi
fi
printf 'Run it with: %s\n' "$target"
