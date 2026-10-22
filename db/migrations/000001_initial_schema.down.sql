-- TEHLİKE: Beş tablodaki verileri siler. Kullanıcı onayı olmadan çalıştırmayın.
BEGIN;
-- Tablolar silinmeden önce döngü oluşturan yabancı anahtar bağlantıları kaldırılır.
ALTER TABLE requests DROP CONSTRAINT IF EXISTS requests_reservation_id_fkey;
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_company_id_fkey;
DROP TABLE IF EXISTS reservations;
DROP TABLE IF EXISTS requests;
DROP TABLE IF EXISTS vehicles;
DROP TABLE IF EXISTS companies;
DROP TABLE IF EXISTS users;
DROP FUNCTION IF EXISTS set_updated_at();
-- Başka şemalar kullanabileceği için btree_gist eklentisi korunur.
COMMIT;
