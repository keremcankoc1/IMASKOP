-- Bu migration mevcut tabloları korur; yalnızca yeni veri bütünlüğü kuralları ekler.
-- İşlem (transaction), bir adım başarısızsa bu dosyadaki tüm değişiklikleri geri alır.
BEGIN;
-- Danışma kilidi, iki migration işleminin aynı anda şemayı değiştirmesini önler.
SELECT pg_advisory_xact_lock(17442026);

-- Birleşik benzersizlik, iki sütunun birlikte başka bir tablodan referans alınmasını sağlar.
-- id zaten benzersizdir; bu ek indeksler birleşik yabancı anahtarlar için gereklidir.
CREATE UNIQUE INDEX vehicles_id_company_unique ON vehicles(id, company_id);
CREATE UNIQUE INDEX requests_id_requester_unique ON requests(id, requester_id);
CREATE UNIQUE INDEX reservations_id_request_unique ON reservations(id, request_id);

-- Birleşik yabancı anahtar: rezervasyonun şirketi aracın gerçek şirketi olmak zorundadır.
ALTER TABLE reservations ADD CONSTRAINT reservations_vehicle_company_fkey
    FOREIGN KEY (vehicle_id, company_id) REFERENCES vehicles(id, company_id);
-- Rezervasyonun müşterisi, rezervasyona bağlı talebi açan AFAD kullanıcısı olmalıdır.
ALTER TABLE reservations ADD CONSTRAINT reservations_request_customer_fkey
    FOREIGN KEY (request_id, customer_id) REFERENCES requests(id, requester_id);
-- Talepteki rezervasyon bağlantısı başka bir talebin rezervasyonuna işaret edemez.
ALTER TABLE requests ADD CONSTRAINT requests_own_reservation_fkey
    FOREIGN KEY (reservation_id, id) REFERENCES reservations(id, request_id);

-- Tetikleyici (trigger), doğrudan SQL ile yapılan aktif rezervasyonları da denetler.
-- Önce talep, sonra araç kilitlenir: eşzamanlı işlemler aynı kilit sırasını kullanır.
CREATE FUNCTION validate_active_reservation() RETURNS TRIGGER AS $$
DECLARE
    req requests%ROWTYPE;
    veh vehicles%ROWTYPE;
BEGIN
    IF NEW.status NOT IN ('PENDING', 'CONFIRMED') THEN
        RETURN NEW;
    END IF;
    SELECT * INTO req FROM requests WHERE id = NEW.request_id FOR UPDATE;
    SELECT * INTO veh FROM vehicles WHERE id = NEW.vehicle_id FOR UPDATE;
    -- P0001 gibi genel hata yerine 23514, uygulamanın tanıdığı veri kontrolü hatasıdır.
    IF req.id IS NULL OR veh.id IS NULL
       OR req.requester_id <> NEW.customer_id
       OR veh.company_id <> NEW.company_id
       OR req.vehicle_type <> veh.type
       OR veh.status IN ('MAINTENANCE', 'IN_USE')
       OR req.status NOT IN ('MATCHED', 'RESERVED')
       OR req.matched_vehicle_id IS DISTINCT FROM NEW.vehicle_id
       OR req.start_at <> NEW.start_at
       OR (req.end_at IS NOT NULL AND req.end_at <> NEW.end_at)
       OR (COALESCE(req.passengers, 0) > 0
           AND (veh.passengers IS NULL OR veh.passengers < req.passengers)) THEN
        RAISE EXCEPTION 'Rezervasyon talep, arac, sirket veya tarih bilgileriyle uyusmuyor'
            USING ERRCODE = '23514';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER reservations_validate_active BEFORE INSERT OR UPDATE ON reservations
    FOR EACH ROW EXECUTE FUNCTION validate_active_reservation();

-- Sürüm kaydı aynı işlem içinde yazılır; şema ve sürüm bilgisi birlikte kalıcılaşır.
INSERT INTO schema_migrations(version) VALUES ('000002');
COMMIT;
