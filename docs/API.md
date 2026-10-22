# .NET frontend için gerçek API sözleşmesi

Yerel base URL: `http://127.0.0.1:8080`. API yolu: `/api`. Sağlık kontrolü: `GET /health`.

İstek gövdelerinde `Content-Type: application/json` kullanın. Alan adları mevcut Go modellerindeki gibi `snake_case` biçimindedir. Koordinat alanları şirket/araçta `lat`, `lon`; taleplerde `pick_up_lat`, `pick_up_lon`, `dropoff_lat`, `dropoff_lon` şeklindedir. UUID'ler JSON string, zamanlar UTC veya ofsetli RFC3339 string olmalıdır (`2026-11-01T10:00:00+03:00`).

## Kimlik doğrulama

`POST /api/auth/register` yalnızca şirket hesabı oluşturur:

```json
{"email":"sirket@example.com","password":"GUCLU_BIR_PAROLA","name":"Şirket Yetkilisi","role":"COMPANY"}
```

`role` gönderilmezse COMPANY kabul edilir; ADMIN veya CUSTOMER gönderilirse 403 döner. Parola 12–72 UTF-8 bayt olmalıdır. E-posta normalize edilir; büyük/küçük harf farkıyla ikinci kayıt açılamaz. Parola bcrypt ile saklanır ve hash yanıtlarda bulunmaz.

Kayıt yanıtı (201):

```json
{"id":"11111111-1111-4111-8111-111111111111","email":"sirket@example.com","name":"Şirket Yetkilisi","role":"COMPANY","created_at":"2026-10-22T14:00:00Z","updated_at":"2026-10-22T14:00:00Z"}
```

`POST /api/auth/login`:

```json
{"email":"sirket@example.com","password":"GUCLU_BIR_PAROLA"}
```

Yanıt (200):

```json
{"access_token":"SUNUCUNUN_VERDIGI_TOKEN","token_type":"Bearer","expires_at":"2026-10-22T22:00:00Z","user":{"id":"11111111-1111-4111-8111-111111111111","email":"sirket@example.com","name":"Şirket Yetkilisi","role":"COMPANY","company_id":"22222222-2222-4222-8222-222222222222","created_at":"2026-10-22T14:00:00Z","updated_at":"2026-10-22T14:00:00Z"}}
```

Sonraki isteklerde `Authorization: Bearer SUNUCUNUN_VERDIGI_TOKEN` başlığı kullanılır. Token 8 saat geçerlidir; standart JWT değildir, HMAC-SHA256 ile imzalanmış sunucu token'ıdır. Frontend token içeriğini çözümlememeli; kullanıcı/rol için login yanıtını veya `GET /api/me` kullanmalıdır. Süre dolunca yeniden giriş gerekir. Logout/refresh/password-reset endpoint'i henüz yoktur. Şirketi yeni oluşturduktan sonra `/api/me` yenilenmelidir.

AFAD hesapları CUSTOMER rolündedir ve halka açık kayıt endpoint'inden açılamaz. Yetkili kişi README'deki hesap hazırlama komutunu kullanır. Giriş/kayıt toplamı IP başına dakikada 30 istekle sınırlıdır. Proxy başlıkları otomatik güvenilir kabul edilmez.

## Çalışan endpoint'ler

| Metot ve yol | Yetki | Gövde / davranış |
|---|---|---|
| GET `/health` | Açık | DB bağlantı sağlığı |
| POST `/api/auth/register` | Açık | Yukarıdaki kayıt gövdesi; 201 User |
| POST `/api/auth/login` | Açık | email, password; 200 token + User |
| GET `/api/me` | Giriş | 200 User |
| POST `/api/companies` | COMPANY / ADMIN | CompanyInput; 201 Company |
| PUT `/api/companies/{id}` | Şirket sahibi / ADMIN | CompanyInput; 200 Company |
| PATCH `/api/companies/{id}/location` | Şirket sahibi / ADMIN | lat, lon; 200 Company |
| POST `/api/vehicles` | Şirket sahibi / ADMIN | VehicleInput; 201 Vehicle |
| GET `/api/vehicles` | Giriş | Filtreli Vehicle dizisi |
| GET `/api/vehicles/{id}` | AFAD / ilgili şirket / ADMIN | 200 Vehicle |
| PUT `/api/vehicles/{id}` | Şirket sahibi / ADMIN | VehicleInput; 200 Vehicle |
| PATCH `/api/vehicles/{id}/location` | Şirket sahibi / ADMIN | lat, lon; 200 Vehicle |
| POST `/api/requests` | CUSTOMER / ADMIN | RequestInput; 201 Request |
| GET `/api/requests` | CUSTOMER / ADMIN | Kendi talepleri; ADMIN tümü; Request dizisi |
| GET `/api/requests/{id}` | Talep sahibi / ADMIN | 200 Request |
| GET `/api/requests/{id}/candidates` | Talep sahibi / ADMIN | Uygun Vehicle dizisi; opsiyonel end_at sorgusu |
| POST `/api/requests/{id}/match` | Talep sahibi / ADMIN | vehicle_id; 200 Request |
| POST `/api/reservations` | Talep sahibi / ADMIN | request_id, vehicle_id, opsiyonel end_at; 201 Reservation |
| GET `/api/reservations` | Giriş | Yalnızca rolün görebildiği Reservation dizisi |
| GET `/api/reservations/{id}` | Müşteri / ilgili şirket / ADMIN | 200 Reservation |
| POST `/api/reservations/{id}/confirm` | İlgili şirket / ADMIN | Gövde yok; PENDING → CONFIRMED |
| POST `/api/reservations/{id}/cancel` | Müşteri / ilgili şirket / ADMIN | Gövde yok; PENDING veya CONFIRMED → CANCELLED |
| POST `/api/reservations/{id}/complete` | İlgili şirket / ADMIN | Gövde yok; CONFIRMED → COMPLETED |

