$ErrorActionPreference = "Stop"
$running = Get-Process -Name "Nexora" -ErrorAction SilentlyContinue
if ($running) {
  throw "Close Nexora before rolling back. No files were changed."
}
$target = Join-Path $env:LOCALAPPDATA "Programs\Nexora"
$installed = Join-Path $target "Nexora.exe"
$previous = Join-Path $target "Nexora.previous.exe"
if (!(Test-Path $previous)) {
  throw "No previous Nexora version is available at $previous"
}
$staged = Join-Path $target (".Nexora.rollback.{0}.exe" -f [guid]::NewGuid().ToString("N"))
if (Test-Path $installed) { Copy-Item -Force $installed $staged }
Move-Item -Force $previous $installed
if (Test-Path $staged) { Move-Item -Force $staged $previous }
Write-Host "Restored the previous version to $installed"
