param(
    [Parameter(Mandatory=$true)][string]$SecretsDirectory,
    [string]$PsqlPath='C:\Program Files\PostgreSQL\17\bin\psql.exe'
)
$ErrorActionPreference='Stop'
$env:PGCLIENTENCODING='UTF8'
$OutputEncoding=[Text.UTF8Encoding]::new($false)
$ownerFile=Join-Path $SecretsDirectory 'neon-owner.env'
$ownerURL=(Get-Content -LiteralPath $ownerFile | Where-Object {$_ -like 'DATABASE_URL=*'}).Substring(13)
$uri=[Uri]$ownerURL
if(-not $uri.Host.EndsWith('.neon.tech')){throw 'Yalnizca Neon sunucusu kabul edilir.'}
$parts=$uri.UserInfo -split ':',2
$env:PGHOST=$uri.Host;$env:PGPORT='5432'
$env:PGUSER=[Uri]::UnescapeDataString($parts[0])
$env:PGPASSWORD=[Uri]::UnescapeDataString($parts[1])
$env:PGDATABASE=$uri.AbsolutePath.Trim('/')
$env:PGSSLMODE='verify-full';$env:PGCONNECT_TIMEOUT='15'
$caFile=Join-Path $SecretsDirectory 'ca-bundle.pem'
if(Test-Path -LiteralPath $caFile){$env:PGSSLROOTCERT=$caFile}
function Scalar([string]$query){
    $result=& $PsqlPath -X -w -v ON_ERROR_STOP=1 -t -A -c $query
    if($LASTEXITCODE -ne 0){throw 'Neon sorgusu basarisiz.'}
    return ($result -join '').Trim()
}
function RunSQL([string]$sql){
    $sql | & $PsqlPath -X -w -v ON_ERROR_STOP=1 -q -f -
    if($LASTEXITCODE -ne 0){throw 'Neon SQL islemi basarisiz.'}
}
if((Scalar "SELECT count(*) FROM pg_database WHERE datname='imaskop'") -eq '0'){
    # Yeni, boş bir veritabanı oluşturulur; mevcut veritabanlarına dokunulmaz.
    RunSQL 'CREATE DATABASE imaskop;'
}
$env:PGDATABASE='imaskop'
$tableCount=[int](Scalar "SELECT count(*) FROM pg_tables WHERE schemaname='public'")
if($tableCount -eq 0){
    & $PsqlPath -X -w -v ON_ERROR_STOP=1 -f (Join-Path $PSScriptRoot 'migrations\000001_initial_schema.up.sql')
    if($LASTEXITCODE -ne 0){throw '000001 basarisiz.'}
    # Sürüm tablosu ilk migration'ın başarıyla uygulandığını kaydeder.
    RunSQL "CREATE TABLE schema_migrations(version text PRIMARY KEY,applied_at timestamptz NOT NULL DEFAULT now()); INSERT INTO schema_migrations(version) VALUES('000001');"
}elseif((Scalar "SELECT to_regclass('public.schema_migrations') IS NOT NULL") -ne 't'){
    throw 'Mevcut bulut semasi var; baseline otomatik kabul edilmedi.'
}
if((Scalar "SELECT count(*) FROM schema_migrations WHERE version='000002'") -eq '0'){
    & $PsqlPath -X -w -v ON_ERROR_STOP=1 -f (Join-Path $PSScriptRoot 'migrations\000002_reservation_integrity.up.sql')
    if($LASTEXITCODE -ne 0){throw '000002 basarisiz.'}
}
$appFile=Join-Path $SecretsDirectory 'render.env'
if((Scalar "SELECT count(*) FROM pg_roles WHERE rolname='imaskop_app'") -eq '0'){
    if(Test-Path -LiteralPath $appFile){throw 'Mevcut uygulama ayarlari korunuyor; rol elle incelenmeli.'}
    $rng=[Security.Cryptography.RandomNumberGenerator]::Create()
    $passBytes=New-Object byte[] 24;$secretBytes=New-Object byte[] 32
    $rng.GetBytes($passBytes);$rng.GetBytes($secretBytes);$rng.Dispose()
    $password=[BitConverter]::ToString($passBytes).Replace('-','').ToLower()
    $secret=[BitConverter]::ToString($secretBytes).Replace('-','').ToLower()
    # SQL ile açılan Neon rolü neon_superuser grubuna eklenmez; yönetim yetkisi verilmez.
    RunSQL "CREATE ROLE imaskop_app LOGIN PASSWORD '$password' NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS;"
    $builder=[UriBuilder]::new($uri)
    $builder.UserName='imaskop_app';$builder.Password=$password;$builder.Path='/imaskop'
    $builder.Query='sslmode=verify-full&connect_timeout=15'
    @"
APP_ENV=production
HTTP_ADDR=0.0.0.0:10000
GO_VERSION=1.26.5
DATABASE_URL=$($builder.Uri.AbsoluteUri)
AUTH_SECRET=$secret
"@ | Set-Content -LiteralPath $appFile -Encoding utf8
}elseif(-not(Test-Path -LiteralPath $appFile)){throw 'Uygulama rolu var, ancak gizli dosya yok; parola otomatik degistirilmedi.'}
# Uygulama yalnızca gerekli tablolarda okuma/ekleme/güncelleme yapabilir.
RunSQL @'
REVOKE CREATE ON SCHEMA public FROM PUBLIC;
GRANT CONNECT ON DATABASE imaskop TO imaskop_app;
GRANT USAGE ON SCHEMA public TO imaskop_app;
GRANT SELECT,INSERT,UPDATE ON users,companies,vehicles,requests,reservations TO imaskop_app;
GRANT SELECT ON schema_migrations TO imaskop_app;
'@
Write-Output 'Neon imaskop veritabani, SQL migration ve sinirli uygulama rolu hazir.'
