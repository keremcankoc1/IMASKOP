$ErrorActionPreference = 'Stop'
# Paket ve Go sürümü indirilmesini engeller; bu betik yalnızca mevcut kurulumu kullanır.
$env:GOPROXY = 'off'
$env:GOTOOLCHAIN = 'local'
Push-Location $PSScriptRoot
try {
    go run -mod=readonly ./cmd/api -env .env
    if ($LASTEXITCODE -ne 0) { throw 'API baslatilamadi.' }
} finally { Pop-Location }
