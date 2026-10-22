package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"imaskop/internal/model"
	"imaskop/internal/repository"
)

type RequestInput struct {
	PickUpLat   *float64          `json:"pick_up_lat"`
	PickUpLon   *float64          `json:"pick_up_lon"`
	DropoffLat  *float64          `json:"dropoff_lat"`
	DropoffLon  *float64          `json:"dropoff_lon"`
	VehicleType model.VehicleType `json:"vehicle_type"`
	Passengers  *int              `json:"passengers"`
	StartAt     time.Time         `json:"start_at"`
	EndAt       *time.Time        `json:"end_at"`
}

func (s *Service) CreateRequest(ctx context.Context, u model.User, in RequestInput) (json.RawMessage, error) {
	if err := Role(u, model.RoleCustomer, model.RoleAdmin); err != nil {
		return nil, err
	}
	if in.PickUpLat == nil || in.PickUpLon == nil || !location(in.PickUpLat, in.PickUpLon) || !location(in.DropoffLat, in.DropoffLon) || !vehicleType(in.VehicleType) || in.StartAt.IsZero() || in.EndAt != nil && !in.EndAt.After(in.StartAt) || in.Passengers != nil && *in.Passengers < 0 {
		return nil, Bad("invalid_request")
	}
	return repository.One(ctx, s.Store.DB, "INSERT INTO requests AS r(requester_id,pick_up_lat,pick_up_lon,dropoff_lat,dropoff_lon,vehicle_type,passengers,start_at,end_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9) RETURNING to_jsonb(r)", u.ID, in.PickUpLat, in.PickUpLon, in.DropoffLat, in.DropoffLon, in.VehicleType, in.Passengers, in.StartAt, in.EndAt)
}
func readRequest(ctx context.Context, q repository.Queryer, u model.User, id string, lock bool) (model.Request, error) {
	var r model.Request
	query := "SELECT id,requester_id,vehicle_type,passengers,start_at,end_at,status,matched_vehicle_id,reservation_id FROM requests WHERE id=$1"
	if lock {
		query += " FOR UPDATE"
	}
	err := q.QueryRowContext(ctx, query, id).Scan(&r.ID, &r.RequesterID, &r.TypeOfVehicle, &r.Passengers, &r.StartAt, &r.EndAt, &r.Status, &r.MatchedVehicleID, &r.ReservationID)
	if err != nil {
		return r, err
	}
	if u.Role != model.RoleAdmin && (u.Role != model.RoleCustomer || u.ID != r.RequesterID) {
		return r, Forbidden()
	}
	return r, nil
}
func (s *Service) Requests(ctx context.Context, u model.User) ([]json.RawMessage, error) {
	if err := Role(u, model.RoleCustomer, model.RoleAdmin); err != nil {
		return nil, err
	}
	return repository.Many(ctx, s.Store.DB, "SELECT to_jsonb(r) FROM requests r WHERE ($1='ADMIN' OR requester_id=$2) ORDER BY created_at DESC LIMIT 200", u.Role, u.ID)
}
func (s *Service) Request(ctx context.Context, u model.User, id string) (json.RawMessage, error) {
	if _, err := readRequest(ctx, s.Store.DB, u, id, false); err != nil {
		return nil, err
	}
	return repository.One(ctx, s.Store.DB, "SELECT to_jsonb(r) FROM requests r WHERE id=$1", id)
}
func (s *Service) Candidates(ctx context.Context, u model.User, id string, end *time.Time) ([]json.RawMessage, error) {
	r, err := readRequest(ctx, s.Store.DB, u, id, false)
	if err != nil {
		return nil, err
	}
	if r.Status != model.RequestStatusPending && r.Status != model.RequestStatusMatched {
		return nil, Conflict("request_not_open")
	}
	if r.EndAt != nil {
		end = r.EndAt
	}
	if end == nil {
		return nil, Bad("end_at_required_for_availability")
	}
	p := 0
	if r.Passengers != nil {
		p = *r.Passengers
	}
	return s.Vehicles(ctx, u, VehicleFilter{Type: string(r.TypeOfVehicle), Passengers: p, Start: &r.StartAt, End: end})
}
func checkVehicle(ctx context.Context, q repository.Queryer, r model.Request, id string, end *time.Time) (string, error) {
	var cid string
	var typ, status string
	var capacity *int
	if err := q.QueryRowContext(ctx, "SELECT company_id,type,status,passengers FROM vehicles WHERE id=$1 FOR UPDATE", id).Scan(&cid, &typ, &status, &capacity); err != nil {
		return "", err
	}
	if typ != string(r.TypeOfVehicle) || status == "MAINTENANCE" || status == "IN_USE" || r.Passengers != nil && *r.Passengers > 0 && (capacity == nil || *capacity < *r.Passengers) {
		return "", Conflict("vehicle_not_eligible")
	}
	if end != nil {
		var overlap bool
		err := q.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM reservations WHERE vehicle_id=$1 AND status IN ('PENDING','CONFIRMED') AND tstzrange(start_at,end_at,'[)') && tstzrange($2,$3,'[)'))", id, r.StartAt, end).Scan(&overlap)
		if err != nil {
			return "", err
		}
		if overlap {
			return "", Conflict("reservation_overlap")
		}
	}
	return cid, nil
}
func (s *Service) Match(ctx context.Context, u model.User, id, vid string) (json.RawMessage, error) {
	var result json.RawMessage
	err := s.Store.Transaction(ctx, func(tx *sql.Tx) error {
		r, err := readRequest(ctx, tx, u, id, true)
		if err != nil {
			return err
		}
		if r.Status != model.RequestStatusPending && r.Status != model.RequestStatusMatched {
			return Conflict("request_not_open")
		}
		if _, err = checkVehicle(ctx, tx, r, vid, r.EndAt); err != nil {
			return err
		}
		result, err = repository.One(ctx, tx, "UPDATE requests r SET matched_vehicle_id=$1,status='MATCHED' WHERE id=$2 RETURNING to_jsonb(r)", vid, id)
		return err
	})
	return result, err
}

