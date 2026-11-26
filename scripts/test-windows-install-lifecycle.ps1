$ErrorActionPreference = "Stop"
$repoRoot = Split-Path -Parent $PSScriptRoot
$tempRoot = Join-Path ([IO.Path]::GetTempPath()) ("Nexora-lifecycle-{0}" -f [guid]::NewGuid().ToString("N"))
$oldLocalAppData = $env:LOCALAPPDATA
$oldAppData = $env:APPDATA
try {
  $env:LOCALAPPDATA = Join-Path $tempRoot "local"
  $env:APPDATA = Join-Path $tempRoot "roaming"
  $target = Join-Path $env:LOCALAPPDATA "Programs\Nexora"
  $configDir = Join-Path $env:APPDATA "Nexora"
  $startMenu = Join-Path $env:APPDATA "Microsoft\Windows\Start Menu\Programs"
  $shortcut = Join-Path $startMenu "Nexora.lnk"
  New-Item -ItemType Directory -Force -Path $target,$configDir,$startMenu | Out-Null

  $v1 = Join-Path $tempRoot "Nexora-v1.exe"
  $v2 = Join-Path $tempRoot "Nexora-v2.exe"
  Set-Content -NoNewline -Path $v1 -Value "consumer-test-version-one"
  Set-Content -NoNewline -Path $v2 -Value "consumer-test-version-two"
  Set-Content -NoNewline -Path $shortcut -Value "user-owned-shortcut"
  [IO.File]::WriteAllText((Join-Path $configDir "config.json"), '{"keep":true}')
  [IO.File]::WriteAllText((Join-Path $configDir "secrets.json"), '{"runtime_api_key":"keep-for-test"}')

  & (Join-Path $PSScriptRoot "install-windows.ps1") -Source $v1
  if ((Get-Content -Raw (Join-Path $target "Nexora.exe")) -ne "consumer-test-version-one") { throw "First install did not place version one." }
  if (Test-Path (Join-Path $target "Nexora.previous.exe")) { throw "First install unexpectedly created a previous version." }

  & (Join-Path $PSScriptRoot "install-windows.ps1") -Source $v2
  if ((Get-Content -Raw (Join-Path $target "Nexora.exe")) -ne "consumer-test-version-two") { throw "Upgrade did not place version two." }
  if ((Get-Content -Raw (Join-Path $target "Nexora.previous.exe")) -ne "consumer-test-version-one") { throw "Upgrade did not preserve version one." }

  & (Join-Path $PSScriptRoot "rollback-windows.ps1")
  if ((Get-Content -Raw (Join-Path $target "Nexora.exe")) -ne "consumer-test-version-one") { throw "Rollback did not restore version one." }
  if ((Get-Content -Raw (Join-Path $target "Nexora.previous.exe")) -ne "consumer-test-version-two") { throw "Rollback did not preserve version two." }

  function Read-Host { return "n" }
  try {
    . (Join-Path $PSScriptRoot "uninstall-windows.ps1")
  } finally {
    Remove-Item Function:\Read-Host -ErrorAction SilentlyContinue
  }
  if (Test-Path $target) { throw "Uninstaller left the application directory behind." }
  if ((Get-Content -Raw (Join-Path $configDir "config.json")) -ne '{"keep":true}') { throw "Uninstall changed the saved configuration." }
  if ((Get-Content -Raw (Join-Path $configDir "secrets.json")) -ne '{"runtime_api_key":"keep-for-test"}') { throw "Uninstall changed fallback secrets." }
  if ((Get-Content -Raw $shortcut) -ne "user-owned-shortcut") { throw "Uninstall removed or changed an unowned Start Menu shortcut." }

  # A managed shortcut is removed only while its contents still match the
  # ownership digest saved by the installer.
  New-Item -ItemType Directory -Force -Path $target | Out-Null
  $managedShortcutContent = "installer-managed-shortcut"
  Set-Content -NoNewline -Path $shortcut -Value $managedShortcutContent
  $managedShortcutHash = (Get-FileHash -LiteralPath $shortcut -Algorithm SHA256).Hash
  Set-Content -Encoding ASCII -Path (Join-Path $target "start-menu-shortcut-managed") -Value @($shortcut, $managedShortcutHash)
  function Read-Host { return "n" }
  try {
    . (Join-Path $PSScriptRoot "uninstall-windows.ps1")
  } finally {
    Remove-Item Function:\Read-Host -ErrorAction SilentlyContinue
  }
  if (Test-Path $shortcut) { throw "Uninstall left an unchanged installer-managed shortcut behind." }

  New-Item -ItemType Directory -Force -Path $target | Out-Null
  Set-Content -NoNewline -Path $shortcut -Value "modified-by-user"
  $originalShortcutHash = (Get-FileHash -LiteralPath $shortcut -Algorithm SHA256).Hash
  Set-Content -NoNewline -Path $shortcut -Value "changed-after-install"
  Set-Content -Encoding ASCII -Path (Join-Path $target "start-menu-shortcut-managed") -Value @($shortcut, $originalShortcutHash)
  function Read-Host { return "n" }
  try {
    . (Join-Path $PSScriptRoot "uninstall-windows.ps1")
  } finally {
    Remove-Item Function:\Read-Host -ErrorAction SilentlyContinue
  }
  if ((Get-Content -Raw $shortcut) -ne "changed-after-install") { throw "Uninstall removed or changed a modified installer-managed shortcut." }
  Write-Host "Windows install/upgrade/rollback/uninstall lifecycle checks passed."
} finally {
  $env:LOCALAPPDATA = $oldLocalAppData
  $env:APPDATA = $oldAppData
  if (Test-Path $tempRoot) { Remove-Item -Recurse -Force $tempRoot }
}
