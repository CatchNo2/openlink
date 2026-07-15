<#
  OpenLink one-click build & install
  - Builds openlink.exe from the latest source in this repo
  - Overwrites %USERPROFILE%\.openlink\openlink.exe
  Usage:
      .\build.ps1          # backend only
      .\build.ps1 full     # backend + rebuild Chrome extension
  Note: uses the current working tree; does NOT run git pull.
#>

$ErrorActionPreference = 'Continue'
$Root      = Split-Path -Parent $MyInvocation.MyCommand.Definition
$TargetDir = Join-Path $env:USERPROFILE '.openlink'
$Target    = Join-Path $TargetDir 'openlink.exe'
$TempExe   = Join-Path $Root 'openlink_build_tmp.exe'

function Step($n, $msg) { Write-Host "`n[$n] $msg" -ForegroundColor Cyan }

# ---------- 1. build backend ----------
Step 1 "Building latest openlink.exe with Go ..."
Push-Location $Root
go build -o openlink_build_tmp.exe ./cmd/server/
if ($LASTEXITCODE -ne 0) {
    Write-Host "Build failed. Make sure Go (1.23+) is in PATH." -ForegroundColor Red
    Pop-Location
    exit 1
}
Pop-Location
Write-Host "    Build OK." -ForegroundColor Green

# ---------- 2. stop old process ----------
Step 2 "Stopping any running openlink process ..."
$old = Get-Process openlink -ErrorAction SilentlyContinue
if ($old) {
    $old | Stop-Process -Force
    Start-Sleep -Milliseconds 500
    Write-Host "    Old process stopped." -ForegroundColor Yellow
} else {
    Write-Host "    None running, skipped." -ForegroundColor Gray
}

# ---------- 3. install ----------
Step 3 "Installing to $Target ..."
if (-not (Test-Path $TargetDir)) { New-Item -ItemType Directory -Path $TargetDir | Out-Null }
Copy-Item $TempExe -Destination $Target -Force
Remove-Item $TempExe -Force

Write-Host "`nDone! Latest openlink installed at:" -ForegroundColor Green
Write-Host "   $Target"
Write-Host "   Built at: $(Get-Item $Target).LastWriteTime" -ForegroundColor Gray

# ---------- optional: build extension ----------
if ($args -contains 'full') {
    Write-Host "`n[optional] 'full' detected -> rebuilding Chrome extension ..." -ForegroundColor Cyan
    if (Get-Command node -ErrorAction SilentlyContinue) {
        Push-Location (Join-Path $Root 'extension')
        npm install
        npm run build
        Pop-Location
        Write-Host "Extension rebuilt (extension/dist/). Refresh it at chrome://extensions." -ForegroundColor Green
    } else {
        Write-Host "Node.js not found, skipping extension build." -ForegroundColor Yellow
    }
}

Write-Host "`nAll done." -ForegroundColor DarkGray
