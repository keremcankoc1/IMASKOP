package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"imaskop/internal/config"
	"imaskop/internal/database"
	"imaskop/internal/handler"
	"imaskop/internal/model"
	"imaskop/internal/repository"
	"imaskop/internal/service"
)

func main() {
	if err := run(); err != nil {
		slog.Error("application stopped", "reason", err.Error())
		os.Exit(1)
	}
}
func run() error {
	env := flag.String("env", "", "optional environment file; never auto-loaded in production")
	create := flag.Bool("create-user", false, "provision AFAD/admin account; password from PROVISION_PASSWORD")
	email := flag.String("email", "", "account email")
	name := flag.String("name", "", "account name")
	role := flag.String("role", "CUSTOMER", "CUSTOMER or ADMIN")
	check := flag.Bool("check-db", false, "check application database connection and migrations")
	flag.Parse()
	if *env != "" {
		if err := config.LoadEnv(*env); err != nil {
			return err
		}
	}
	c, err := config.Load()
	if err != nil {
		return err
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	db, err := database.Open(ctx, c.DatabaseURL)
	if err != nil {
		return err
	}
	defer db.Close()
	var version string
	if err = db.QueryRowContext(ctx, "SELECT version FROM schema_migrations WHERE version='000002'").Scan(&version); err != nil {
		return errors.New("migration 000002 required; run the SQL migration script with administrator credentials")
	}
	s := &service.Service{Store: &repository.Store{DB: db}, Secret: []byte(c.Secret)}
	if *check {
		slog.Info("PostgreSQL connection and migrations OK", "database", "imaskop")
		return nil
	}
	if *create {
		if *role != "CUSTOMER" && *role != "ADMIN" {
			return errors.New("provision role must be CUSTOMER or ADMIN")
		}
		_, err = s.CreateUser(ctx, service.Credentials{Email: *email, Password: os.Getenv("PROVISION_PASSWORD"), Name: *name}, model.UserRole(*role))
		os.Unsetenv("PROVISION_PASSWORD")
		if err != nil {
			return errors.New("account provisioning failed; inspect email uniqueness and password length (12-72 bytes)")
		}
		slog.Info("account provisioned")
		return nil
	}
	api := &handler.API{Service: s, Origin: c.Origin}
	server := &http.Server{Addr: c.Address, Handler: api.Routes(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16 << 10}
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		server.Shutdown(shutdown)
	}()
	slog.Info("HTTP API ready", "address", c.Address, "environment", c.Environment)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
