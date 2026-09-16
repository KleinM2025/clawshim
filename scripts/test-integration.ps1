#!/usr/bin/env pwsh
# Integration smoke tests for clawshim (Windows).
#
# Requires: node on PATH (or pass -NodePath). Builds the shim (or reuses
# dist/integration/openclaw.exe when go is unavailable), then exercises the
# exact failure mode this tool exists for: long argv on Windows.
#
# Run from the repo root:  pwsh -File scripts/test-integration.ps1

param(
  [string]$NodePath = "",
  [switch]$SkipBuild
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
if (Get-Variable -Name PSNativeCommandArgumentPassing -ErrorAction SilentlyContinue) {
  $PSNativeCommandArgumentPassing = 'Standard' # exact native arg round-trip (pwsh 7.2+)
}

$repoRoot = Split-Path -Parent $PSScriptRoot
$distDir = Join-Path $repoRoot 'dist\integration'
$exe = Join-Path $distDir 'openclaw.exe'
New-Item -ItemType Directory -Force -Path $distDir | Out-Null

if (-not $SkipBuild) {
  $go = Get-Command go -ErrorAction SilentlyContinue
  if ($go) {
    Write-Host '== building =='
    Push-Location $repoRoot
    try { go build -o $exe ./cmd/clawshim } finally { Pop-Location }
  } elseif (Test-Path $exe) {
    Write-Host '== go not found; reusing existing build =='
  } else {
    throw "go not found and no prebuilt $exe"
  }
}

$node = if ($NodePath) { $NodePath } else {
  $c = Get-Command node -ErrorAction SilentlyContinue
  if ($c) { $c.Source } else { throw 'node not found; pass -NodePath' }
}
if (-not (Test-Path $node)) { throw "node not found at $node" }

$work = Join-Path ([IO.Path]::GetTempPath()) ("clawshim-it-" + [guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Force -Path $work | Out-Null
$failures = 0

function Assert([bool]$Cond, [string]$Msg) {
  if ($Cond) { Write-Host "  [ok]   $Msg" }
  else { Write-Host "  [FAIL] $Msg" -ForegroundColor Red; $script:failures++ }
}

function Run-Shim([string[]]$Arguments) {
  $stdoutFile = Join-Path $work 'stdout.txt'
  $stderrFile = Join-Path $work 'stderr.txt'
  Remove-Item $stdoutFile, $stderrFile -ErrorAction SilentlyContinue
  & $exe @Arguments 1> $stdoutFile 2> $stderrFile
  $code = $LASTEXITCODE
  return @{
    Code   = $code
    Stdout = (Get-Content $stdoutFile -Raw -ErrorAction SilentlyContinue)
    Stderr = (Get-Content $stderrFile -Raw -ErrorAction SilentlyContinue)
  }
}

try {
  Copy-Item (Join-Path $repoRoot 'testdata\stub-echo.mjs') $work

  $countFile = Join-Path $work 'count.txt'
  $cfg = @{
    runtime      = $node
    entry        = (Join-Path $work 'stub-echo.mjs')
    env          = @{
      CLAWSHIM_TEST_ENV        = 'injected-ok'
      CLAWSHIM_TEST_COUNT_FILE = $countFile
    }
    versionCache = @{ path = (Join-Path $work 'vc.json') }
  } | ConvertTo-Json -Depth 5
  Set-Content -Path (Join-Path $distDir 'clawshim.json') -Value $cfg -Encoding utf8NoBOM

  Write-Host '== case 1: short args + stdout purity =='
  $r = Run-Shim @('hello', 'world')
  Assert ($r.Code -eq 0) "exit 0 (got $($r.Code))"
  $j = $r.Stdout | ConvertFrom-Json
  Assert ($j.argc -eq 2) "argc=2 (got $($j.argc))"
  Assert ($j.maxArgLen -eq 5) "maxArgLen=5 (got $($j.maxArgLen))"
  Assert ([string]::IsNullOrWhiteSpace($r.Stderr)) "stderr quiet on success"

  Write-Host '== case 2: 12KB argument (incident size) =='
  $big = 'x' * 12000
  $r = Run-Shim @($big, 'tail')
  $j = $r.Stdout | ConvertFrom-Json
  Assert ($r.Code -eq 0) "exit 0"
  Assert ($j.maxArgLen -eq 12000) "12KB arg intact (got $($j.maxArgLen))"
  Assert ($j.argc -eq 2) "argc=2"

  Write-Host '== case 3: 30KB argument (near CreateProcess cap) =='
  $big30 = 'y' * 30000
  $r = Run-Shim @($big30)
  $j = $r.Stdout | ConvertFrom-Json
  Assert ($j.maxArgLen -eq 30000) "30KB arg intact (got $($j.maxArgLen))"

  Write-Host '== case 4: quoting / backslash / unicode round-trip =='
  $tricky = 'He said "hi" C:\path\with\backslash\ 中文 🦞'
  $r = Run-Shim @($tricky)
  $j = $r.Stdout | ConvertFrom-Json
  Assert ($j.maxArgLen -eq $tricky.Length) "tricky arg length $($tricky.Length) (got $($j.maxArgLen))"

  Write-Host '== case 5: exit code passthrough =='
  $r = Run-Shim @('exit:7')
  Assert ($r.Code -eq 7) "exit 7 (got $($r.Code))"

  Write-Host '== case 6: env injection =='
  $r = Run-Shim @()
  $j = $r.Stdout | ConvertFrom-Json
  Assert ($j.env -eq 'injected-ok') "env injected (got $($j.env))"

  Write-Host '== case 7: version probe + cache =='
  if (Test-Path $countFile) { Remove-Item $countFile }
  $r1 = Run-Shim @('--version')
  Assert ($r1.Code -eq 0 -and $r1.Stdout -match 'OpenClaw 2099\.1\.2') "first --version probed: $($r1.Stdout.Trim())"
  $runsAfterFirst = (Get-Content $countFile | Measure-Object -Line).Lines
  $r2 = Run-Shim @('--version')
  $runsAfterSecond = (Get-Content $countFile | Measure-Object -Line).Lines
  Assert ($r2.Stdout.Trim() -eq $r1.Stdout.Trim()) "second --version identical"
  Assert ($runsAfterSecond -eq $runsAfterFirst) "second --version served from cache (no extra node run)"

  Write-Host '== case 8: management commands =='
  $r = Run-Shim @('--shim-info')
  Assert ($r.Code -eq 0 -and $r.Stdout -match 'clawshim') "--shim-info ok"
  $r = Run-Shim @('--shim-doctor')
  Assert ($r.Code -eq 0) "--shim-doctor exit 0 (got $($r.Code))"
  Assert ($r.Stdout -match 'version probe:') "doctor ran live probe"
} finally {
  if ($failures -eq 0) {
    Remove-Item $work -Recurse -Force -ErrorAction SilentlyContinue
  } else {
    Write-Host "work dir kept for debugging: $work" -ForegroundColor Yellow
  }
}

if ($failures -gt 0) {
  Write-Host "$failures assertion(s) failed" -ForegroundColor Red
  exit 1
}
Write-Host 'all integration cases passed'
