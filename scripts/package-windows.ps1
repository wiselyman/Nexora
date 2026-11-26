param(
  [Parameter(Mandatory=$true)][string]$Version,
  [Parameter(Mandatory=$true)][string]$BinaryDirectory,
  [Parameter(Mandatory=$true)][string]$OutputDirectory
)
$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot
$binaryDirectory = (Resolve-Path -LiteralPath $BinaryDirectory).Path
New-Item -ItemType Directory -Force -Path $OutputDirectory | Out-Null
$outputDirectory = (Resolve-Path -LiteralPath $OutputDirectory).Path
$compiler = "${env:ProgramFiles(x86)}\Inno Setup 6\ISCC.exe"
if (!(Test-Path -LiteralPath $compiler)) { $compiler = "${env:ProgramFiles}\Inno Setup 6\ISCC.exe" }
if (!(Test-Path -LiteralPath $compiler)) { throw "Inno Setup 6 compiler not found: $compiler" }
$script = Join-Path $root "packaging\windows\Nexora.iss"
foreach ($arch in @("amd64", "arm64")) {
  $binary = Join-Path $binaryDirectory "Nexora-$Version-windows-$arch.exe"
  if (!(Test-Path -LiteralPath $binary -PathType Leaf)) { throw "Missing Windows binary: $binary" }
  & $compiler "/DAppVersion=$Version" "/DTargetArch=$arch" "/DBinaryPath=$binary" "/DOutputPath=$outputDirectory" $script
  if ($LASTEXITCODE -ne 0) { throw "Inno Setup failed for Windows/$arch with exit code $LASTEXITCODE" }
}

# Smoke-test a full user-scoped install and uninstall using the amd64 payload.
$installRoot = Join-Path $env:RUNNER_TEMP "Nexora-installer-smoke"
$installer = Join-Path $outputDirectory "Nexora-$Version-windows-amd64-setup.exe"
& $installer "/VERYSILENT" "/SUPPRESSMSGBOXES" "/NORESTART" "/DIR=$installRoot"
if ($LASTEXITCODE -ne 0) { throw "Silent installer smoke test failed with exit code $LASTEXITCODE" }
$installedBinary = Join-Path $installRoot "Nexora.exe"
if (!(Test-Path -LiteralPath $installedBinary -PathType Leaf)) { throw "Installer did not place Nexora.exe" }
$versionOutput = & $installedBinary -version
if ($LASTEXITCODE -ne 0 -or $versionOutput -ne $Version) { throw "Installed binary returned unexpected version: $versionOutput" }
$uninstaller = Join-Path $installRoot "unins000.exe"
& $uninstaller "/VERYSILENT" "/SUPPRESSMSGBOXES" "/NORESTART"
if ($LASTEXITCODE -ne 0) { throw "Silent uninstaller smoke test failed with exit code $LASTEXITCODE" }
if (Test-Path -LiteralPath $installedBinary) { throw "Uninstaller left Nexora.exe behind" }
Write-Host "Windows installer, version launch, and uninstall smoke checks passed."
