param(
    [switch]$RemoveKey
)

$ErrorActionPreference = "Stop"

if (-not (Get-Command docker -ErrorAction SilentlyContinue)) {
    throw "Docker Desktop / docker CLI není dostupný."
}

$existing = docker ps -a --filter "name=^/sourbase-csfd$" --format "{{.Names}}"
if ($existing -eq "sourbase-csfd") {
    docker stop sourbase-csfd | Out-Null
    if ($LASTEXITCODE -ne 0) { throw "ČSFD konektor se nepodařilo zastavit." }
    docker rm sourbase-csfd | Out-Null
    if ($LASTEXITCODE -ne 0) { throw "ČSFD kontejner se nepodařilo odstranit." }
}

if ($RemoveKey) {
    $keyFile = Join-Path (Join-Path $env:LOCALAPPDATA "Sourbase") "csfd-api-key.dpapi"
    if (Test-Path $keyFile) {
        Remove-Item -LiteralPath $keyFile -Force
    }
    Write-Host "ČSFD konektor je vypnutý a jeho uložený API klíč byl odstraněn." -ForegroundColor Green
}
else {
    Write-Host "ČSFD konektor je vypnutý. Šifrovaný API klíč zůstal uložen pro příští spuštění." -ForegroundColor Green
}
