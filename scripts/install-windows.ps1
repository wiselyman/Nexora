param(
  [Parameter(Mandatory=$true)][string]$Source
)
$ErrorActionPreference = "Stop"
$running = Get-Process -Name "Nexora" -ErrorAction SilentlyContinue
if ($running) {
  throw "Close Nexora before upgrading it. No files were changed."
}
$sourcePath = (Resolve-Path -LiteralPath $Source -ErrorAction Stop).Path
if (!(Test-Path -LiteralPath $sourcePath -PathType Leaf)) {
  throw "Installer source must be a file: $Source"
}
$target = Join-Path $env:LOCALAPPDATA "Programs\Nexora"
New-Item -ItemType Directory -Force -Path $target | Out-Null
$installed = Join-Path $target "Nexora.exe"
$previous = Join-Path $target "Nexora.previous.exe"
$staged = Join-Path $target (".Nexora.new.{0}.exe" -f [guid]::NewGuid().ToString("N"))
$hadInstalled = Test-Path -LiteralPath $installed -PathType Leaf
try {
  Copy-Item -Force $sourcePath $staged
} catch {
  if (Test-Path $staged) { Remove-Item -Force $staged }
  throw
}
if ($hadInstalled) {
  Copy-Item -Force $installed $previous
}
try {
  Move-Item -Force $staged $installed
} catch {
  if ($hadInstalled -and (Test-Path $previous)) {
    Copy-Item -Force $previous $installed
  } elseif (!$hadInstalled -and (Test-Path $installed)) {
    Remove-Item -Force $installed
  }
  throw
} finally {
  if (Test-Path $staged) { Remove-Item -Force $staged }
}
$startMenu = Join-Path $env:APPDATA "Microsoft\Windows\Start Menu\Programs"
$shortcutPath = Join-Path $startMenu "Nexora.lnk"
$shortcutMarker = Join-Path $target "start-menu-shortcut-managed"
$mayManageShortcut = !(Test-Path $shortcutPath)
if ((Test-Path $shortcutPath) -and (Test-Path $shortcutMarker)) {
  $markerLines = Get-Content $shortcutMarker
  if ($markerLines.Count -ge 2 -and $markerLines[0] -eq $shortcutPath -and
      $markerLines[1] -match '^[A-Fa-f0-9]{64}$') {
    $currentShortcutHash = (Get-FileHash -LiteralPath $shortcutPath -Algorithm SHA256).Hash
    $mayManageShortcut = $currentShortcutHash -ieq $markerLines[1]
  }
}
try {
  New-Item -ItemType Directory -Force -Path $startMenu | Out-Null
  if ($mayManageShortcut) {
    $shell = New-Object -ComObject WScript.Shell
    $shortcut = $shell.CreateShortcut($shortcutPath)
    $shortcut.TargetPath = Join-Path $target "Nexora.exe"
    $shortcut.WorkingDirectory = $target
    $shortcut.Description = "Nexora local computer assistant"
    $shortcut.WindowStyle = 7
    $shortcut.Save()
    $shortcutHash = (Get-FileHash -LiteralPath $shortcutPath -Algorithm SHA256).Hash
    Set-Content -Encoding ASCII -Path $shortcutMarker -Value @($shortcutPath, $shortcutHash)
    Write-Host "A Nexora shortcut was added to the Start Menu."
  } else {
    Write-Warning "A same-name Start Menu shortcut already exists and is not managed by this installer; it was left unchanged."
  }
} catch {
  Write-Warning ("Nexora is installed, but its Start Menu shortcut could not be created: {0}" -f $_.Exception.Message)
}
Write-Host "Installed Nexora to $target\Nexora.exe"
Write-Host "If no Start Menu shortcut is available, launch this file directly: $installed"
if (Test-Path $previous) { Write-Host "Previous version saved. Run rollback-windows.ps1 from this folder to restore it." }
