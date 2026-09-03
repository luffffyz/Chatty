# Chatty 开发启动器（Windows）
# Wails v3 的 dev 模式需要前端 vite dev server 先就绪(端口 9245)。
# 本脚本: 启动 vite → 等待就绪 → 运行 wails3 dev(前台) → 退出时清理 vite。
# 用法: powershell -ExecutionPolicy Bypass -File scripts/dev.ps1
$ErrorActionPreference = 'Stop'

$repo = Split-Path $PSScriptRoot -Parent
$frontend = Join-Path $repo 'frontend'

# 1) 启动 vite dev server(9245)
$vite = Start-Process -FilePath 'npm.cmd' -ArgumentList 'run', 'dev', '--', '--port', '9245', '--strictPort' `
  -WorkingDirectory $frontend -WindowStyle Hidden -PassThru
Write-Host 'vite dev server 启动中(localhost:9245)...'

# 2) 等待端口就绪(最多 30s)
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
  throw 'vite 未能启动(端口 9245 未就绪)。请检查 frontend/npm run dev 是否报错。'
}
Write-Host 'vite 就绪。启动 wails3 dev(退出按 Ctrl+C)...'

# 3) 前台运行 wails3 dev
try {
  Push-Location $repo
  & wails3 dev
} finally {
  Pop-Location
  Stop-Process -Id $vite.Id -Force -ErrorAction SilentlyContinue
}
