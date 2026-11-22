param(
    [string]$ApiKey
)

$ErrorActionPreference = "Stop"

if (-not (Get-Command docker -ErrorAction SilentlyContinue)) {
    throw "Docker Desktop / docker CLI není dostupný. Nainstalujte a spusťte Docker Desktop."
}

$stateDir = Join-Path $env:LOCALAPPDATA "Sourbase"
$keyFile = Join-Path $stateDir "csfd-api-key.dpapi"
New-Item -ItemType Directory -Force -Path $stateDir | Out-Null

# Reuse the current-user DPAPI-protected key across service restarts.
if ([string]::IsNullOrWhiteSpace($ApiKey) -and (Test-Path $keyFile)) {
    $encryptedKey = [IO.File]::ReadAllText($keyFile)
    $secureKey = ConvertTo-SecureString -String $encryptedKey
    $pointer = [Runtime.InteropServices.Marshal]::SecureStringToBSTR($secureKey)
    try {
        $ApiKey = [Runtime.InteropServices.Marshal]::PtrToStringBSTR($pointer)
    }
    finally {
        [Runtime.InteropServices.Marshal]::ZeroFreeBSTR($pointer)
    }
}

if ([string]::IsNullOrWhiteSpace($ApiKey)) {
    $bytes = New-Object byte[] 32
    $rng = [Security.Cryptography.RandomNumberGenerator]::Create()
    try {
        $rng.GetBytes($bytes)
    }
    finally {
        $rng.Dispose()
    }
    $ApiKey = ([BitConverter]::ToString($bytes)).Replace("-", "").ToLowerInvariant()
}

$secureKey = ConvertTo-SecureString -String $ApiKey -AsPlainText -Force
$encryptedKey = ConvertFrom-SecureString -SecureString $secureKey
[IO.File]::WriteAllText($keyFile, $encryptedKey, [Text.Encoding]::ASCII)

Write-Host "Stahuji ČSFD konektor..." -ForegroundColor Cyan
docker pull bartholomej/node-csfd-api
if ($LASTEXITCODE -ne 0) { throw "Stažení ČSFD konektoru selhalo." }

$existing = docker ps -a --filter "name=^/sourbase-csfd$" --format "{{.Names}}"
if ($existing -eq "sourbase-csfd") {
    docker rm -f sourbase-csfd | Out-Null
    if ($LASTEXITCODE -ne 0) { throw "Předchozí ČSFD kontejner se nepodařilo odstranit." }
}

# Bind the published port only to loopback; never expose it to the LAN.
$previousApiKey = $env:API_KEY
try {
    $env:API_KEY = $ApiKey
    docker run --detach --name sourbase-csfd --restart unless-stopped --publish "127.0.0.1:3000:3000" --env LANGUAGE=cs --env API_KEY bartholomej/node-csfd-api
    if ($LASTEXITCODE -ne 0) { throw "ČSFD konektor se nepodařilo spustit." }
}
finally {
    if ($null -eq $previousApiKey) {
        Remove-Item Env:API_KEY -ErrorAction SilentlyContinue
    }
    else {
        $env:API_KEY = $previousApiKey
    }
}

Write-Host ""
Write-Host "ČSFD API běží na http://127.0.0.1:3000" -ForegroundColor Green
Write-Host "API klíč pro Sourbase / Nastavení / ČSFD:" -ForegroundColor Yellow
Write-Host $ApiKey
Write-Host ""
Write-Host "Klíč je uložen šifrovaně pro tento účet Windows: $keyFile"
Write-Host "Pro vypnutí spusťte .\Stop-Sourbase-CSFD.ps1"
