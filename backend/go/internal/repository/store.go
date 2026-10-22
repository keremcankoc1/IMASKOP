package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"imaskop/internal/model"
)

type Store struct{ DB *sql.DB }
type Queryer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

func One(ctx context.Context, q Queryer, query string, args ...any) (json.RawMessage, error) {
	var b []byte
	err := q.QueryRowContext(ctx, query, args...).Scan(&b)
	return json.RawMessage(b), err
}
func Many(ctx context.Context, q Queryer, query string, args ...any) ([]json.RawMessage, error) {
	rows, err := q.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []json.RawMessage{}
	for rows.Next() {
		var b []byte
		if err := rows.Scan(&b); err != nil {
			return nil, err
		}
		result = append(result, json.RawMessage(b))
	}
	return result, rows.Err()
}
func (s *Store) Transaction(ctx context.Context, fn func(*sql.Tx) error) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = fn(tx); err != nil {
		return err
	}
	return tx.Commit()
}
func User(ctx context.Context, q Queryer, where string, args ...any) (model.User, error) {
	var u model.User
	err := q.QueryRowContext(ctx, "SELECT id,email,password_hash,name,role,company_id,created_at,updated_at FROM users WHERE "+where, args...).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.Name, &u.Role, &u.CompanyID, &u.CreatedAt, &u.UpdatedAt)
	return u, err
}
func IsMissing(err error) bool { return errors.Is(err, sql.ErrNoRows) }

// SQL identifiers below are constants, never user input. JSON uses the existing model tags.
const CompanyJSON = "(to_jsonb(c)-'latitude'-'longitude') || jsonb_build_object('lat',c.latitude,'lon',c.longitude)"
const VehicleJSON = "(to_jsonb(v)-'latitude'-'longitude') || jsonb_build_object('lat',v.latitude,'lon',v.longitude)"
