param([string]$DatabaseRoot=(Resolve-Path (Join-Path $PSScriptRoot '..\..\..')).Path)
$ErrorActionPreference='Stop'
$env:PGPASSFILE=Join-Path $DatabaseRoot '.pgpass'
$env:PGCLIENTENCODING='UTF8'
$OutputEncoding=[System.Text.UTF8Encoding]::new($false)
$psql='C:\Program Files\PostgreSQL\17\bin\psql.exe'
$pgArgs=@('-X','-h','127.0.0.1','-p','5432','-U','postgres','-d','imaskop','-w','-v','ON_ERROR_STOP=1')
$schema='imaskop_test_'+[DateTime]::UtcNow.ToString('yyyyMMddHHmmss')+'_'+([Guid]::NewGuid().ToString('N').Substring(0,8))
# Test kayıtları public şemasından tamamen ayrıdır. Şema ve verileri otomatik silinmez.
& $psql @pgArgs -q -c "CREATE SCHEMA $schema;"
if($LASTEXITCODE -ne 0){throw 'Test semasi olusturulamadi.'}
& $psql @pgArgs -q -c "SET search_path TO $schema,public;" -f (Join-Path $PSScriptRoot 'migrations\000001_initial_schema.up.sql') -c 'CREATE TABLE schema_migrations(version text PRIMARY KEY, applied_at timestamptz NOT NULL DEFAULT now());' -f (Join-Path $PSScriptRoot 'migrations\000002_reservation_integrity.up.sql') -c "GRANT USAGE ON SCHEMA $schema TO imaskop_app; GRANT SELECT,INSERT,UPDATE ON ALL TABLES IN SCHEMA $schema TO imaskop_app;"
if($LASTEXITCODE -ne 0){throw 'Test semasi hazirlanamadi.'}
$backend=Join-Path $PSScriptRoot '..\backend\go'
$line=Get-Content (Join-Path $backend '.env') | Where-Object {$_ -like 'DATABASE_URL=*'}
$baseURL=$line.Substring('DATABASE_URL='.Length)
$separator=if($baseURL.Contains('?')){'&'}else{'?'}
$env:TEST_DATABASE_URL=$baseURL+$separator+'search_path='+$schema+',public'
$env:GOPROXY='off';$env:GOTOOLCHAIN='local'
Set-Content (Join-Path $PSScriptRoot 'integration-last-schema.txt') $schema
Write-Output "Izole test semasi: $schema (korunacak)"
Push-Location $backend
try {go test -mod=readonly -count=1 -v ./internal/handler; if($LASTEXITCODE -ne 0){throw 'Entegrasyon testi basarisiz.'}} finally {Pop-Location;Remove-Item Env:TEST_DATABASE_URL}
