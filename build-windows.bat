@echo off
setlocal
cd /d "%~dp0"
set GOOS=windows
set GOARCH=amd64
set CGO_ENABLED=0
go test ./... || exit /b 1
go vet ./... || exit /b 1
go build -trimpath -ldflags="-s -w -H=windowsgui" -o "Sourbase.exe" .
if errorlevel 1 exit /b 1
echo Build complete: Sourbase.exe
