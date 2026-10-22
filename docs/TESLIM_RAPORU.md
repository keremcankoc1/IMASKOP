# İMASKOP teslim raporu — 22 Ekim 2026

Öncelikli akış PostgreSQL'e bağlı gerçek HTTP API testinde çalıştı: şirket hesabı → şirket → araç → AFAD talebi → uygun araç → eşleştirme → rezervasyon → şirket onayı. Test kayıtları ayrı şemalarda tutuldu.

## 1. Önceden hazır olanlar

- Gerçek repo yolu `Yeni klasör/IMASKOP`; Go modülü `backend/go` içindeydi. Görev metnindeki `imaskop-backend` klasörü yoktu.
- Beş Go modeli vardı: User, Company, Vehicle, Request, Reservation. `cmd/api/main.go` yalnızca boş `main()` içeriyordu.
- Repository/service/handler/router/database bağlantısı, çalışan endpoint veya AutoMigrate mekanizması yoktu.
- Git çalışma ağacı başlangıçta temizdi; mevcut Git geçmişi korundu.
- PostgreSQL 17, pgAdmin 4, Go 1.26.5 ve Git zaten kuruluydu. Yeni program kurulmadı.
- `imaskop.public` içinde beş iş tablosu, btree_gist, updated_at tetikleyicileri, tarih çakışması constraint'i ve bir talebe tek aktif rezervasyon indeksi zaten vardı. Başlangıçta tüm tablolar boştu.

## 2. Eksikler ve uyumsuzluklar

- Reservation Go modelinde start_at/end_at yoktu; SQL'de iki alan zorunluydu. Modele eklendi.
- Başlangıç SQL'i UUID, isimler, araç türleri, tarih alanları, updated_at ve aktif talep benzersizliği bakımından uygundu.
- Rezervasyonun company_id alanının aracın şirketiyle aynı olması garanti edilmiyordu. Aynı şekilde müşteri/talep ve talep/rezervasyon bağlantıları birleşik ilişkilerle doğrulanmıyordu.
- Uygulama bağlantısı, sınırlı DB rolü, giriş, kayıt ve tüm HTTP iş akışı eksikti.
- Kullanılmayan Gin/MongoDB/QUIC ve benzeri modüller listeliydi; gerçek uygulama kodu bunları kullanmıyordu. Gereksiz indirmeleri önlemek için listeden çıkarıldı; standart Go HTTP router'ı kullanıldı.

## 3. Eklenen/değişen dosyalar

| Yol | Değişiklik |
|---|---|
| backend/go/cmd/api/main.go | Sunucu, sağlık/DB kontrolü, AFAD/ADMIN hesabı hazırlama, kapanış yönetimi |
| backend/go/internal/model/reservation.go | StartAt ve EndAt |
| backend/go/internal/config/config.go | Ortam ve isteğe bağlı .env yükleme; ortam değişkenleri öncelikli |
| backend/go/internal/database/database.go | pgx bağlantısı, timeout/havuz, imaskop ve non-superuser kontrolü |
| backend/go/internal/repository/store.go | Parametreli sorgu, JSON, kullanıcı ve transaction yardımcıları |
| backend/go/internal/service/auth.go | bcrypt, imzalı token, rol denetimi |
| backend/go/internal/service/fleet.go | Şirket/araç, sahiplik, konum ve filtreler |
| backend/go/internal/service/bookings.go | Talep, adaylar, eşleştirme, rezervasyon yaşam döngüsü |
| backend/go/internal/handler/api.go | Gerçek HTTP endpoint'leri, JSON/CORS/hata/kimlik doğrulama |
| backend/go/internal/handler/integration_test.go | Ayrı şemada gerçek HTTP/PostgreSQL testleri |
| backend/go/go.mod, go.sum | Gerekli bağımlılıklar; mevcut checksum kayıtları korundu |
| backend/go/.env.example, .env.production.example | Gizli bilgi içermeyen geliştirme/üretim şablonları |
| backend/go/.env | Yerel gerçek bağlantı ve anahtar; Git dışında |
| backend/go/start-api.ps1 | Paket indirmeden yerel çalıştırma |
| db/migrations/000001* | İlk SQL'in repo içindeki tek çalıştırılan kaynağı; Türkçe açıklamalar |
| db/migrations/000002_reservation_integrity.up.sql | Veri silmeden ilişki ve rezervasyon denetimi |
| db/migrate.ps1 | Mevcut şemayı baseline yapma ve sürümlü migration |
| db/setup-local.ps1 | Sınırlı uygulama rolü, yetkiler ve yerel gizli ayarlar |
| db/test-integration.ps1 | İzole test şeması hazırlama ve test çalıştırma |
| README.md, db/README.md, docs/API.md | Çalıştırma, PostgreSQL ve frontend belgeleri |
| .gitignore | Yerel sırlar, derlenmiş çıktı ve test şema notunu dışlama |

