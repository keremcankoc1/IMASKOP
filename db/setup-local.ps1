param([string]$DatabaseRoot = (Resolve-Path (Join-Path $PSScriptRoot '..\..\..')).Path)
$ErrorActionPreference='Stop'
$env:PGCLIENTENCODING='UTF8'
$OutputEncoding=[System.Text.UTF8Encoding]::new($false)
$psql='C:\Program Files\PostgreSQL\17\bin\psql.exe'
$env:PGPASSFILE=Join-Path $DatabaseRoot '.pgpass'
$pgArgs=@('-X','-h','127.0.0.1','-p','5432','-U','postgres','-d','imaskop','-w','-v','ON_ERROR_STOP=1')
$backend=Join-Path $PSScriptRoot '..\backend\go'
$envFile=Join-Path $backend '.env'
$exists=(& $psql @pgArgs -t -A -c "SELECT count(*) FROM pg_roles WHERE rolname='imaskop_app'").Trim()
if($LASTEXITCODE -ne 0){throw 'Rol sorgulanamadi.'}
if($exists -eq '0') {
    if(Test-Path -LiteralPath $envFile){throw 'Mevcut .env korunuyor; rol/parola eslestirmesini elle kontrol edin.'}
    $rng=[System.Security.Cryptography.RandomNumberGenerator]::Create()
    $passwordBytes=New-Object byte[] 24
    $secretBytes=New-Object byte[] 32
    $rng.GetBytes($passwordBytes)
    $rng.GetBytes($secretBytes)
    $rng.Dispose()
    $taskPassword=[BitConverter]::ToString($passwordBytes).Replace('-','').ToLower()
    $taskSecret=[BitConverter]::ToString($secretBytes).Replace('-','').ToLower()
    # Uygulama rolü sunucu veya veritabanı yönetemez, rol oluşturamaz ve RLS kurallarını atlayamaz.
    $roleSql="CREATE ROLE imaskop_app LOGIN PASSWORD '$taskPassword' NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS;"
    $roleSql | & $psql @pgArgs -q -f -
    if($LASTEXITCODE -ne 0){throw 'Uygulama rolu olusturulamadi.'}
    @"
APP_ENV=development
HTTP_ADDR=127.0.0.1:8080
DATABASE_URL=postgresql://imaskop_app:${taskPassword}@127.0.0.1:5432/imaskop?sslmode=disable
AUTH_SECRET=$taskSecret
CORS_ORIGIN=http://localhost:5000
"@ | Set-Content -LiteralPath $envFile -Encoding utf8
} elseif(-not(Test-Path -LiteralPath $envFile)) {throw 'Rol zaten mevcut; .env baglanti bilgileri elle hazirlanmali.'}
$grantSql=@'
-- PUBLIC tüm rolleri temsil eder. Uygulama kullanıcılarının tablo oluşturmasını kapatırız.
REVOKE CREATE ON SCHEMA public FROM PUBLIC;
-- CONNECT bağlantıya, USAGE ise şemadaki nesnelere erişmeye izin verir.
GRANT CONNECT ON DATABASE imaskop TO imaskop_app;
GRANT USAGE ON SCHEMA public TO imaskop_app;
-- Yalnızca gerekli işlemler açılır; DELETE, TRUNCATE ve DDL yetkisi verilmez.
GRANT SELECT, INSERT, UPDATE ON users,companies,vehicles,requests,reservations TO imaskop_app;
GRANT SELECT ON schema_migrations TO imaskop_app;
'@
$grantSql | & $psql @pgArgs -q -f -
if($LASTEXITCODE -ne 0){throw 'Uygulama yetkileri verilemedi.'}
Write-Output 'imaskop_app hazir; gizli baglanti bilgileri backend/go/.env dosyasinda.'
