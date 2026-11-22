$ErrorActionPreference = "Stop"

if (-not (Get-Command docker -ErrorAction SilentlyContinue)) {
    throw "Docker Desktop / docker CLI není dostupný."
}

$existing = docker ps -a --filter "name=^/sourbase-csfd$" --format "{{.Names}}"
if ($existing -eq "sourbase-csfd") {
    docker stop sourbase-csfd | Out-Null
    if ($LASTEXITCODE -ne 0) { throw "ČSFD konektor se nepodařilo zastavit." }
    docker rm sourbase-csfd | Out-Null
}
Write-Host "ČSFD konektor je vypnutý. Šifrovaný API klíč zůstal uložen pro příští spuštění." -ForegroundColor Green
