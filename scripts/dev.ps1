# Chatty dev fallback launcher (Windows)
# Runs the same steps as 'wails3 dev' but without its startup race:
#   1) build the DEV binary
#   2) start vite (9245) in background
#   3) run the app ('wails3 task run')
# NOTE: no Go hot-reload (re-run this script after Go changes); frontend has
# vite HMR. Prefer plain 'wails3 dev' when it works.
# Usage: powershell -ExecutionPolicy Bypass -File scripts/dev.ps1
$ErrorActionPreference = 'Stop'

$repo = Split-Path $PSScriptRoot -Parent
$frontend = Join-Path $repo 'frontend'

# 1) build dev binary
Write-Host 'step 1/3: wails3 build DEV=true ...'
Push-Location $repo
try {
  & wails3 build DEV=true
  if ($LASTEXITCODE -ne 0) { throw 'wails3 build failed' }
} finally { Pop-Location }

# 2) start vite (9245)
$vite = Start-Process -FilePath 'npm.cmd' -ArgumentList 'run', 'dev', '--', '--port', '9245', '--strictPort' `
  -WorkingDirectory $frontend -WindowStyle Hidden -PassThru
Write-Host 'step 2/3: starting vite dev server (localhost:9245)...'

$ready = $false
for ($i = 0; $i -lt 60; $i++) {
  Start-Sleep -Milliseconds 500
  if (Get-NetTCPConnection -LocalPort 9245 -State Listen -ErrorAction SilentlyContinue) { $ready = $true; break }
  if ($vite.HasExited) { break }
}
if (-not $ready) {
  Stop-Process -Id $vite.Id -Force -ErrorAction SilentlyContinue
  throw 'vite failed to start (port 9245 not listening). Check frontend: npm run dev.'
}
Write-Host 'vite ready.'

# 3) run the app
Write-Host 'step 3/3: running app (press Ctrl+C to exit)...'
try {
  Push-Location $repo
  & wails3 task run
} finally {
  Pop-Location
  Stop-Process -Id $vite.Id -Force -ErrorAction SilentlyContinue
}
