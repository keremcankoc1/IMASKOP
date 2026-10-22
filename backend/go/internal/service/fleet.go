package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"
	"imaskop/internal/model"
	"imaskop/internal/repository"
)

type CompanyInput struct {
	Name      string   `json:"name"`
	TaxNumber string   `json:"tax_number"`
	Phone     *string  `json:"phone"`
	Address   *string  `json:"address"`
	Lat       *float64 `json:"lat"`
	Lon       *float64 `json:"lon"`
}
type Location struct {
	Lat *float64 `json:"lat"`
	Lon *float64 `json:"lon"`
}

func location(lat, lon *float64) bool {
	return (lat == nil && lon == nil) || (lat != nil && lon != nil && !math.IsNaN(*lat) && !math.IsNaN(*lon) && *lat >= -90 && *lat <= 90 && *lon >= -180 && *lon <= 180)
}
func vehicleType(t model.VehicleType) bool {
	return t == model.TypeTruck || t == model.TypeExcavator || t == model.TypeBackhoe
}
func (s *Service) companyAccess(ctx context.Context, q repository.Queryer, u model.User, id string) error {
	if u.Role == model.RoleAdmin {
		return nil
	}
	if u.Role != model.RoleCompany {
		return Forbidden()
	}
	var yes bool
	err := q.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM companies WHERE id=$1 AND owner_id=$2)", id, u.ID).Scan(&yes)
	if err != nil {
		return err
	}
	if !yes {
		return Forbidden()
	}
	return nil
}
func (s *Service) CreateCompany(ctx context.Context, u model.User, in CompanyInput) (json.RawMessage, error) {
	if err := Role(u, model.RoleCompany, model.RoleAdmin); err != nil {
		return nil, err
	}
	if !validCompany(in) {
		return nil, Bad("invalid_company")
	}
	var result json.RawMessage
	err := s.Store.Transaction(ctx, func(tx *sql.Tx) error {
		var company *string
		if err := tx.QueryRowContext(ctx, "SELECT company_id FROM users WHERE id=$1 FOR UPDATE", u.ID).Scan(&company); err != nil {
			return err
		}
		if company != nil {
			return Conflict("company_already_exists")
		}
		var id string
		err := tx.QueryRowContext(ctx, "INSERT INTO companies(name,tax_number,phone,address,latitude,longitude,owner_id) VALUES($1,$2,$3,$4,$5,$6,$7) RETURNING id", strings.TrimSpace(in.Name), strings.TrimSpace(in.TaxNumber), in.Phone, in.Address, in.Lat, in.Lon, u.ID).Scan(&id)
		if err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, "UPDATE users SET company_id=$1 WHERE id=$2", id, u.ID); err != nil {
			return err
		}
		result, err = repository.One(ctx, tx, "SELECT "+repository.CompanyJSON+" FROM companies c WHERE id=$1", id)
		return err
	})
	return result, err
}
func validCompany(in CompanyInput) bool {
	return len(strings.TrimSpace(in.Name)) > 0 && len(in.Name) <= 200 && len(strings.TrimSpace(in.TaxNumber)) > 0 && len(in.TaxNumber) <= 30 && location(in.Lat, in.Lon)
}
func (s *Service) UpdateCompany(ctx context.Context, u model.User, id string, in CompanyInput) (json.RawMessage, error) {
	if !validCompany(in) {
		return nil, Bad("invalid_company")
	}
	if err := s.companyAccess(ctx, s.Store.DB, u, id); err != nil {
		return nil, err
	}
	return repository.One(ctx, s.Store.DB, "UPDATE companies c SET name=$1,tax_number=$2,phone=$3,address=$4,latitude=$5,longitude=$6 WHERE id=$7 RETURNING "+repository.CompanyJSON, in.Name, in.TaxNumber, in.Phone, in.Address, in.Lat, in.Lon, id)
}
func (s *Service) CompanyLocation(ctx context.Context, u model.User, id string, in Location) (json.RawMessage, error) {
	if in.Lat == nil || in.Lon == nil || !location(in.Lat, in.Lon) {
		return nil, Bad("invalid_location")
	}
	if err := s.companyAccess(ctx, s.Store.DB, u, id); err != nil {
		return nil, err
	}
	return repository.One(ctx, s.Store.DB, "UPDATE companies c SET latitude=$1,longitude=$2 WHERE id=$3 RETURNING "+repository.CompanyJSON, in.Lat, in.Lon, id)
}

