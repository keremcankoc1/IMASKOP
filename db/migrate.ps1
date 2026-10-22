param(
    [string]$DatabaseRoot = (Resolve-Path (Join-Path $PSScriptRoot '..\..\..')).Path,
    [switch]$BaselineExisting
)
$ErrorActionPreference = 'Stop'
$env:PGCLIENTENCODING = 'UTF8'
$OutputEncoding = [System.Text.UTF8Encoding]::new($false)
$psql = 'C:\Program Files\PostgreSQL\17\bin\psql.exe'
$env:PGPASSFILE = Join-Path $DatabaseRoot '.pgpass'
$pgArgs = @('-X','-h','127.0.0.1','-p','5432','-U','postgres','-d','imaskop','-w','-v','ON_ERROR_STOP=1')
function SqlScalar([string]$Query) {
    $result = & $psql @pgArgs -t -A -c $Query
    if ($LASTEXITCODE -ne 0) { throw 'Veritabani sorgusu basarisiz.' }
    return ($result -join '').Trim()
}
if ((SqlScalar 'SELECT current_database()') -ne 'imaskop') { throw 'Yanlis veritabani.' }
$tableCount = [int](SqlScalar "SELECT count(*) FROM pg_tables WHERE schemaname='public' AND tablename IN ('users','companies','vehicles','requests','reservations')")
if ($tableCount -ne 0 -and $tableCount -ne 5) { throw 'Kismi sema bulundu; migration otomatik uygulanmadi.' }
$ledger = SqlScalar "SELECT to_regclass('public.schema_migrations') IS NOT NULL"
if ($tableCount -eq 5 -and $ledger -eq 'f') {
    if (-not $BaselineExisting) { throw 'Mevcut ilk semayi dogruladiktan sonra -BaselineExisting kullanin.' }
    # Kontrol edilen ilk sürümün temel kuralları; eksikse sessizce geçmiş kaydı üretilmez.
    $checks = SqlScalar "SELECT count(*) FROM pg_constraint WHERE conname IN ('reservations_vehicle_no_overlap','reservations_time_valid','requests_time_valid','requests_reservation_id_fkey','users_company_id_fkey') AND connamespace='public'::regnamespace"
    $activeIndex = SqlScalar "SELECT to_regclass('public.reservations_one_active_per_request') IS NOT NULL"
    $triggers = SqlScalar "SELECT count(*) FROM information_schema.triggers WHERE trigger_schema='public' AND trigger_name IN ('users_set_updated_at','companies_set_updated_at','vehicles_set_updated_at','requests_set_updated_at','reservations_set_updated_at')"
    if ($checks -ne '5' -or $activeIndex -ne 't' -or $triggers -ne '5') { throw 'Ilk surum kurallari eksik. Elle inceleme gerekiyor.' }
}
if ($tableCount -eq 0) {
    & $psql @pgArgs -f (Join-Path $PSScriptRoot 'migrations\000001_initial_schema.up.sql')
    if ($LASTEXITCODE -ne 0) { throw 'Ilk migration basarisiz.' }
}
$bootstrap = @'
-- Sürüm tablosu hangi SQL dosyalarının uygulandığını kaydeder; uygulama tablo oluşturmaz.
CREATE TABLE IF NOT EXISTS public.schema_migrations (
    version TEXT PRIMARY KEY,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- Mevcut ve doğrulanan ilk şema tekrar çalıştırılmadan kayda alınır.
INSERT INTO public.schema_migrations(version) VALUES ('000001') ON CONFLICT DO NOTHING;
'@
$bootstrap | & $psql @pgArgs -q -f -
if ($LASTEXITCODE -ne 0) { throw 'Surum tablosu hazirlanamadi.' }
if ((SqlScalar "SELECT count(*) FROM public.schema_migrations WHERE version='000002'") -eq '0') {
    & $psql @pgArgs -f (Join-Path $PSScriptRoot 'migrations\000002_reservation_integrity.up.sql')
    if ($LASTEXITCODE -ne 0) { throw '000002 basarisiz; veriler korundu.' }
}
Write-Output 'Migration surumleri 000001 ve 000002 hazir.'
