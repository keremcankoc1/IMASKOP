-- AFAD araç rezervasyon sisteminin ilk PostgreSQL şemasıdır.
-- İlk sürümde mevcut Go modellerinin tablo ve alan adları esas alınmıştır.
-- Yalnızca boş şemada çalıştırılır; uygulanmış veritabanında yeniden çalıştırmayın.
-- UUID tekil kimlik, PRIMARY KEY anahtar, DEFAULT varsayılan değer anlamına gelir.
-- NOT NULL zorunlu alanı, CHECK ise izin verilen değer veya aralıkları belirtir.

BEGIN;

-- btree_gist, UUID eşitliği ile zaman aralığı çakışmasını aynı indekste denetlemeyi sağlar.
CREATE EXTENSION IF NOT EXISTS btree_gist;

CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) NOT NULL,
    password_hash TEXT NOT NULL,
    name VARCHAR(150) NOT NULL,
    role TEXT NOT NULL DEFAULT 'CUSTOMER'
        CHECK (role IN ('ADMIN', 'COMPANY', 'CUSTOMER')),
    company_id UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
-- LOWER e-postayı küçük harfe çevirir; UNIQUE INDEX aynı e-postayla ikinci kaydı engeller.
CREATE UNIQUE INDEX users_email_lower_unique ON users (LOWER(email));

CREATE TABLE companies (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(200) NOT NULL,
    tax_number VARCHAR(30) NOT NULL UNIQUE,
    phone VARCHAR(30),
    address TEXT,
    latitude DOUBLE PRECISION,
    longitude DOUBLE PRECISION,
    owner_id UUID NOT NULL REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT companies_location_valid CHECK (
        (latitude IS NULL AND longitude IS NULL)
        OR (latitude IS NOT NULL AND longitude IS NOT NULL
            AND latitude BETWEEN -90 AND 90 AND longitude BETWEEN -180 AND 180)
    )
);

-- Kullanıcı-şirket döngüsü nedeniyle yabancı anahtar şirket tablosu oluşturulduktan sonra eklenir.
ALTER TABLE users
    ADD CONSTRAINT users_company_id_fkey
    FOREIGN KEY (company_id) REFERENCES companies(id);

CREATE TABLE vehicles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id UUID NOT NULL REFERENCES companies(id),
    plate VARCHAR(30) NOT NULL UNIQUE,
    type TEXT NOT NULL
        CHECK (type IN ('TRUCK', 'EXCAVATOR', 'BACKHOE')),
    brand VARCHAR(100),
    model VARCHAR(100),
    year INTEGER CHECK (year IS NULL OR year BETWEEN 1900 AND 2100),
    passengers INTEGER CHECK (passengers >= 0),
    cargo_kg NUMERIC(12, 2) CHECK (cargo_kg >= 0),
    latitude DOUBLE PRECISION,
    longitude DOUBLE PRECISION,
    location_updated_at TIMESTAMPTZ,
    status TEXT NOT NULL DEFAULT 'AVAILABLE'
        CHECK (status IN ('AVAILABLE', 'RESERVED', 'IN_USE', 'MAINTENANCE')),
    price_per_km NUMERIC(12, 2) CHECK (price_per_km >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT vehicles_location_valid CHECK (
        (latitude IS NULL AND longitude IS NULL)
        OR (latitude IS NOT NULL AND longitude IS NOT NULL
            AND latitude BETWEEN -90 AND 90 AND longitude BETWEEN -180 AND 180)
    )
);