type VehicleInput struct {
	CompanyID  uuid.UUID           `json:"company_id"`
	Plate      string              `json:"plate"`
	Type       model.VehicleType   `json:"type"`
	Brand      *string             `json:"brand"`
	Model      *string             `json:"model"`
	Year       *int                `json:"year"`
	Passengers *int                `json:"passengers"`
	CargoKg    *float64            `json:"cargo_kg"`
	Lat        *float64            `json:"lat"`
	Lon        *float64            `json:"lon"`
	PricePerKm *float64            `json:"price_per_km"`
	Status     model.VehicleStatus `json:"status"`
}

func validVehicle(in VehicleInput) bool {
	if !vehicleType(in.Type) || strings.TrimSpace(in.Plate) == "" || len(in.Plate) > 30 || !location(in.Lat, in.Lon) {
		return false
	}
	if in.Passengers != nil && *in.Passengers < 0 || in.CargoKg != nil && *in.CargoKg < 0 || in.PricePerKm != nil && *in.PricePerKm < 0 || in.Year != nil && (*in.Year < 1900 || *in.Year > 2100) {
		return false
	}
	return in.Status == "" || in.Status == model.StatusAvailable || in.Status == model.StatusMaintenance || in.Status == model.StatusInUse
}
func (s *Service) CreateVehicle(ctx context.Context, u model.User, in VehicleInput) (json.RawMessage, error) {
	if !validVehicle(in) || in.CompanyID == uuid.Nil {
		return nil, Bad("invalid_vehicle")
	}
	if err := s.companyAccess(ctx, s.Store.DB, u, in.CompanyID.String()); err != nil {
		return nil, err
	}
	if in.Status == "" {
		in.Status = model.StatusAvailable
	}
	return repository.One(ctx, s.Store.DB, "INSERT INTO vehicles AS v(company_id,plate,type,brand,model,year,passengers,cargo_kg,latitude,longitude,location_updated_at,price_per_km,status) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,CASE WHEN $9::float8 IS NULL THEN NULL ELSE now() END,$11,$12) RETURNING "+repository.VehicleJSON, in.CompanyID, strings.ToUpper(strings.TrimSpace(in.Plate)), in.Type, in.Brand, in.Model, in.Year, in.Passengers, in.CargoKg, in.Lat, in.Lon, in.PricePerKm, in.Status)
}
func (s *Service) vehicleAccess(ctx context.Context, q repository.Queryer, u model.User, id string) error {
	var cid string
	if err := q.QueryRowContext(ctx, "SELECT company_id FROM vehicles WHERE id=$1", id).Scan(&cid); err != nil {
		return err
	}
	return s.companyAccess(ctx, q, u, cid)
}
func (s *Service) UpdateVehicle(ctx context.Context, u model.User, id string, in VehicleInput) (json.RawMessage, error) {
	if !validVehicle(in) {
		return nil, Bad("invalid_vehicle")
	}
	if in.Status == "" {
		in.Status = model.StatusAvailable
	}
	var result json.RawMessage
	err := s.Store.Transaction(ctx, func(tx *sql.Tx) error {
		var cid string
		if err := tx.QueryRowContext(ctx, "SELECT company_id FROM vehicles WHERE id=$1 FOR UPDATE", id).Scan(&cid); err != nil {
			return err
		}
		if err := s.companyAccess(ctx, tx, u, cid); err != nil {
			return err
		}
		if in.CompanyID != uuid.Nil && in.CompanyID.String() != cid {
			return Bad("vehicle_company_cannot_change")
		}
		// Active bookings must keep their agreed type/capacity and operational availability.
		var active bool
		if err := tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM reservations WHERE vehicle_id=$1 AND status IN ('PENDING','CONFIRMED'))", id).Scan(&active); err != nil {
			return err
		}
		if active {
			return Conflict("vehicle_has_active_reservations")
		}
		var err error
		result, err = repository.One(ctx, tx, "UPDATE vehicles v SET plate=$1,type=$2,brand=$3,model=$4,year=$5,passengers=$6,cargo_kg=$7,latitude=$8,longitude=$9,location_updated_at=CASE WHEN $8::float8 IS NULL THEN NULL ELSE now() END,price_per_km=$10,status=$11 WHERE id=$12 RETURNING "+repository.VehicleJSON, strings.ToUpper(strings.TrimSpace(in.Plate)), in.Type, in.Brand, in.Model, in.Year, in.Passengers, in.CargoKg, in.Lat, in.Lon, in.PricePerKm, in.Status, id)
		return err
	})
	return result, err
}
func (s *Service) VehicleLocation(ctx context.Context, u model.User, id string, in Location) (json.RawMessage, error) {
	if in.Lat == nil || in.Lon == nil || !location(in.Lat, in.Lon) {
		return nil, Bad("invalid_location")
	}
	if err := s.vehicleAccess(ctx, s.Store.DB, u, id); err != nil {
		return nil, err
	}
	return repository.One(ctx, s.Store.DB, "UPDATE vehicles v SET latitude=$1,longitude=$2,location_updated_at=now() WHERE id=$3 RETURNING "+repository.VehicleJSON, in.Lat, in.Lon, id)
}