Şirket yetkisi `companies.owner_id` üzerinden kontrol edilir. Başka şirketin araç/rezervasyonunu değiştirme ve görme yetkisi yoktur. Ek şirket çalışanı davet etme mekanizması henüz yoktur. Liste yanıtları doğrudan JSON dizileridir; en yeni 200 kaydı getirir. Sayfalama henüz yoktur.

## Şirket ve araç gövdeleri

CompanyInput (`POST` ve tam güncelleme `PUT`):

```json
{"name":"Örnek İş Makineleri","tax_number":"1234567890","phone":"03120000000","address":"Ankara","lat":39.93,"lon":32.86}
```

`name`, `tax_number` zorunlu; diğerleri isteğe bağlıdır. Konum gönderilirse iki koordinat birlikte gönderilir. Yanıt ek olarak `id`, `owner_id`, `created_at`, `updated_at` içerir. Her kullanıcı bu akışta bir şirket açabilir. Şirket oluşturulduğunda User.company_id güncellenir.

VehicleInput:

```json
{"company_id":"22222222-2222-4222-8222-222222222222","plate":"06ABC123","type":"TRUCK","brand":"Örnek","model":"Kamyon","year":2024,"passengers":3,"cargo_kg":10000,"lat":39.93,"lon":32.86,"price_per_km":25,"status":"AVAILABLE"}
```

Oluştururken `company_id`, `plate`, `type` zorunlu. Araç türleri: TRUCK, EXCAVATOR, BACKHOE. İstemci AVAILABLE, IN_USE veya MAINTENANCE durumunu seçebilir; durum gönderilmezse AVAILABLE olur. RESERVED veritabanı modelinde korunur, fakat tarihe dayalı rezervasyon oluştururken aracın genel durumu RESERVED yapılmaz. Böylece başka tarihlerde rezervasyon alabilir. Gerçek müsaitlik her zaman rezervasyon aralıklarından hesaplanır; MAINTENANCE ve IN_USE araçlar ayrıca elenir.

`PUT` tam güncellemedir: zorunlu plate/type alanlarını ve korunacak opsiyonel alanları tekrar gönderin. Gönderilmeyen opsiyonel alanlar NULL olur. Araç şirketi değiştirilemez. Aktif rezervasyonu olan aracın tam güncellemesi 409 döner; konum güncellemesi yapılabilir. Konum PATCH gövdesi: `{"lat":39.94,"lon":32.85}`. Sıfır koordinat geçerlidir; eksik koordinat geçersizdir.

Vehicle yanıt örneği:

```json
{"id":"33333333-3333-4333-8333-333333333333","company_id":"22222222-2222-4222-8222-222222222222","plate":"06ABC123","type":"TRUCK","status":"AVAILABLE","brand":"Örnek","model":"Kamyon","year":2024,"passengers":3,"cargo_kg":10000,"lat":39.93,"lon":32.86,"location_updated_at":"2026-10-22T14:00:00Z","price_per_km":25,"created_at":"2026-10-22T14:00:00Z","updated_at":"2026-10-22T14:00:00Z"}
```

Sorgular:

```text
GET /api/vehicles?type=TRUCK&passengers=2&cargo_kg=5000
GET /api/vehicles?company_id=22222222-2222-4222-8222-222222222222
GET /api/vehicles?type=TRUCK&start_at=2026-11-01T07%3A00%3A00Z&end_at=2026-11-01T09%3A00%3A00Z
```

`start_at` ve `end_at` birlikte verilmelidir. Parametreleri .NET URI araçlarıyla URL encode edin; `+03:00` içindeki `+` özellikle encode edilmelidir. Bilinmeyen kapasite, pozitif kapasite talebini karşılamaz. Tarih belirtilmeyen liste tüm uygun/kayıtlı araçları gösterir, müsaitlik garantisi vermez.

## Talep → eşleştirme → rezervasyon

1. AFAD `POST /api/requests` gönderir:

