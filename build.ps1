# MochiBox Build Script
$ErrorActionPreference = "Stop"

Write-Host "Checking/Downloading Kubo Binary..."
node scripts/download-kubo.js
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

Write-Host "Building Go Backend..."
Push-Location backend
go build -o mochibox-core.exe
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
Pop-Location

Write-Host "Building Frontend..."
Push-Location frontend
npm run build
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
Pop-Location

Write-Host "Preparing Electron..."
Push-Location electron
# Ensure bin directory exists
if (-not (Test-Path "resources/bin")) {
    New-Item -ItemType Directory -Force -Path "resources/bin"
}
Copy-Item ../backend/mochibox-core.exe resources/bin/

Write-Host "Building Electron App..."
# npm run build # Or electron-builder
# For now, we just ensure it compiles
npm run build
# npx electron-builder --win --x64
Pop-Location

Write-Host "Build Complete!"