type VehicleFilter struct {
	Type       string
	Passengers int
	CargoKg    float64
	Start, End *time.Time
	CompanyID  string
}

func (s *Service) Vehicles(ctx context.Context, u model.User, f VehicleFilter) ([]json.RawMessage, error) {
	if f.Type != "" && !vehicleType(model.VehicleType(f.Type)) || f.Passengers < 0 || f.CargoKg < 0 || math.IsNaN(f.CargoKg) || math.IsInf(f.CargoKg, 0) {
		return nil, Bad("invalid_filter")
	}
	if (f.Start == nil) != (f.End == nil) || f.Start != nil && !f.End.After(*f.Start) {
		return nil, Bad("invalid_time_range")
	}
	// Company accounts see their fleet; AFAD/admin can browse all companies.
	return repository.Many(ctx, s.Store.DB, "SELECT "+repository.VehicleJSON+" FROM vehicles v JOIN companies c ON c.id=v.company_id WHERE ($1='' OR v.type=$1) AND ($2=0 OR v.passengers >= $2) AND ($3::numeric=0 OR v.cargo_kg >= $3) AND ($4::timestamptz IS NULL OR (v.status NOT IN ('MAINTENANCE','IN_USE') AND NOT EXISTS(SELECT 1 FROM reservations r WHERE r.vehicle_id=v.id AND r.status IN ('PENDING','CONFIRMED') AND tstzrange(r.start_at,r.end_at,'[)') && tstzrange($4,$5,'[)')))) AND ($6 <> 'COMPANY' OR c.owner_id=$7) AND ($8='' OR v.company_id::text=$8) ORDER BY v.created_at DESC LIMIT 200", f.Type, f.Passengers, f.CargoKg, f.Start, f.End, u.Role, u.ID, f.CompanyID)
}
func (s *Service) Vehicle(ctx context.Context, u model.User, id string) (json.RawMessage, error) {
	if u.Role == model.RoleCompany {
		if err := s.vehicleAccess(ctx, s.Store.DB, u, id); err != nil {
			return nil, err
		}
	}
	return repository.One(ctx, s.Store.DB, "SELECT "+repository.VehicleJSON+" FROM vehicles v WHERE id=$1", id)
}
