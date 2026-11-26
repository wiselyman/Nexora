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

foreach ($arch in @("amd64", "arm64")) {
  $installer = Join-Path $outputDirectory "Nexora-$Version-windows-$arch-setup.exe"
  $file = Get-Item -LiteralPath $installer -ErrorAction Stop
  if ($file.Length -lt 1MB) { throw "Windows installer is unexpectedly small: $installer" }
}
Write-Host "Windows amd64 and arm64 installers compiled and passed size checks."
