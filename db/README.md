# PostgreSQL ve migration rehberi

Veritabanı: `imaskop`, PostgreSQL 17. Veriler çalışma klasöründeki `postgres-data` içinde korunur. Klasör yeniden oluşturulmadı ve taşınmadı. Kök `.env`/`.pgpass` yönetici içindir; backend kendi `.env` dosyasındaki `imaskop_app` rolünü kullanır.

## Tek migration kaynağı

Bu repodaki `db/migrations` dosyaları tek çalıştırılan kaynaktır. Çalışma klasörünün kökündeki önceki `000001*` dosyaları tarihsel kopyalardır; bunları ayrıca çalıştırmayın/düzenlemeyin. Backend tablo oluşturmaz; GORM/AutoMigrate yoktur.

- `000001_initial_schema.up.sql`: mevcut ilk şema. Bu makinede beş tablo zaten vardı; yeniden uygulanmadı. UUID, isimler, CHECK, FK, tarih aralıkları, updated_at tetikleyicileri ve kısmi aktif rezervasyon indeksi incelendi.
- `000002_reservation_integrity.up.sql`: araç-şirket, talep-müşteri, talep-rezervasyon birleşik FK'ları ve aktif rezervasyon doğrulama tetikleyicisi. Verileri silmez.
- `schema_migrations`: hangi sürümlerin uygulandığını kaydeder. İlk sürüm, mevcut şema denetlendikten sonra baseline olarak kayda alındı.

Repo kökünde:

```powershell
# Mevcut şemada yalnızca ilk kayıt kurulumu sırasında:
.\db\migrate.ps1 -BaselineExisting
# Sonraki çalıştırmalarda uygulanmış migration'lar atlanır:
.\db\migrate.ps1
# Yerel sınırlı uygulama rolünü/ayarlarını hazırlayın; mevcut .env üzerine yazmaz:
.\db\setup-local.ps1
```

Betikler yönetici `.pgpass` dosyasını kullanır ve Windows'taki mevcut PostgreSQL 17 kurulum yoluna göre hazırlanmıştır. Program/paket kurmaz. `down.sql` çalıştırılmadı; veri silen geri alma işlemleri ayrıca kullanıcı onayı gerektirir.

## SQL terimleri

- `UUID PRIMARY KEY`: her kaydın tekil kimliğidir; `gen_random_uuid()` sunucuda üretir.
- `FOREIGN KEY`: başka tablodaki kaydın varlığını doğrular. Birleşik FK, örneğin araç ve şirketin birlikte doğru olmasını sağlar.
- `CHECK`: izin verilen değerleri, koordinatları ve tarih sırasını denetler.
- `UNIQUE INDEX ... WHERE`: yalnızca PENDING/CONFIRMED kayıtları kapsayan kısmi benzersizliktir; bir talebin aynı anda tek aktif rezervasyonu olabilir.
- `btree_gist`: UUID eşitliği ile tarih aralığı kontrolünü aynı GiST indeksinde kullanmayı sağlayan eklentidir.
- `EXCLUDE USING gist`: aynı araç ve çakışan `tstzrange` aralığına sahip aktif iki kaydı PostgreSQL düzeyinde engeller. `&&` aralıkların çakışma işlemcisidir. `[)` başlangıç dahil, bitiş hariç anlamına gelir.
- `TIMESTAMPTZ`: zaman dilimi farklarını doğru karşılaştıran zaman türüdür.
- `FOR UPDATE`: satırı transaction sonuna kadar kilitler; iki işlem aynı kaydı eşzamanlı değiştiremez.
- `TRIGGER`: INSERT/UPDATE sırasında otomatik çalışan fonksiyondur. updated_at, doğrudan SQL ile güncellense bile yenilenir.
- `$1`, `$2`: Go sorgularındaki parametre yer tutucularıdır; kullanıcı metni SQL'e birleştirilmez.

## Uygulama rolü

`imaskop_app` superuser değildir; veritabanı, rol veya public şemasında tablo oluşturamaz. Beş iş tablosunda SELECT/INSERT/UPDATE ve migration tablosunda SELECT yetkisi vardır. DELETE/TRUNCATE/DDL verilmez. Şirket bazlı yetki API service katmanındadır; veritabanına doğrudan bağlanmak şirket kullanıcılarına sunulan bir özellik değildir.

## Test verileri

`test-integration.ps1` yeni `imaskop_test_*` şemasında aynı SQL migration'larını uygular, yalnızca o şemaya bağlanan API testlerini çalıştırır. Veritabanının adı yine `imaskop` kalır. `public` tablolarının boşluğu ayrıca kontrol edilir. Test şemaları hata ayıklamak için korunur; hiçbir otomatik DROP/TRUNCATE çalıştırılmaz. `integration-last-schema.txt` son şemayı yerel olarak kaydeder ve Git dışında tutulur.