```json
{"pick_up_lat":39.93,"pick_up_lon":32.86,"dropoff_lat":39.8,"dropoff_lon":32.7,"vehicle_type":"TRUCK","passengers":2,"start_at":"2026-11-01T10:00:00+03:00","end_at":"2026-11-01T12:00:00+03:00"}
```

201 Request yanıtı: girdi alanları + `id`, `requester_id`, `status:"PENDING"`, `matched_vehicle_id:null`, `reservation_id:null`, `created_at`, `updated_at`.

2. `GET /api/requests/{requestId}/candidates`: tür, kapasite ve rezervasyon tarihlerine göre uygun araçları döndürür. Talepte bitiş tarihi yoksa bu sorguda `?end_at=...` zorunludur. Yakınlığa göre otomatik sıralama/rota hesabı henüz yoktur.
3. `POST /api/requests/{requestId}/match`:

```json
{"vehicle_id":"33333333-3333-4333-8333-333333333333"}
```

Yanıt Request.status MATCHED ve matched_vehicle_id. Eşleştirme tek başına zamanı bloke etmez.

4. `POST /api/reservations`:

```json
{"request_id":"44444444-4444-4444-8444-444444444444","vehicle_id":"33333333-3333-4333-8333-333333333333"}
```

Talepte end_at yoksa burada geçerli end_at gönderilmelidir. Başlangıç talebin başlangıcıdır; talepte bitiş varsa değiştirilmez. Fiyat/şirket/müşteri/status istemciden alınmaz.

201 Reservation yanıtı:

```json
{"id":"55555555-5555-4555-8555-555555555555","request_id":"44444444-4444-4444-8444-444444444444","vehicle_id":"33333333-3333-4333-8333-333333333333","company_id":"22222222-2222-4222-8222-222222222222","customer_id":"66666666-6666-4666-8666-666666666666","status":"PENDING","start_at":"2026-11-01T10:00:00+03:00","end_at":"2026-11-01T12:00:00+03:00","total_price":null,"confirmed_at":null,"completed_at":null,"cancelled_at":null,"created_at":"2026-10-22T14:00:00Z","updated_at":"2026-10-22T14:00:00Z"}
```

Talep aynı transaction içinde RESERVED olur ve reservation_id yazılır. Şirketi aracın gerçek şirketi, müşteriyi talep sahibi belirler. Opsiyonel alanlar SQL JSON yanıtlarında `null` olabilir; .NET DTO'larında nullable kullanın.

5. Şirket `POST /api/reservations/{id}/confirm` gönderir; 200 yanıtında status CONFIRMED ve confirmed_at dolar.

PENDING ve CONFIRMED rezervasyonlar zamanı bloke eder. Aralık `[başlangıç, bitiş)` biçimindedir; bir rezervasyonun bitiş anında diğeri başlayabilir. İptal/tamamlama blokeyi kaldırır. Çakışma 409 döner; frontend güncel adayları tekrar istemelidir. PENDING için otomatik süre sonu yoktur: bu mevcut SQL kuralı korunmuştur ve ürün ekibinin onayı gerekir.

İptal Request.status değerini CANCELLED yapar. Tamamlama Reservation.status değerini COMPLETED yapar; Request modelinde COMPLETED olmadığından talep RESERVED olarak kalır. Otomatik ücret/rota hesabı yoktur; total_price NULL kalır, arayüz bunu 0 olarak göstermemelidir.

## Hatalar ve CORS

İş kuralları ve doğrulama hataları: `{"error":"kod"}`. Örnek: `{"error":"reservation_overlap"}`.

| HTTP | Örnek kodlar / anlam |
|---|---|
| 400 | invalid_json, invalid_id, invalid_request, invalid_vehicle, invalid_filter, invalid_timestamp, valid_end_at_required, invalid_data |
| 401 | bearer_token_required, invalid_token, invalid_credentials |
| 403 | forbidden; rol veya sahiplik yetkisi yok |
| 404 | not_found; yetkisiz rezervasyon ayrıntısı da 404 olabilir |
| 409 | duplicate_record, reservation_overlap, relationship_conflict, request_must_match_vehicle, vehicle_has_active_reservations, invalid_reservation_transition, retry_transaction |
| 415 | application_json_required |
| 429 | rate_limited |
| 500 | internal_error; veritabanı ayrıntısı istemciye verilmez |
| 503 | database_unavailable, request_timeout |

Tanımsız yol ve yanlış HTTP metodu standart HTTP 404/405 döndürebilir; frontend hata gövdesini JSON varsaymadan Content-Type kontrol etmelidir. Gövde sınırı 64 KiB; büyük/bozuk gövde 400 invalid_json döner. Tanımsız JSON alanları reddedilir.

Tarayıcı frontend için `CORS_ORIGIN`, gerçek frontend origin'ine tam eşit olmalıdır. Yerel varsayılan `http://localhost:5000`; HTTPS veya farklı port kullanılıyorsa `.env` düzenlenip API yeniden başlatılır. Masaüstü .NET HTTP istemcisi CORS'a tabi değildir. Frontend'e veritabanı parolası veya AUTH_SECRET verilmez.
