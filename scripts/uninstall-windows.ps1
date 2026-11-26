$ErrorActionPreference = "Stop"
$running = Get-Process -Name "Nexora" -ErrorAction SilentlyContinue
if ($running) {
  throw "Close Nexora before uninstalling it. No files were removed."
}
$target = Join-Path $env:LOCALAPPDATA "Programs\Nexora"
$shortcut = Join-Path $env:APPDATA "Microsoft\Windows\Start Menu\Programs\Nexora.lnk"
$shortcutMarker = Join-Path $target "start-menu-shortcut-managed"
$configDir = Join-Path $env:APPDATA "Nexora"
$startupShortcut = Join-Path $env:APPDATA "Microsoft\Windows\Start Menu\Programs\Startup\Nexora.lnk"
$startupMarker = Join-Path $configDir "startup-managed"
$startupMarkerLines = @()
$startupSafeToPurge = $true
if (Test-Path $startupMarker) {
  $startupMarkerLines = @(Get-Content $startupMarker)
  if ($startupMarkerLines.Count -lt 2 -or $startupMarkerLines[0] -ne $startupShortcut -or
      $startupMarkerLines[1] -notmatch '^[A-Fa-f0-9]{64}$') {
    $startupSafeToPurge = $false
  } elseif (Test-Path $startupShortcut) {
    $currentStartupHash = (Get-FileHash -LiteralPath $startupShortcut -Algorithm SHA256).Hash
    if ($currentStartupHash -ine $startupMarkerLines[1]) { $startupSafeToPurge = $false }
  }
}

Write-Host "Settings and fallback secrets are stored in $configDir. Credentials saved in Windows Credential Manager are removed only if you choose to delete user data."
$answer = Read-Host "Delete the Nexora settings and fallback secrets? This cannot be undone (y/N)"
if ($answer -ceq "y" -and !$startupSafeToPurge) {
  Write-Warning "The login startup shortcut changed or cannot be verified; user data and that shortcut will be kept."
  $answer = "n"
}
if ($answer -ceq "y") {
  $installedBinary = Join-Path $target "Nexora.exe"
  if (!(Test-Path -LiteralPath $installedBinary -PathType Leaf)) {
    throw "The Nexora binary is unavailable, so its Windows Credential Manager entries could not be removed; no files were removed."
  }
  & $installedBinary -purge-user-data
  if ($LASTEXITCODE -ne 0) {
    throw "Could not remove all Nexora user data (exit $LASTEXITCODE); no launch entries or application files were removed."
  }
  Write-Host "Removed Nexora settings, fallback secrets, and Windows Credential Manager entries."
} else {
  Write-Host "Kept Nexora settings and fallback secrets."
}

if ($startupMarkerLines.Count -ge 2 -and $startupMarkerLines[0] -eq $startupShortcut) {
  if (!(Test-Path $startupShortcut)) {
    if (Test-Path $startupMarker) { Remove-Item -Force $startupMarker }
    Write-Host "Removed the stale Nexora startup ownership marker."
  } elseif ($startupMarkerLines[1] -match '^[A-Fa-f0-9]{64}$' -and
      (Get-FileHash -LiteralPath $startupShortcut -Algorithm SHA256).Hash -ieq $startupMarkerLines[1]) {
    Remove-Item -Force $startupShortcut
    if (Test-Path $startupMarker) { Remove-Item -Force $startupMarker }
    Write-Host "Removed the Nexora login startup shortcut."
  } else {
    Write-Warning "The login startup shortcut changed after Nexora created it; it was left unchanged."
  }
}
if (Test-Path $shortcutMarker) {
  $shortcutMarkerLines = @(Get-Content $shortcutMarker)
  $removeManagedShortcut = $false
  if ($shortcutMarkerLines.Count -ge 2 -and $shortcutMarkerLines[0] -eq $shortcut -and
      $shortcutMarkerLines[1] -match '^[A-Fa-f0-9]{64}$' -and (Test-Path $shortcut)) {
    $currentShortcutHash = (Get-FileHash -LiteralPath $shortcut -Algorithm SHA256).Hash
    $removeManagedShortcut = $currentShortcutHash -ieq $shortcutMarkerLines[1]
  }
  if ($removeManagedShortcut) {
    Remove-Item -Force $shortcut
    Write-Host "Removed the unchanged installer-managed Start Menu shortcut."
  } elseif (Test-Path $shortcut) {
    Write-Host "The same-name Start Menu shortcut changed or cannot be verified; it was left unchanged."
  }
  Remove-Item -Force $shortcutMarker
} else {
  Write-Host "Left any unowned same-name Start Menu shortcut unchanged."
}
if (Test-Path $target) { Remove-Item -Recurse -Force $target }
Write-Host "Removed the Nexora application."
