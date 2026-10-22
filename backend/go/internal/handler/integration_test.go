package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"imaskop/internal/database"
	"imaskop/internal/model"
	"imaskop/internal/repository"
	"imaskop/internal/service"
)

func TestIntegration(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL required; run db/test-integration.ps1")
	}
	if !strings.Contains(url, "search_path=imaskop_test_") {
		t.Fatal("integration tests require a separate imaskop_test_ schema")
	}
	ctx := context.Background()
	db, err := database.Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var schema string
	if err := db.QueryRow("SELECT current_schema()").Scan(&schema); err != nil || !strings.HasPrefix(schema, "imaskop_test_") {
		t.Fatal("refusing tests outside an isolated schema")
	}
	s := &service.Service{Store: &repository.Store{DB: db}, Secret: []byte("integration-test-secret-at-least-32-bytes")}
	api := &API{Service: s, Origin: "http://localhost:5000"}
	server := httptest.NewServer(api.Routes())
	defer server.Close()
	activeT := t
	run := func(name string, fn func(*testing.T)) {
		t.Run(name, func(child *testing.T) {
			activeT = child
			defer func() { activeT = t }()
			fn(child)
		})
	}
	call := func(method, path, token string, input any, want int) map[string]any {
		activeT.Helper()
		var b io.Reader
		if input != nil {
			data, _ := json.Marshal(input)
			b = bytes.NewReader(data)
		}
		req, _ := http.NewRequest(method, server.URL+path, b)
		if input != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			activeT.Fatal(err)
		}
		defer resp.Body.Close()
		data, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != want {
			activeT.Fatalf("%s %s: want %d got %d body=%s", method, path, want, resp.StatusCode, data)
		}
		var result map[string]any
		if len(data) > 0 && data[0] == '{' {
			if err := json.Unmarshal(data, &result); err != nil {
				activeT.Fatal(err)
			}
		}
		return result
	}
	list := func(path, token string) []map[string]any {
		activeT.Helper()
		req, _ := http.NewRequest("GET", server.URL+path, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			activeT.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			activeT.Fatalf("list status %d", resp.StatusCode)
		}
		var rows []map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&rows); err != nil {
			activeT.Fatal(err)
		}
		return rows
	}
	password := "Integration-Password-2026!"
	login := func(email string) string {
		t.Helper()
		return call("POST", "/api/auth/login", "", map[string]any{"email": email, "password": password}, 200)["access_token"].(string)
	}
	register := func(email string) string {
		t.Helper()
		call("POST", "/api/auth/register", "", map[string]any{"email": email, "password": password, "name": "TEST Company", "role": "COMPANY"}, 201)
		return login(email)
	}
	run("public-registration-cannot-create-admin-or-afad", func(t *testing.T) {
		for _, role := range []string{"ADMIN", "CUSTOMER"} {
			call("POST", "/api/auth/register", "", map[string]any{"email": "escalate@example.test", "password": password, "name": "TEST", "role": role}, 403)
		}
	})
	if _, err := s.CreateUser(ctx, service.Credentials{Email: "afad@example.test", Password: password, Name: "TEST AFAD"}, model.RoleCustomer); err != nil {
		t.Fatal(err)
	}
	afad := login("afad@example.test")
	owner := register("owner@example.test")
	stranger := register("other@example.test")
	run("auth-errors", func(t *testing.T) {
		call("GET", "/api/me", "", nil, 401)
		call("GET", "/api/me", afad+"tampered", nil, 401)
		call("POST", "/api/auth/login", "", map[string]any{"email": "afad@example.test", "password": "wrong"}, 401)
	})
	company := call("POST", "/api/companies", owner, map[string]any{"name": "TEST Fleet", "tax_number": "TEST-001", "lat": 39.9, "lon": 32.8}, 201)["id"].(string)
	company2 := call("POST", "/api/companies", stranger, map[string]any{"name": "TEST Other", "tax_number": "TEST-002"}, 201)["id"].(string)
	vehicleData := map[string]any{"company_id": company, "plate": "TEST-001", "type": "TRUCK", "passengers": 4, "cargo_kg": 5000, "lat": 39.9, "lon": 32.8}
	vehicle := call("POST", "/api/vehicles", owner, vehicleData, 201)["id"].(string)
	run("vehicle-persists", func(t *testing.T) {
		var count int
		if err := db.QueryRow("SELECT count(*) FROM vehicles WHERE id=$1 AND company_id=$2", vehicle, company).Scan(&count); err != nil || count != 1 {
			t.Fatalf("vehicle not persisted: %v", err)
		}
	})
	run("ownership-and-validation", func(t *testing.T) {
		call("POST", "/api/vehicles", stranger, vehicleData, 403)
		call("PUT", "/api/vehicles/"+vehicle, stranger, vehicleData, 403)
		call("PATCH", "/api/vehicles/"+vehicle+"/location", stranger, map[string]any{"lat": 0, "lon": 0}, 403)
		call("PATCH", "/api/vehicles/"+vehicle+"/location", owner, map[string]any{"lat": 0, "lon": 0}, 200)
		call("POST", "/api/requests", owner, map[string]any{}, 403)
		call("GET", "/api/vehicles?passengers=-1", afad, nil, 400)
		call("GET", "/api/vehicles?cargo_kg=NaN", afad, nil, 400)
		call("GET", "/api/vehicles/not-uuid", afad, nil, 400)
	})
	start := time.Now().UTC().Add(24 * time.Hour).Truncate(time.Second)
	end := start.Add(2 * time.Hour)
	request := func(from, to time.Time) string {
		t.Helper()
		return call("POST", "/api/requests", afad, map[string]any{"pick_up_lat": 39.9, "pick_up_lon": 32.8, "vehicle_type": "TRUCK", "passengers": 2, "start_at": from, "end_at": to}, 201)["id"].(string)
	}
	match := func(id string) {
		t.Helper()
		call("POST", "/api/requests/"+id+"/match", afad, map[string]any{"vehicle_id": vehicle}, 200)
	}
	reserve := func(id string, status int) map[string]any {
		t.Helper()
		return call("POST", "/api/reservations", afad, map[string]any{"request_id": id, "vehicle_id": vehicle}, status)
	}
	r1, r2 := request(start, end), request(start.Add(time.Minute), end)
	call("GET", "/api/requests/"+r1+"/candidates", afad, nil, 200)
	match(r1)
	match(r2)
	reservation := reserve(r1, 201)["id"].(string)
	run("pending-availability-and-capacity-filter", func(t *testing.T) {
		if len(list("/api/requests/"+r2+"/candidates", afad)) != 0 {
			t.Fatal("pending booking did not block availability")
		}
		if len(list("/api/vehicles?type=TRUCK&passengers=5", afad)) != 0 {
			t.Fatal("capacity filter ignored")
		}
		if len(list("/api/vehicles?type=EXCAVATOR", afad)) != 0 {
			t.Fatal("type filter ignored")
		}
		if len(list("/api/vehicles", stranger)) != 0 {
			t.Fatal("company can list another fleet")
		}
	})
	run("pending-reservation-blocks-and-overlap-is-rejected", func(t *testing.T) { reserve(r2, 409) })
	run("reservation-ownership", func(t *testing.T) {
		call("POST", "/api/reservations/"+reservation+"/confirm", afad, nil, 403)
		call("POST", "/api/reservations/"+reservation+"/confirm", stranger, nil, 403)
		call("GET", "/api/reservations/"+reservation, stranger, nil, 404)
	})
	run("company-confirms", func(t *testing.T) {
		got := call("POST", "/api/reservations/"+reservation+"/confirm", owner, nil, 200)
		if got["status"] != "CONFIRMED" || got["confirmed_at"] == nil {
			t.Fatal("confirmation not persisted")
		}
	})
	run("adjacent-booking-is-allowed", func(t *testing.T) { id := request(end, end.Add(time.Hour)); match(id); reserve(id, 201) })
	run("database-rejects-mismatched-company", func(t *testing.T) {
		tx, _ := db.BeginTx(ctx, nil)
		defer tx.Rollback()
		_, err := tx.Exec("INSERT INTO reservations(request_id,vehicle_id,company_id,customer_id,start_at,end_at) SELECT request_id,vehicle_id,$1,customer_id,start_at,end_at FROM reservations WHERE id=$2", company2, reservation)
		var pe *pgconn.PgError
		if !asPG(err, &pe) || (pe.Code != "23514" && pe.Code != "23503") {
			t.Fatalf("mismatched company accepted or wrong failure: %v", err)
		}
	})
	run("read-api-data", func(t *testing.T) {
		call("GET", "/api/vehicles", afad, nil, 200)
		call("GET", "/api/vehicles/"+vehicle, afad, nil, 200)
		call("GET", "/api/reservations", owner, nil, 200)
		call("GET", "/api/reservations/"+reservation, afad, nil, 200)
	})
	run("cancel-releases-availability", func(t *testing.T) {
		call("POST", "/api/reservations/"+reservation+"/cancel", afad, nil, 200)
		reserve(r2, 201)
	})
	run("complete-lifecycle", func(t *testing.T) {
		r := reserve(r2, 409)
		_ = r
		var id string
		if err := db.QueryRow("SELECT id FROM reservations WHERE request_id=$1", r2).Scan(&id); err != nil {
			t.Fatal(err)
		}
		call("POST", "/api/reservations/"+id+"/complete", owner, nil, 409)
		call("POST", "/api/reservations/"+id+"/confirm", owner, nil, 200)
		call("POST", "/api/reservations/"+id+"/complete", owner, nil, 200)
		call("POST", "/api/reservations/"+id+"/cancel", owner, nil, 409)
	})
	run("concurrent-api-reservations-exactly-one-wins", func(t *testing.T) {
		from := start.Add(48 * time.Hour)
		to := from.Add(time.Hour)
		ids := []string{request(from, to), request(from, to)}
		for _, id := range ids {
			match(id)
		}
		statuses := make(chan int, 2)
		var wg sync.WaitGroup
		for _, id := range ids {
			wg.Add(1)
			go func(id string) {
				defer wg.Done()
				b, _ := json.Marshal(map[string]string{"request_id": id, "vehicle_id": vehicle})
				req, _ := http.NewRequest("POST", server.URL+"/api/reservations", bytes.NewReader(b))
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("Authorization", "Bearer "+afad)
				resp, err := http.DefaultClient.Do(req)
				if err != nil {
					statuses <- 0
					return
				}
				io.Copy(io.Discard, resp.Body)
				resp.Body.Close()
				statuses <- resp.StatusCode
			}(id)
		}
		wg.Wait()
		close(statuses)
		counts := map[int]int{}
		for status := range statuses {
			counts[status]++
		}
		if counts[201] != 1 || counts[409] != 1 {
			t.Fatalf("concurrent results %v", counts)
		}
	})
	run("postgres-exclusion-protects-concurrent-direct-sql", func(t *testing.T) {
		from := start.Add(96 * time.Hour)
		to := from.Add(time.Hour)
		ids := []string{request(from, to), request(from, to)}
		for _, id := range ids {
			match(id)
		}
		var customer string
		if err := db.QueryRow("SELECT id FROM users WHERE email='afad@example.test'").Scan(&customer); err != nil {
			t.Fatal(err)
		}
		tx1, err := db.BeginTx(ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		defer tx1.Rollback()
		query := "INSERT INTO reservations(request_id,vehicle_id,company_id,customer_id,start_at,end_at) VALUES($1,$2,$3,$4,$5,$6)"
		if _, err = tx1.Exec(query, ids[0], vehicle, company, customer, from, to); err != nil {
			t.Fatal(err)
		}
		done := make(chan error, 1)
		go func() {
			tx2, err := db.BeginTx(ctx, nil)
			if err != nil {
				done <- err
				return
			}
			defer tx2.Rollback()
			c, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()
			_, err = tx2.ExecContext(c, query, ids[1], vehicle, company, customer, from, to)
			done <- err
		}()
		if err = tx1.Commit(); err != nil {
			t.Fatal(err)
		}
		err = <-done
		var pe *pgconn.PgError
		if !asPG(err, &pe) || pe.Code != "23P01" {
			t.Fatalf("expected exclusion SQLSTATE 23P01, got %v", err)
		}
	})
	run("updated-at-and-limited-role", func(t *testing.T) {
		var super, createdb, createrole, createSchema bool
		if err := db.QueryRow("SELECT rolsuper,rolcreatedb,rolcreaterole,has_schema_privilege(current_user,'public','CREATE') FROM pg_roles WHERE rolname=current_user").Scan(&super, &createdb, &createrole, &createSchema); err != nil || super || createdb || createrole || createSchema {
			t.Fatalf("excess role permissions %v", err)
		}
		var created, updated time.Time
		if err := db.QueryRow("SELECT created_at,updated_at FROM vehicles WHERE id=$1", vehicle).Scan(&created, &updated); err != nil || !updated.After(created) {
			t.Fatalf("updated_at did not advance: %v", err)
		}
	})
	run("company-update-location-and-full-vehicle-update", func(t *testing.T) {
		call("PUT", "/api/companies/"+company, owner, map[string]any{"name": "TEST Updated", "tax_number": "TEST-001"}, 200)
		call("PATCH", "/api/companies/"+company+"/location", owner, map[string]any{"lat": 0, "lon": 0}, 200)
		call("PATCH", "/api/companies/"+company+"/location", stranger, map[string]any{"lat": 0, "lon": 0}, 403)
		// Future active reservations prevent changing the booked vehicle's capacity/type.
		call("PUT", "/api/vehicles/"+vehicle, owner, vehicleData, 409)
		spare := call("POST", "/api/vehicles", owner, map[string]any{"company_id": company, "plate": "TEST-SPARE", "type": "BACKHOE"}, 201)["id"].(string)
		call("PUT", "/api/vehicles/"+spare, owner, map[string]any{"plate": "TEST-SPARE", "type": "BACKHOE", "status": "MAINTENANCE"}, 200)
	})
	run("cors", func(t *testing.T) {
		req, _ := http.NewRequest("OPTIONS", server.URL+"/api/vehicles", nil)
		req.Header.Set("Origin", "http://localhost:5000")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != 204 || resp.Header.Get("Access-Control-Allow-Origin") != "http://localhost:5000" {
			t.Fatal("preflight failed")
		}
	})
}
func asPG(err error, target **pgconn.PgError) bool {
	if e, ok := err.(*pgconn.PgError); ok {
		*target = e
		return true
	}
	return false
}
