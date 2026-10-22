package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func Open(ctx context.Context, url string) (*sql.DB, error) {
	db, err := sql.Open("pgx", url)
	if err != nil {
		return nil, errors.New("invalid PostgreSQL configuration")
	}
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)
	c, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err = db.PingContext(c); err != nil {
		db.Close()
		return nil, errors.New("PostgreSQL connection failed; check server, database, credentials and port")
	}
	var name string
	var super bool
	if err = db.QueryRowContext(c, "SELECT current_database(), rolsuper FROM pg_roles WHERE rolname=current_user").Scan(&name, &super); err != nil {
		db.Close()
		return nil, errors.New("could not inspect PostgreSQL role")
	}
	if name != "imaskop" || super {
		db.Close()
		return nil, fmt.Errorf("application requires database imaskop and a non-superuser role")
	}
	return db, nil
}
