# MochiBox Build Script
$ErrorActionPreference = "Stop"

Write-Host "1. Downloading Multi-platform IPFS Binaries..."
node scripts/download-kubo.js
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

Write-Host "2. Building Go Backend (Cross-Compile)..."
node scripts/build-backend.js
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

Write-Host "3. Building Frontend..."
Push-Location frontend
npm run build
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
Pop-Location

Write-Host "4. Preparing Electron..."
Push-Location electron
npm run build
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

Write-Host "Build Prep Complete!"
Write-Host "To package for specific platforms, run inside 'electron' directory:"
Write-Host "  npm run dist -- --win"
Write-Host "  npm run dist -- --linux"
Write-Host "  npm run dist -- --mac"
Pop-Location

