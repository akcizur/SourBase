param(
    [string]$ApiKey
)

$ErrorActionPreference = "Stop"

if (-not (Get-Command docker -ErrorAction SilentlyContinue)) {
    throw "Docker Desktop není dostupný. Nainstalujte a spusťte Docker Desktop nebo spusťte node-csfd-api samostatně."
}

if ([string]::IsNullOrWhiteSpace($ApiKey)) {
    $ApiKey = Read-Host "Zadejte lokální API klíč pro Sourbase ČSFD konektor"
}
if ([string]::IsNullOrWhiteSpace($ApiKey)) {
    throw "API klíč nesmí být prázdný. Spusťte skript znovu a zadejte nový klíč."
}

Write-Host "Stahuji lokální ČSFD konektor..." -ForegroundColor Cyan
docker pull bartholomej/node-csfd-api
if ($LASTEXITCODE -ne 0) { throw "Stažení ČSFD konektoru selhalo." }

# Publish the container only on loopback; do not expose the scraper to the LAN.
docker rm -f sourbase-csfd 2>$null | Out-Null
docker run --rm --name sourbase-csfd -p "127.0.0.1:3000:3000" -e "LANGUAGE=cs" -e "API_KEY=$ApiKey" bartholomej/node-csfd-api
if ($LASTEXITCODE -ne 0) { throw "ČSFD konektor se nepodařilo spustit." }