CREATE TABLE requests (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    requester_id UUID NOT NULL REFERENCES users(id),
    pick_up_lat DOUBLE PRECISION NOT NULL CHECK (pick_up_lat BETWEEN -90 AND 90),
    pick_up_lon DOUBLE PRECISION NOT NULL CHECK (pick_up_lon BETWEEN -180 AND 180),
    dropoff_lat DOUBLE PRECISION,
    dropoff_lon DOUBLE PRECISION,
    vehicle_type TEXT NOT NULL
        CHECK (vehicle_type IN ('TRUCK', 'EXCAVATOR', 'BACKHOE')),
    passengers INTEGER CHECK (passengers >= 0),
    start_at TIMESTAMPTZ NOT NULL,
    end_at TIMESTAMPTZ,
    status TEXT NOT NULL DEFAULT 'PENDING'
        CHECK (status IN ('PENDING', 'MATCHED', 'RESERVED', 'CANCELLED')),
    matched_vehicle_id UUID REFERENCES vehicles(id),
    reservation_id UUID, -- Yabancı anahtar, reservations tablosu oluşturulduktan sonra eklenir.
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT requests_dropoff_valid CHECK (
        (dropoff_lat IS NULL AND dropoff_lon IS NULL)
        OR (dropoff_lat IS NOT NULL AND dropoff_lon IS NOT NULL
            AND dropoff_lat BETWEEN -90 AND 90 AND dropoff_lon BETWEEN -180 AND 180)
    ),
    CONSTRAINT requests_time_valid CHECK (end_at IS NULL OR end_at > start_at)
);

CREATE TABLE reservations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    request_id UUID NOT NULL REFERENCES requests(id),
    vehicle_id UUID NOT NULL REFERENCES vehicles(id),
    company_id UUID NOT NULL REFERENCES companies(id),
    customer_id UUID NOT NULL REFERENCES users(id),
    status TEXT NOT NULL DEFAULT 'PENDING'
        CHECK (status IN ('PENDING', 'CONFIRMED', 'COMPLETED', 'CANCELLED')),
    total_price NUMERIC(12, 2) CHECK (total_price >= 0),
    confirmed_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    cancelled_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    start_at TIMESTAMPTZ NOT NULL,
    end_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT reservations_time_valid CHECK (end_at > start_at),
    -- [başlangıç,bitiş) aralığı, önceki rezervasyon bittiği anda yenisinin başlamasına izin verir.
    -- EXCLUDE aynı araçta çakışmayı engeller; && zaman aralıklarının çakışma işlemcisidir.
    CONSTRAINT reservations_vehicle_no_overlap
        EXCLUDE USING gist (
            vehicle_id WITH =,
            tstzrange(start_at, end_at, '[)') WITH &&
        ) WHERE (status IN ('PENDING', 'CONFIRMED'))
);

-- Mevcut Go Request modelinde ReservationID bulunduğu için bu bağlantı korunur.
-- 000002 sürümü, bu rezervasyonun aynı talebe ait olmasını da denetler.
ALTER TABLE requests
    ADD CONSTRAINT requests_reservation_id_fkey
    FOREIGN KEY (reservation_id) REFERENCES reservations(id);

-- Kısmi indeks yalnızca aktif durumları kapsar: bir talepte tek aktif rezervasyon olabilir.
-- İptal edilmiş veya tamamlanmış geçmiş kayıtlar korunur.
CREATE UNIQUE INDEX reservations_one_active_per_request
    ON reservations (request_id)
    WHERE status IN ('PENDING', 'CONFIRMED');

CREATE INDEX users_company_id_idx ON users(company_id);
CREATE INDEX companies_owner_id_idx ON companies(owner_id);
CREATE INDEX vehicles_company_id_idx ON vehicles(company_id);
CREATE INDEX vehicles_type_status_idx ON vehicles(type, status);
CREATE INDEX requests_requester_id_idx ON requests(requester_id);
CREATE INDEX requests_matched_vehicle_id_idx ON requests(matched_vehicle_id);
CREATE INDEX reservations_company_id_idx ON reservations(company_id);
CREATE INDEX reservations_customer_id_idx ON reservations(customer_id);
CREATE INDEX reservations_request_id_idx ON reservations(request_id);

-- Tetikleyici, Go dışında SQL ile güncellenen kayıtlarda da updated_at tarihini yeniler.
-- NOW() işlemin geçerli zamanını, NEW ise güncellenecek satırı temsil eder.
CREATE FUNCTION set_updated_at() RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER users_set_updated_at BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER companies_set_updated_at BEFORE UPDATE ON companies
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER vehicles_set_updated_at BEFORE UPDATE ON vehicles
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER requests_set_updated_at BEFORE UPDATE ON requests
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER reservations_set_updated_at BEFORE UPDATE ON reservations
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

COMMIT;
