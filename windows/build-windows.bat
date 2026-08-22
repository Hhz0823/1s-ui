@echo off
setlocal enabledelayedexpansion

echo Building S-UI for Windows...

cd /d "%~dp0\.."

REM Check if Go is installed
go version >nul 2>&1
if errorlevel 1 (
    echo Error: Go is not installed or not in PATH
    echo Please install Go from https://golang.org/dl/
    pause
    exit /b 1
)

echo Building backend...
set CGO_ENABLED=1
set GOOS=windows
set GOARCH=amd64

REM Try to build with CGO first
go -C backend build -ldflags "-w -s" -tags "with_quic,with_grpc,with_utls,with_acme,with_gvisor,with_tailscale" -o ..\sui.exe .
if errorlevel 1 (
    echo Warning: CGO build failed, trying without CGO...
    set CGO_ENABLED=0
    go -C backend build -ldflags "-w -s" -tags "with_quic,with_grpc,with_utls,with_acme,with_gvisor,with_tailscale" -o ..\sui.exe .
    if errorlevel 1 (
        echo Error: Failed to build backend
        pause
        exit /b 1
    )
    echo Built without CGO (some features may be limited)
) else (
    echo Built with CGO
)

echo Build completed successfully!
echo Output: sui.exe
pause
