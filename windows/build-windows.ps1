# PowerShell script for building S-UI on Windows
param(
    [string]$Architecture = "amd64",
    [switch]$NoCGO,
    [switch]$Help
)

if ($Help) {
    Write-Host "Usage: .\build-windows.ps1 [-Architecture <arch>] [-NoCGO] [-Help]"
    Write-Host "Architectures: amd64, 386, arm64"
    Write-Host "Examples:"
    Write-Host "  .\build-windows.ps1                    # Build for amd64 with CGO"
    Write-Host "  .\build-windows.ps1 -Architecture 386 # Build for 32-bit Windows"
    Write-Host "  .\build-windows.ps1 -NoCGO            # Build without CGO"
    exit 0
}

Write-Host "Building S-UI for Windows ($Architecture)..." -ForegroundColor Green
Set-Location (Resolve-Path (Join-Path $PSScriptRoot ".."))

# Check if Go is installed
try {
    $goVersion = go version 2>$null
    if ($LASTEXITCODE -ne 0) {
        throw "Go not found"
    }
    Write-Host "Go version: $goVersion" -ForegroundColor Green
} catch {
    Write-Host "Error: Go is not installed or not in PATH" -ForegroundColor Red
    Write-Host "Please install Go from https://golang.org/dl/" -ForegroundColor Yellow
    Read-Host "Press Enter to exit"
    exit 1
}

# Build backend
Write-Host "Building backend..." -ForegroundColor Yellow

# Set environment variables
$env:GOOS = "windows"
$env:GOARCH = $Architecture

if ($NoCGO) {
    $env:CGO_ENABLED = "0"
    Write-Host "Building without CGO..." -ForegroundColor Yellow
} else {
    $env:CGO_ENABLED = "1"
    Write-Host "Building with CGO..." -ForegroundColor Yellow
}

# Build command
$buildCmd = "go -C backend build -ldflags `"-w -s`" -tags `"with_quic,with_grpc,with_utls,with_acme,with_gvisor,with_tailscale`" -o ..\sui.exe ."

try {
    Invoke-Expression $buildCmd
    if ($LASTEXITCODE -ne 0) {
        if (!$NoCGO) {
            Write-Host "CGO build failed, trying without CGO..." -ForegroundColor Yellow
            $env:CGO_ENABLED = "0"
            Invoke-Expression $buildCmd
            if ($LASTEXITCODE -ne 0) {
                throw "Failed to build backend even without CGO"
            }
            Write-Host "Built without CGO (some features may be limited)" -ForegroundColor Yellow
        } else {
            throw "Failed to build backend"
        }
    } else {
        if ($env:CGO_ENABLED -eq "1") {
            Write-Host "Built successfully with CGO" -ForegroundColor Green
        } else {
            Write-Host "Built successfully without CGO" -ForegroundColor Green
        }
    }
} catch {
    Write-Host "Error: $_" -ForegroundColor Red
    Read-Host "Press Enter to exit"
    exit 1
}

Write-Host "Build completed successfully!" -ForegroundColor Green
Write-Host "Output: sui.exe" -ForegroundColor Green

# Show file info
if (Test-Path "sui.exe") {
    $fileInfo = Get-Item "sui.exe"
    Write-Host "File size: $([math]::Round($fileInfo.Length / 1MB, 2)) MB" -ForegroundColor Cyan
    Write-Host "Created: $($fileInfo.CreationTime)" -ForegroundColor Cyan
}

Read-Host "Press Enter to exit"
