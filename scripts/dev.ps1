# Chatty dev launcher (Windows)
# Wails v3 dev mode requires the frontend vite dev server to be ready first
# (port 9245). This script starts vite, waits for the port, runs 'wails3 dev'
# in the foreground, then cleans up vite on exit.
# Usage: powershell -ExecutionPolicy Bypass -File scripts/dev.ps1
$ErrorActionPreference = 'Stop'

$repo = Split-Path $PSScriptRoot -Parent
$frontend = Join-Path $repo 'frontend'

# 1) start vite dev server (9245)
$vite = Start-Process -FilePath 'npm.cmd' -ArgumentList 'run', 'dev', '--', '--port', '9245', '--strictPort' `
  -WorkingDirectory $frontend -WindowStyle Hidden -PassThru
Write-Host 'starting vite dev server (localhost:9245)...'

# 2) wait until the port is listening (up to 30s)
$ready = $false
for ($i = 0; $i -lt 60; $i++) {
  Start-Sleep -Milliseconds 500
  if (Get-NetTCPConnection -LocalPort 9245 -State Listen -ErrorAction SilentlyContinue) {
    $ready = $true
    break
  }
  if ($vite.HasExited) { break }
}
if (-not $ready) {
  Stop-Process -Id $vite.Id -Force -ErrorAction SilentlyContinue
  throw 'vite failed to start (port 9245 not listening). Check frontend: npm run dev.'
}
Write-Host 'vite ready. running wails3 dev (press Ctrl+C to exit)...'

# 3) run wails3 dev in the foreground
try {
  Push-Location $repo
  & wails3 dev
} finally {
  Pop-Location
  Stop-Process -Id $vite.Id -Force -ErrorAction SilentlyContinue
}
