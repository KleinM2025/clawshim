#!/usr/bin/env pwsh
<#
.SYNOPSIS
Removes a clawshim installation.

.DESCRIPTION
Deletes the installed shim files. Optionally removes the configuration and
version cache, and removes the install directory from the user PATH.

.EXAMPLE
pwsh -File uninstall.ps1

.EXAMPLE
pwsh -File uninstall.ps1 -RemoveConfig -RemoveFromPath
#>
param(
  [string]$InstallDir = (Join-Path $env:LOCALAPPDATA 'clawshim\bin'),
  [switch]$RemoveConfig,
  [switch]$RemoveFromPath
)

$ErrorActionPreference = 'Stop'

foreach ($f in @('openclaw.exe', 'README.md', 'clawshim.json', 'clawshim.json.example')) {
  $p = Join-Path $InstallDir $f
  if (Test-Path $p) {
    Remove-Item $p -Force
    Write-Host "removed $p"
  }
}

if ($RemoveConfig) {
  foreach ($d in @((Join-Path $env:LOCALAPPDATA 'clawshim'), (Join-Path $env:APPDATA 'clawshim'))) {
    if (Test-Path $d) {
      Remove-Item $d -Recurse -Force
      Write-Host "removed $d"
    }
  }
}

if ($RemoveFromPath) {
  $userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
  $parts = @($userPath -split ';' | Where-Object { $_ -ne '' -and $_ -ne $InstallDir })
  [Environment]::SetEnvironmentVariable('Path', ($parts -join ';'), 'User')
  Write-Host "removed $InstallDir from the user PATH"
}

Write-Host ''
Write-Host 'Notes:'
Write-Host '  - If you set MULTICA_OPENCLAW_PATH, remove it for the daemon user now.'
Write-Host "  - If the shim lived elsewhere (e.g. %USERPROFILE%\.multica\bin), delete it there too."
Write-Host '  - Then: multica daemon restart'
