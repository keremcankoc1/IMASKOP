# IMASKOP

AFAD araç rezervasyonu: Go HTTP API + PostgreSQL. .NET frontend bu API'yi kullanır; PostgreSQL'e doğrudan bağlanmaz.

Bu çalışma yerel geliştirme içindir. [Teslim raporu](docs/TESLIM_RAPORU.md), [frontend API sözleşmesi](docs/API.md) ve [veritabanı rehberi](db/README.md) ayrıntıları içerir.

## Başlatma

1. Veritabanı çalışma klasöründeki `start-db.ps1` betiğini çalıştırın.
2. `backend/go/start-api.ps1` betiğini çalıştırın. Bu betik paket veya Go sürümü indirmez.
3. `http://127.0.0.1:8080/health` adresi `{"status":"ok"}` döndürmelidir.

Yerel bağlantı ve token anahtarı `backend/go/.env` içindedir ve Git dışında tutulur. Çalışma klasörünün kökündeki `.env`, yönetici hesabına aittir; backend için kullanmayın.

```powershell
# backend/go klasöründe bağlantıyı kontrol edin:
$env:GOPROXY = 'off'
$env:GOTOOLCHAIN = 'local'
go run -mod=readonly ./cmd/api -env .env -check-db
```

## AFAD hesabı

Kayıt API'si yalnızca COMPANY hesabı oluşturur. AFAD (CUSTOMER) ve ADMIN hesapları yetkili yerel işletmeci tarafından açılır. Gerçek AFAD hesabı henüz açılmadı; test hesapları ayrı test şemalarındadır.

```powershell
# backend/go klasöründe; Read-Host parolayı ekrana yazmaz.
$secure = Read-Host 'AFAD parolası (12–72 UTF-8 bayt)' -AsSecureString
$credential = New-Object System.Management.Automation.PSCredential('afad', $secure)
$env:PROVISION_PASSWORD = $credential.GetNetworkCredential().Password
try {
    go run -mod=readonly ./cmd/api -env .env -create-user -email 'GERCEK_AFAD_EPOSTASI' -name 'AFAD' -role CUSTOMER
} finally {
    Remove-Item Env:PROVISION_PASSWORD
}
```

Üretimde ortam değişkenleri dağıtım sistemi üzerinden sağlanmalıdır. `.env.production.example` yalnızca şablondur; HTTPS reverse proxy ve doğrulanan PostgreSQL TLS bağlantısı ayrıca yapılandırılmalıdır.

## Test

```powershell
# Repo kökünde; yalnızca kurulu araçları ve önbellekteki paketleri kullanır:
.\db\test-integration.ps1
# backend/go klasöründe:
go test -mod=readonly ./...
go vet -mod=readonly ./...
```

Entegrasyon betiği her çalıştırmada `imaskop` içinde ayrı bir `imaskop_test_*` şeması oluşturur. `public` kayıtlarına test verisi yazmaz. Şemaları otomatik silmez. Normal `go test ./...`, `TEST_DATABASE_URL` yoksa entegrasyon testini atlar; bağlantılı test için betiği kullanın.

## Mimari

- `internal/model`: mevcut beş model; Reservation'a eksik başlangıç/bitiş tarihleri eklendi.
- `internal/config`, `internal/database`: ortam ayarları ve sınırlı PostgreSQL bağlantısı.
- `internal/repository`: sorgulama ve transaction yardımcıları.
- `internal/service`: yetkilendirme, şirket/araç ve rezervasyon iş kuralları.
- `internal/handler`: HTTP yönlendirme, JSON doğrulama, kimlik doğrulama, CORS ve hata eşleme.
- `db/migrations`: tek çalıştırılan SQL migration kaynağı. AutoMigrate yoktur.

Go modül listesinde önceden kullanılmayan Gin/MongoDB/QUIC bağımlılıkları vardı. Boş `main.go` için mevcut router mimarisi bulunmadığından standart `net/http` kullanıldı; kullanılmayan paketler `go.mod` listesinden çıkarıldı. Eski `go.sum` kayıtları korunmuştur. GitHub'a gönderim yapılmadı.