type ReservationInput struct {
	RequestID uuid.UUID  `json:"request_id"`
	VehicleID uuid.UUID  `json:"vehicle_id"`
	EndAt     *time.Time `json:"end_at"`
}

func (s *Service) CreateReservation(ctx context.Context, u model.User, in ReservationInput) (json.RawMessage, error) {
	if in.RequestID == uuid.Nil || in.VehicleID == uuid.Nil {
		return nil, Bad("invalid_reservation")
	}
	var result json.RawMessage
	err := s.Store.Transaction(ctx, func(tx *sql.Tx) error {
		r, err := readRequest(ctx, tx, u, in.RequestID.String(), true)
		if err != nil {
			return err
		}
		if r.Status != model.RequestStatusMatched || r.MatchedVehicleID == nil || *r.MatchedVehicleID != in.VehicleID {
			return Conflict("request_must_match_vehicle")
		}
		end := r.EndAt
		if end == nil {
			end = in.EndAt
		} else if in.EndAt != nil && !in.EndAt.Equal(*end) {
			return Bad("reservation_dates_must_match_request")
		}
		if end == nil || !end.After(r.StartAt) {
			return Bad("valid_end_at_required")
		}
		cid, err := checkVehicle(ctx, tx, r, in.VehicleID.String(), end)
		if err != nil {
			return err
		}
		var rid string
		err = tx.QueryRowContext(ctx, "INSERT INTO reservations(request_id,vehicle_id,company_id,customer_id,start_at,end_at) VALUES($1,$2,$3,$4,$5,$6) RETURNING id", r.ID, in.VehicleID, cid, r.RequesterID, r.StartAt, end).Scan(&rid)
		if err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, "UPDATE requests SET reservation_id=$1,status='RESERVED',end_at=$2 WHERE id=$3", rid, end, r.ID); err != nil {
			return err
		}
		result, err = repository.One(ctx, tx, "SELECT to_jsonb(r) FROM reservations r WHERE id=$1", rid)
		return err
	})
	return result, err
}
func (s *Service) Reservations(ctx context.Context, u model.User) ([]json.RawMessage, error) {
	return repository.Many(ctx, s.Store.DB, "SELECT to_jsonb(r) FROM reservations r JOIN companies c ON c.id=r.company_id WHERE $1='ADMIN' OR ($1='COMPANY' AND c.owner_id=$2) OR ($1='CUSTOMER' AND r.customer_id=$2) ORDER BY r.created_at DESC LIMIT 200", u.Role, u.ID)
}
func (s *Service) Reservation(ctx context.Context, u model.User, id string) (json.RawMessage, error) {
	return repository.One(ctx, s.Store.DB, "SELECT to_jsonb(r) FROM reservations r JOIN companies c ON c.id=r.company_id WHERE r.id=$1 AND ($2='ADMIN' OR ($2='COMPANY' AND c.owner_id=$3) OR ($2='CUSTOMER' AND r.customer_id=$3))", id, u.Role, u.ID)
}
func (s *Service) ReservationAction(ctx context.Context, u model.User, id, action string) (json.RawMessage, error) {
	var result json.RawMessage
	err := s.Store.Transaction(ctx, func(tx *sql.Tx) error {
		// Always lock request, vehicle, then reservation to serialize lifecycle changes.
		var reqID, vehicleID string
		if err := tx.QueryRowContext(ctx, "SELECT request_id,vehicle_id FROM reservations WHERE id=$1", id).Scan(&reqID, &vehicleID); err != nil {
			return err
		}
		var ignore string
		if err := tx.QueryRowContext(ctx, "SELECT id FROM requests WHERE id=$1 FOR UPDATE", reqID).Scan(&ignore); err != nil {
			return err
		}
		if err := tx.QueryRowContext(ctx, "SELECT id FROM vehicles WHERE id=$1 FOR UPDATE", vehicleID).Scan(&ignore); err != nil {
			return err
		}
		var status, cid, customer string
		if err := tx.QueryRowContext(ctx, "SELECT status,company_id,customer_id FROM reservations WHERE id=$1 FOR UPDATE", id).Scan(&status, &cid, &customer); err != nil {
			return err
		}
		allowed := u.Role == model.RoleAdmin
		if u.Role == model.RoleCompany {
			if err := s.companyAccess(ctx, tx, u, cid); err != nil {
				return err
			}
			allowed = true
		}
		if action == "cancel" && u.Role == model.RoleCustomer && u.ID.String() == customer {
			allowed = true
		}
		if !allowed {
			return Forbidden()
		}
		var query string
		switch action {
		case "confirm":
			if status != "PENDING" {
				return Conflict("invalid_reservation_transition")
			}
			query = "UPDATE reservations r SET status='CONFIRMED',confirmed_at=now() WHERE id=$1 RETURNING to_jsonb(r)"
		case "cancel":
			if status != "PENDING" && status != "CONFIRMED" {
				return Conflict("invalid_reservation_transition")
			}
			query = "UPDATE reservations r SET status='CANCELLED',cancelled_at=now() WHERE id=$1 RETURNING to_jsonb(r)"
		case "complete":
			if status != "CONFIRMED" {
				return Conflict("invalid_reservation_transition")
			}
			query = "UPDATE reservations r SET status='COMPLETED',completed_at=now() WHERE id=$1 RETURNING to_jsonb(r)"
		default:
			return Bad("invalid_action")
		}
		var err error
		result, err = repository.One(ctx, tx, query, id)
		if err != nil {
			return err
		}
		if action == "cancel" {
			_, err = tx.ExecContext(ctx, "UPDATE requests SET status='CANCELLED' WHERE id=$1", reqID)
		}
		return err
	})
	return result, err
}
