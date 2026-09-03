# Copy Microsoft YaHei fonts from the system into frontend/public/fonts
# for Typst layout. YaHei is proprietary and must NOT be redistributed:
# these two files are excluded via .gitignore, so they never enter the repo
# or the release bundle - local build / runtime only.
# Usage: powershell -ExecutionPolicy Bypass -File scripts/fetch-system-fonts.ps1
$ErrorActionPreference = 'Stop'

$sysFonts = "$env:WINDIR\Fonts"
$target = Join-Path $PSScriptRoot '..\frontend\public\fonts'
New-Item -ItemType Directory -Force -Path $target | Out-Null

$files = @{
  'msyh.ttc'   = 'Microsoft YaHei Regular'
  'msyhbd.ttc' = 'Microsoft YaHei Bold'
}

foreach ($name in $files.Keys) {
  $src = Join-Path $sysFonts $name
  $dst = Join-Path $target $name
  if (Test-Path $src) {
    Copy-Item -Force $src $dst
    $mb = [Math]::Round((Get-Item $dst).Length / 1MB, 1)
    Write-Host "copied $name ($mb MB) - $($files[$name])"
  } else {
    Write-Warning "system font not found: $src"
  }
}
Write-Host 'done. If YaHei is missing, manually copy msyh.ttc / msyhbd.ttc from C:\Windows\Fonts into frontend\public\fonts\.'