Mevcut diğer dört modelin içeriği ve frontend dosyaları değiştirilmedi. Git commit/push yapılmadı.

## 4. PostgreSQL bağlantısı

Backend `127.0.0.1:5432/imaskop` adresine `imaskop_app` rolüyle bağlanıyor. Bağlantı kontrolü başarılı. Bu rol superuser, rol/veritabanı oluşturucu veya public şeması oluşturucusu değil. İş tablolarında gerekli SELECT/INSERT/UPDATE yetkisi var; DELETE/TRUNCATE/DDL verilmedi. Uygulama açılırken superuser bağlantısı reddediliyor.

Yönetici `.env` ve `.pgpass` dosyalarındaki sırlar çıktıya yazılmadı. Backend sırları kendi Git dışı `.env` dosyasına yazıldı. Ortam değişkenleri .env dosyasından önce gelir. Üretim ayarları ayrı örnekle belgelendi; üretime dağıtım yapılmadı.

## 5. Migration durumu

`public.schema_migrations` içinde 000001 ve 000002 kayıtları var. Mevcut ilk şema incelenerek baseline olarak kaydedildi; public tablolarına ilk migration tekrar uygulanmadı. 000002 başarıyla uygulandı. Tekrar çalıştırma denendi; mevcut sürüm atlandı.

Araç/şirket, talep/müşteri ve talep/rezervasyon eşleşmeleri birleşik FK'larla korunuyor. Aktif rezervasyon tetikleyicisi araç türünü, kapasiteyi, operasyon durumunu, eşleşmiş aracı ve tarihleri doğruluyor. Mevcut GiST exclusion constraint'i PENDING/CONFIRMED rezervasyonlarda çakışmayı PostgreSQL seviyesinde engelliyor.

Hiçbir down migration, DROP DATABASE, DROP TABLE, TRUNCATE veya veri silme çalıştırılmadı. postgres-data taşınmadı/sıfırlanmadı. Kök SQL dosyaları tarihsel kopya olarak korundu; çalıştırılan tek kaynak repo içindeki db/migrations.

## 6. Çalışan ve sınırlı işlevler

Çalışanlar: şirket kaydı, giriş, parola hashleme, rol ve sahiplik kontrolü, şirket oluşturma/güncelleme/konum, araç oluşturma/listeleme/düzenleme/konum, tür-kapasite-tarih filtresi, AFAD talebi, adaylar, eşleştirme, rezervasyon oluşturma/onay/iptal/tamamlama ve okuma endpoint'leri.

PENDING rezervasyonlar da müsaitliği bloke eder. Bu mevcut SQL davranışıdır; korundu. Bekleyen rezervasyon için otomatik süre sonu yok; iş kuralı olarak ayrıca kararlaştırılmalı.

Henüz bulunmayanlar: yakınlığa göre otomatik eşleştirme/harita/rota, otomatik ücret hesabı, ödeme, refresh/logout/parola sıfırlama, şirket çalışanı daveti, liste sayfalama ve otomatik PENDING zaman aşımı. total_price NULL kalır. Eşleştirme, tarih/tür/kapasiteye göre uygun araç seçimini sağlar; mesafe sıralaması yapmaz. Liste sınırı 200'dür.

Gerçek AFAD hesabı açılmadı; gerçek e-posta ve parola kullanıcı tarafından README'deki güvenli yerel komutla verilmeli. Test AFAD hesabı izole şemadadır. Üretim dağıtımı ve gerçek .NET uygulamasından çağrı henüz test edilmedi.

## 7. Testler

Son entegrasyon çalıştırması 18 alt kontrolle başarılı:

