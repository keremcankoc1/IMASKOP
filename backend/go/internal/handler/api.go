package handler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"imaskop/internal/model"
	"imaskop/internal/repository"
	"imaskop/internal/service"
)

type API struct {
	Service *service.Service
	Origin  string
	limiter limiter
}
type endpoint func(http.ResponseWriter, *http.Request, model.User) (any, error)

func write(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
func failure(w http.ResponseWriter, err error) {
	status, code := 500, "internal_error"
	var se *service.Error
	var pe *pgconn.PgError
	switch {
	case errors.As(err, &se):
		status, code = se.Status, se.Code
	case repository.IsMissing(err):
		status, code = 404, "not_found"
	case errors.As(err, &pe):
		switch pe.Code {
		case "23P01":
			status, code = 409, "reservation_overlap"
		case "23505":
			status, code = 409, "duplicate_record"
		case "23503":
			status, code = 409, "relationship_conflict"
		case "23514", "22003", "22001", "22P02":
			status, code = 400, "invalid_data"
		case "40P01", "40001":
			status, code = 409, "retry_transaction"
		}
	case errors.Is(err, context.DeadlineExceeded):
		status, code = 503, "request_timeout"
	}
	if status == 500 {
		slog.Error("API operation failed", "error_type", fmtError(err))
	}
	write(w, status, map[string]string{"error": code})
}
func fmtError(err error) string {
	var pe *pgconn.PgError
	if errors.As(err, &pe) {
		return "postgres:" + pe.Code
	}
	return "operation_error"
}
func body(w http.ResponseWriter, r *http.Request, dst any) error {
	if strings.Split(r.Header.Get("Content-Type"), ";")[0] != "application/json" {
		return &service.Error{Status: 415, Code: "application_json_required"}
	}
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if err := d.Decode(dst); err != nil {
		return service.Bad("invalid_json")
	}
	if d.Decode(&struct{}{}) != io.EOF {
		return service.Bad("single_json_object_required")
	}
	return nil
}
func (a *API) wrap(auth bool, status int, fn endpoint) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		r = r.WithContext(ctx)
		var u model.User
		if auth {
			header := r.Header.Get("Authorization")
			if !strings.HasPrefix(header, "Bearer ") {
				failure(w, &service.Error{Status: 401, Code: "bearer_token_required"})
				return
			}
			var err error
			u, err = a.Service.Authenticate(ctx, strings.TrimPrefix(header, "Bearer "))
			if err != nil {
				failure(w, err)
				return
			}
		}
		if id := r.PathValue("id"); id != "" {
			if _, err := uuid.Parse(id); err != nil {
				failure(w, service.Bad("invalid_id"))
				return
			}
		}
		v, err := fn(w, r, u)
		if err != nil {
			failure(w, err)
			return
		}
		write(w, status, v)
	}
}
func (a *API) Routes() http.Handler {
	mux := http.NewServeMux()
	s := a.Service
	mux.HandleFunc("GET /health", a.wrap(false, 200, func(w http.ResponseWriter, r *http.Request, u model.User) (any, error) {
		if err := s.Store.DB.PingContext(r.Context()); err != nil {
			return nil, &service.Error{Status: 503, Code: "database_unavailable"}
		}
		return map[string]string{"status": "ok"}, nil
	}))
	mux.HandleFunc("POST /api/auth/register", a.wrap(false, 201, func(w http.ResponseWriter, r *http.Request, u model.User) (any, error) {
		if !a.limiter.allow(r) {
			return nil, &service.Error{Status: 429, Code: "rate_limited"}
		}
		var in service.Credentials
		if err := body(w, r, &in); err != nil {
			return nil, err
		}
		if in.Role != "" && in.Role != "COMPANY" {
			return nil, service.Forbidden()
		}
		return s.CreateUser(r.Context(), in, model.RoleCompany)
	}))
	mux.HandleFunc("POST /api/auth/login", a.wrap(false, 200, func(w http.ResponseWriter, r *http.Request, u model.User) (any, error) {
		if !a.limiter.allow(r) {
			return nil, &service.Error{Status: 429, Code: "rate_limited"}
		}
		var in service.Credentials
		if err := body(w, r, &in); err != nil {
			return nil, err
		}
		return s.Login(r.Context(), in)
	}))
	mux.HandleFunc("GET /api/me", a.wrap(true, 200, func(w http.ResponseWriter, r *http.Request, u model.User) (any, error) { return u, nil }))
	mux.HandleFunc("POST /api/companies", a.wrap(true, 201, func(w http.ResponseWriter, r *http.Request, u model.User) (any, error) {
		var in service.CompanyInput
		if err := body(w, r, &in); err != nil {
			return nil, err
		}
		return s.CreateCompany(r.Context(), u, in)
	}))
	mux.HandleFunc("PUT /api/companies/{id}", a.wrap(true, 200, func(w http.ResponseWriter, r *http.Request, u model.User) (any, error) {
		var in service.CompanyInput
		if err := body(w, r, &in); err != nil {
			return nil, err
		}
		return s.UpdateCompany(r.Context(), u, r.PathValue("id"), in)
	}))
	mux.HandleFunc("PATCH /api/companies/{id}/location", a.wrap(true, 200, func(w http.ResponseWriter, r *http.Request, u model.User) (any, error) {
		var in service.Location
		if err := body(w, r, &in); err != nil {
			return nil, err
		}
		return s.CompanyLocation(r.Context(), u, r.PathValue("id"), in)
	}))
	mux.HandleFunc("POST /api/vehicles", a.wrap(true, 201, func(w http.ResponseWriter, r *http.Request, u model.User) (any, error) {
		var in service.VehicleInput
		if err := body(w, r, &in); err != nil {
			return nil, err
		}
		return s.CreateVehicle(r.Context(), u, in)
	}))
	mux.HandleFunc("GET /api/vehicles", a.wrap(true, 200, func(w http.ResponseWriter, r *http.Request, u model.User) (any, error) {
		q := r.URL.Query()
		f := service.VehicleFilter{Type: q.Get("type"), CompanyID: q.Get("company_id")}
		var err error
		if q.Has("passengers") {
			f.Passengers, err = strconv.Atoi(q.Get("passengers"))
			if err != nil {
				return nil, service.Bad("invalid_passengers")
			}
		}
		if q.Has("cargo_kg") {
			f.CargoKg, err = strconv.ParseFloat(q.Get("cargo_kg"), 64)
			if err != nil {
				return nil, service.Bad("invalid_cargo_kg")
			}
		}
		if f.CompanyID != "" {
			if _, err := uuid.Parse(f.CompanyID); err != nil {
				return nil, service.Bad("invalid_company_id")
			}
		}
		f.Start, err = parseTime(q.Get("start_at"))
		if err != nil {
			return nil, err
		}
		f.End, err = parseTime(q.Get("end_at"))
		if err != nil {
			return nil, err
		}
		return s.Vehicles(r.Context(), u, f)
	}))
	mux.HandleFunc("GET /api/vehicles/{id}", a.wrap(true, 200, func(w http.ResponseWriter, r *http.Request, u model.User) (any, error) {
		return s.Vehicle(r.Context(), u, r.PathValue("id"))
	}))
	mux.HandleFunc("PUT /api/vehicles/{id}", a.wrap(true, 200, func(w http.ResponseWriter, r *http.Request, u model.User) (any, error) {
		var in service.VehicleInput
		if err := body(w, r, &in); err != nil {
			return nil, err
		}
		return s.UpdateVehicle(r.Context(), u, r.PathValue("id"), in)
	}))
	mux.HandleFunc("PATCH /api/vehicles/{id}/location", a.wrap(true, 200, func(w http.ResponseWriter, r *http.Request, u model.User) (any, error) {
		var in service.Location
		if err := body(w, r, &in); err != nil {
			return nil, err
		}
		return s.VehicleLocation(r.Context(), u, r.PathValue("id"), in)
	}))
	mux.HandleFunc("POST /api/requests", a.wrap(true, 201, func(w http.ResponseWriter, r *http.Request, u model.User) (any, error) {
		var in service.RequestInput
		if err := body(w, r, &in); err != nil {
			return nil, err
		}
		return s.CreateRequest(r.Context(), u, in)
	}))
	mux.HandleFunc("GET /api/requests", a.wrap(true, 200, func(w http.ResponseWriter, r *http.Request, u model.User) (any, error) {
		return s.Requests(r.Context(), u)
	}))
	mux.HandleFunc("GET /api/requests/{id}", a.wrap(true, 200, func(w http.ResponseWriter, r *http.Request, u model.User) (any, error) {
		return s.Request(r.Context(), u, r.PathValue("id"))
	}))
	mux.HandleFunc("GET /api/requests/{id}/candidates", a.wrap(true, 200, func(w http.ResponseWriter, r *http.Request, u model.User) (any, error) {
		end, err := parseTime(r.URL.Query().Get("end_at"))
		if err != nil {
			return nil, err
		}
		return s.Candidates(r.Context(), u, r.PathValue("id"), end)
	}))
	mux.HandleFunc("POST /api/requests/{id}/match", a.wrap(true, 200, func(w http.ResponseWriter, r *http.Request, u model.User) (any, error) {
		var in struct {
			VehicleID uuid.UUID `json:"vehicle_id"`
		}
		if err := body(w, r, &in); err != nil {
			return nil, err
		}
		if in.VehicleID == uuid.Nil {
			return nil, service.Bad("invalid_vehicle_id")
		}
		return s.Match(r.Context(), u, r.PathValue("id"), in.VehicleID.String())
	}))
	mux.HandleFunc("POST /api/reservations", a.wrap(true, 201, func(w http.ResponseWriter, r *http.Request, u model.User) (any, error) {
		var in service.ReservationInput
		if err := body(w, r, &in); err != nil {
			return nil, err
		}
		return s.CreateReservation(r.Context(), u, in)
	}))
	mux.HandleFunc("GET /api/reservations", a.wrap(true, 200, func(w http.ResponseWriter, r *http.Request, u model.User) (any, error) {
		return s.Reservations(r.Context(), u)
	}))
	mux.HandleFunc("GET /api/reservations/{id}", a.wrap(true, 200, func(w http.ResponseWriter, r *http.Request, u model.User) (any, error) {
		return s.Reservation(r.Context(), u, r.PathValue("id"))
	}))
	for _, action := range []string{"confirm", "cancel", "complete"} {
		mux.HandleFunc("POST /api/reservations/{id}/"+action, a.wrap(true, 200, func(w http.ResponseWriter, r *http.Request, u model.User) (any, error) {
			return s.ReservationAction(r.Context(), u, r.PathValue("id"), action)
		}))
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cache-Control", "no-store")
		origin := r.Header.Get("Origin")
		if origin != "" {
			w.Header().Add("Vary", "Origin")
			if a.Origin == "" || origin != a.Origin {
				failure(w, service.Forbidden())
				return
			}
			w.Header().Set("Access-Control-Allow-Origin", a.Origin)
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, OPTIONS")
		}
		if r.Method == "OPTIONS" {
			w.WriteHeader(204)
			return
		}
		mux.ServeHTTP(w, r)
	})
}
func parseTime(s string) (*time.Time, error) {
	if s == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return nil, service.Bad("invalid_timestamp")
	}
	return &t, nil
}

type bucket struct {
	start time.Time
	count int
}
type limiter struct {
	mu      sync.Mutex
	clients map[string]bucket
}

func (l *limiter) allow(r *http.Request) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.clients == nil {
		l.clients = map[string]bucket{}
	}
	now := time.Now()
	host, _, _ := net.SplitHostPort(r.RemoteAddr)
	if len(l.clients) >= 4096 {
		for k, b := range l.clients {
			if now.Sub(b.start) > time.Minute {
				delete(l.clients, k)
			}
		}
		if _, ok := l.clients[host]; !ok && len(l.clients) >= 4096 {
			return false
		}
	}
	b := l.clients[host]
	if now.Sub(b.start) > time.Minute {
		b = bucket{start: now}
	}
	b.count++
	l.clients[host] = b
	return b.count <= 30
}
