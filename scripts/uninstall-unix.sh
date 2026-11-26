#!/usr/bin/env bash
set -euo pipefail

dest_dir="${DESTDIR:-$HOME/.local/bin}"
target="$dest_dir/Nexora"
if command -v pgrep >/dev/null 2>&1 && pgrep -x Nexora >/dev/null 2>&1; then
  printf 'Close Nexora before uninstalling it. No files were removed.\n' >&2
  exit 1
fi
if [[ "$(uname -s)" == "Darwin" ]]; then
  config_dir="$HOME/Library/Application Support/Nexora"
  startup_entry="$HOME/Library/LaunchAgents/com.nexora.agent.plist"
else
  config_home="${XDG_CONFIG_HOME:-$HOME/.config}"
  [[ "$config_home" == /* ]] || config_home="$HOME/.config"
  config_dir="$config_home/Nexora"
  startup_entry="$config_home/autostart/Nexora.desktop"
fi
startup_marker="$config_dir/startup-managed"
startup_managed_entry=""
startup_expected_digest=""
startup_safe_to_purge=1
if [[ -f "$startup_marker" ]]; then
  startup_managed_entry="$(sed -n '1p' "$startup_marker")"
  startup_expected_digest="$(sed -n '2p' "$startup_marker")"
  if [[ "$startup_managed_entry" != "$startup_entry" ]]; then
    startup_safe_to_purge=0
  elif [[ -e "$startup_entry" ]]; then
    if [[ ! "$startup_expected_digest" =~ ^[a-fA-F0-9]{64}$ ]]; then
      startup_safe_to_purge=0
    elif command -v sha256sum >/dev/null 2>&1; then
      [[ "$(sha256sum "$startup_entry" | awk '{print $1}')" == "$startup_expected_digest" ]] || startup_safe_to_purge=0
    elif command -v shasum >/dev/null 2>&1; then
      [[ "$(shasum -a 256 "$startup_entry" | awk '{print $1}')" == "$startup_expected_digest" ]] || startup_safe_to_purge=0
    else
      startup_safe_to_purge=0
    fi
  fi
fi

linux_data_home=""
app_data_dir=""
desktop_file=""
icon_file=""
desktop_digest=""
icon_digest=""
desktop_marker_present=0
if [[ "$(uname -s)" == "Linux" ]]; then
  linux_data_home="${XDG_DATA_HOME:-$HOME/.local/share}"
  [[ "$linux_data_home" == /* ]] || linux_data_home="$HOME/.local/share"
  app_data_dir="$linux_data_home/Nexora"
  desktop_file="$linux_data_home/applications/Nexora.desktop"
  icon_file="$linux_data_home/icons/hicolor/512x512/apps/Nexora.png"
  desktop_marker="$app_data_dir/desktop-launcher-managed"
  if [[ -f "$desktop_marker" ]]; then
    desktop_marker_present=1
    desktop_digest="$(sed -n 's/^desktop-sha256=//p' "$desktop_marker" | sed -n '1p')"
    icon_digest="$(sed -n 's/^icon-sha256=//p' "$desktop_marker" | sed -n '1p')"
  fi
fi

printf 'Your settings and fallback secrets are in %s. Credentials saved in the OS credential store are removed only if you choose to delete user data.\n' "$config_dir"
delete_user_data=0
if [[ -t 0 && -t 1 ]]; then
  printf 'Delete the Nexora settings and fallback secrets? This cannot be undone. [y/N] '
  read -r answer
  if [[ "$answer" == "y" || "$answer" == "Y" ]]; then delete_user_data=1; fi
fi
if [[ "$delete_user_data" -eq 1 && "$startup_safe_to_purge" -ne 1 ]]; then
  printf 'The login startup entry changed or cannot be verified; kept user data and left that entry unchanged.\n' >&2
  delete_user_data=0
fi
if [[ "$delete_user_data" -eq 1 ]]; then
  if [[ -x "$target" ]]; then
    if "$target" -purge-user-data; then
      printf 'Removed Nexora settings, fallback secrets, and OS credential-store entries.\n'
    else
      printf 'Could not remove all Nexora user data; no launch entries or application files were removed.\n' >&2
      exit 1
    fi
  else
    printf 'The Nexora binary is unavailable, so credentials could not be removed from the OS credential store; no files were removed.\n' >&2
    exit 1
  fi
else
  printf 'Kept Nexora settings and fallback secrets.\n'
fi

if [[ -f "$startup_marker" || ( "$delete_user_data" -eq 1 && -n "$startup_managed_entry" ) ]]; then
  if [[ "$startup_managed_entry" == "$startup_entry" ]]; then
    if [[ ! -e "$startup_entry" ]]; then
      rm -f -- "$startup_marker"
      printf 'Removed the stale Nexora startup ownership marker.\n'
    elif [[ "$startup_expected_digest" =~ ^[a-fA-F0-9]{64}$ ]]; then
      actual_digest=""
      if command -v sha256sum >/dev/null 2>&1; then
        actual_digest="$(sha256sum "$startup_entry" | awk '{print $1}')"
      elif command -v shasum >/dev/null 2>&1; then
        actual_digest="$(shasum -a 256 "$startup_entry" | awk '{print $1}')"
      fi
      if [[ "$actual_digest" == "$startup_expected_digest" ]]; then
        rm -f -- "$startup_entry" "$startup_marker"
        rmdir "$(dirname "$startup_entry")" 2>/dev/null || true
        printf 'Removed the Nexora login startup entry.\n'
      else
        printf 'The Nexora login startup entry changed after installation; it was left unchanged.\n' >&2
      fi
    else
      printf 'The Nexora login startup entry could not be verified; it was left unchanged.\n' >&2
    fi
  fi
fi

if [[ -n "$app_data_dir" && ( "$desktop_marker_present" -eq 1 || -n "$desktop_digest$icon_digest" ) ]]; then
  sha256_file() {
    if command -v shasum >/dev/null 2>&1; then
      shasum -a 256 "$1" | awk '{print $1}'
    elif command -v sha256sum >/dev/null 2>&1; then
      sha256sum "$1" | awk '{print $1}'
    else
      return 1
    fi
  }
  if [[ "$desktop_digest" =~ ^[a-fA-F0-9]{64}$ ]] && [[ -f "$desktop_file" ]] && [[ "$(sha256_file "$desktop_file")" == "$desktop_digest" ]]; then
    rm -f -- "$desktop_file"
    printf 'Removed the unchanged Nexora desktop menu entry.\n'
  elif [[ -e "$desktop_file" ]]; then
    printf 'The desktop menu entry changed or cannot be verified; it was left unchanged.\n' >&2
  fi
  if [[ "$icon_digest" =~ ^[a-fA-F0-9]{64}$ ]] && [[ -f "$icon_file" ]] && [[ "$(sha256_file "$icon_file")" == "$icon_digest" ]]; then
    rm -f -- "$icon_file"
    printf 'Removed the unchanged Nexora application icon.\n'
  elif [[ -e "$icon_file" ]]; then
    printf 'The application icon changed or cannot be verified; it was left unchanged.\n' >&2
  fi
  rm -f -- "$app_data_dir/desktop-launcher-managed"
  rmdir "$app_data_dir" 2>/dev/null || true
fi

if [[ -e "$target" ]]; then
  rm -f -- "$target"
  printf 'Removed %s\n' "$target"
else
  printf 'Nexora binary is not installed at %s\n' "$target"
fi
printf 'The previous-version binary backup, if any, was kept for rollback.\n'