- Halka açık kayıttan ADMIN/CUSTOMER yetkisi alınamaması.
- Eksik/bozuk token ve yanlış parola reddi.
- Araç kaydının gerçekten PostgreSQL'e yazılması.
- Şirket sahipliği, koordinat ve filtre doğrulaması.
- PENDING rezervasyonun adaylardan elenmesi; tür/kapasite filtresi; başka şirketin filosunun listelenmemesi.
- Çakışan ikinci rezervasyonun reddi.
- Başka şirketin/müşterinin onay verememesi ve başka şirketin rezervasyon ayrıntısını okuyamaması.
- Şirket onayı ve confirmed_at.
- Bitiş anında başlayan bitişik rezervasyonun kabulü.
- Yanlış şirketle doğrudan SQL rezervasyonunun reddi.
- Araç/rezervasyonların API'den okunması.
- İptalle müsaitliğin açılması.
- Onay/tamamlama durum sırası ve geçersiz geçişlerin reddi.
- Aynı anda gelen iki HTTP rezervasyonundan tam birinin 201, diğerinin 409 alması.
- Eşzamanlı doğrudan SQL işlemlerinde PostgreSQL'in 23P01 exclusion hatası vermesi.
- updated_at ve sınırlı rol kontrolü.
- Şirket/konum güncellemesi, aktif rezervasyonlu aracın değiştirilememesi, boş aracın düzenlenmesi.
- CORS preflight.

`go test -mod=readonly ./...`, `go vet -mod=readonly ./...` ve API derlemesi başarılı. Normal go test komutu TEST_DATABASE_URL yokken entegrasyonu atlar; yukarıdaki 18 kontrol ayrıca test-integration.ps1 ile gerçek DB'ye bağlı çalıştırıldı.

İlk entegrasyon denemesinde testin ikinci rezervasyonu bitişik üçüncü rezervasyona bir dakika taşıyordu; API doğru biçimde çakışma döndürdü. Test aralığı düzeltildi ve son çalıştırmada başarısız kontrol kalmadı. Ortamın Türkçe karakter/istemci kodlaması sorunu UTF-8 stdin kullanılarak düzeltildi.

Testler üç ayrı imaskop_test_* şemasında çalıştırıldı; ilk kısmi deneme dahil hepsi korundu. Son şema integration-last-schema.txt dosyasında yerel olarak kayıtlı. Son kontrolde public.users/companies/vehicles/requests/reservations tablolarının her birinde kayıt sayısı 0.

## 8. Kullanıcının PostgreSQL tarafında yapacakları

Şu an zorunlu migration işlemi kalmadı. Bilgisayar yeniden açıldığında kök start-db.ps1 ile sunucuyu, ardından backend/go/start-api.ps1 ile API'yi başlatın. Gerçek AFAD hesabını hazırlayın. pgAdmin'de public şemasını kullanın; imaskop_test_* şemaları test verisidir. Bunları temizlemek isterseniz önce hangi şemaların silineceğini ayrıca onaylayın.

## 9. Go geliştiricisinin devralacakları

Öncelikli akış hazır ve yerel olarak doğrulandı. Kodu inceleyip ekiple API sözleşmesini sabitleyin. Ürün kararları: PENDING süresi, tamamlama zamanı kısıtı, ücret/mesafe hesabı, liste sayfalama, şirket çalışan yetkileri ve token yenileme. Üretimde HTTPS, güvenli DB TLS, sır yönetimi ve dağıtım ayarlarını yapılandırın. Migration'ı yalnızca SQL mekanizmasıyla yürütün; AutoMigrate eklemeyin. Mevcut başlangıçta boş main için standart net/http seçildi; çalışan eski router değiştirilmedi.

## 10. .NET geliştiricisinin bilgileri

`docs/API.md` gerçek endpoint tablosunu, rol kurallarını, gövdeleri, yanıtları, hata kodlarını ve örnek akışı içerir. Base URL http://127.0.0.1:8080; girişten alınan token Authorization: Bearer başlığındadır. Koordinat/tarih/nullable alanlara dikkat edin. Tarayıcı uygulaması için CORS_ORIGIN gerçek frontend origin'ine ayarlanmalı. PostgreSQL parolası veya AUTH_SECRET frontend'e gönderilmez. Mevcut .NET dosyaları değiştirilmedi.
