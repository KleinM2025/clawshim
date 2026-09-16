#!/usr/bin/env pwsh
<#
.SYNOPSIS
Installs clawshim as openclaw.exe and optionally configures it.

.DESCRIPTION
Downloads the latest release from GitHub, verifies the SHA256 checksum,
installs openclaw.exe, and optionally runs --shim-setup with a preset.

.EXAMPLE
pwsh -File install.ps1 -Preset autoclaw -AutoClawRoot "D:\AI\AutoClaw"

.EXAMPLE
pwsh -File install.ps1 -Preset npm -AddToPath

.EXAMPLE
pwsh -File install.ps1 -Preset custom -Runtime C:\nodejs\node.exe -Entry C:\openclaw\openclaw.mjs
#>
param(
  [string]$Repo = 'KleinM2025/clawshim',
  [string]$Version = 'latest',
  [string]$InstallDir = (Join-Path $env:LOCALAPPDATA 'clawshim\bin'),
  [ValidateSet('', 'npm', 'autoclaw', 'custom')][string]$Preset = '',
  [string]$AutoClawRoot = '',
  [string]$Runtime = '',
  [string]$Entry = '',
  [string]$StateDir = '',
  [switch]$AddToPath
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$arch = switch ([System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture) {
  'X64' { 'amd64' }
  'Arm64' { 'arm64' }
  default { throw "unsupported architecture: $_" }
}

if ($Version -eq 'latest') {
  $rel = Invoke-RestMethod "https://api.github.com/repos/$Repo/releases/latest"
  $tag = $rel.tag_name
} else {
  $tag = $Version
}

$zipName = "clawshim_${tag}_windows_${arch}.zip"
$base = "https://github.com/$Repo/releases/download/$tag"
$tmp = Join-Path ([IO.Path]::GetTempPath()) ("clawshim-install-" + [guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Force -Path $tmp | Out-Null

try {
  Write-Host "==> Downloading $zipName ($tag)"
  Invoke-WebRequest "$base/$zipName" -OutFile (Join-Path $tmp $zipName)
  Invoke-WebRequest "$base/checksums.txt" -OutFile (Join-Path $tmp 'checksums.txt')

  $line = Get-Content (Join-Path $tmp 'checksums.txt') |
    Where-Object { $_ -match [regex]::Escape($zipName) } |
    Select-Object -First 1
  if (-not $line) { throw "no checksum entry for $zipName" }
  $expectedHash = (($line -split '\s+')[0]).ToLower()
  $actualHash = (Get-FileHash (Join-Path $tmp $zipName) -Algorithm SHA256).Hash.ToLower()
  if ($actualHash -ne $expectedHash) {
    throw "checksum mismatch: expected $expectedHash, got $actualHash"
  }
  Write-Host '==> Checksum verified'

  New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
  Expand-Archive (Join-Path $tmp $zipName) -DestinationPath $InstallDir -Force
  $exe = Join-Path $InstallDir 'openclaw.exe'
  if (-not (Test-Path $exe)) { throw "openclaw.exe missing from archive" }
  Write-Host "==> Installed: $exe"

  if ($Preset -ne '') {
    $setupArgs = @('--shim-setup', '--preset', $Preset, '--force')
    if ($Preset -eq 'autoclaw') {
      if ($AutoClawRoot -eq '') { throw '-Preset autoclaw requires -AutoClawRoot' }
      $setupArgs += @('--autoclaw-root', $AutoClawRoot)
    }
    if ($Preset -eq 'custom') {
      if ($Runtime -eq '' -or $Entry -eq '') { throw '-Preset custom requires -Runtime and -Entry' }
      $setupArgs += @('--runtime', $Runtime, '--entry', $Entry)
      if ($StateDir -ne '') { $setupArgs += @('--state-dir', $StateDir) }
    }
    & $exe @setupArgs
    Write-Host '==> Running doctor'
    & $exe --shim-doctor
  } else {
    Write-Host "Next: configure with: & '$exe' --shim-setup --preset <npm|autoclaw|custom>"
  }

  Write-Host ''
  Write-Host 'To make Multica use this shim:'
  Write-Host "  PATH method: add $InstallDir to PATH *before* %APPDATA%\npm (or before any other openclaw)"
  Write-Host "  Env method:  set MULTICA_OPENCLAW_PATH to '$exe' for the daemon user, then: multica daemon restart"
  if ($AddToPath) {
    $userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
    if (($userPath -split ';') -notcontains $InstallDir) {
      [Environment]::SetEnvironmentVariable('Path', "$InstallDir;$userPath", 'User')
      Write-Host "==> Added $InstallDir to the front of the user PATH"
    } else {
      Write-Host "==> $InstallDir already on the user PATH"
    }
  } else {
    Write-Host "  (run with -AddToPath to add $InstallDir to your user PATH)"
  }
} finally {
  Remove-Item $tmp -Recurse -Force -ErrorAction SilentlyContinue
}
