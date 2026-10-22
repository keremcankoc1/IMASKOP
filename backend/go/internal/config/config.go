package config

import (
	"bufio"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
)

type Config struct{ DatabaseURL, Secret, Address, Origin, Environment string }

// LoadEnv only supplies missing variables; deployment environment takes precedence.
func LoadEnv(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	s := bufio.NewScanner(f)
	for s.Scan() {
		line := strings.TrimSpace(strings.TrimPrefix(s.Text(), "\uFEFF"))
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			return fmt.Errorf("invalid environment file line")
		}
		k = strings.TrimSpace(k)
		if _, set := os.LookupEnv(k); !set {
			os.Setenv(k, strings.Trim(strings.TrimSpace(v), "\"'"))
		}
	}
	return s.Err()
}

func Load() (Config, error) {
	c := Config{DatabaseURL: os.Getenv("DATABASE_URL"), Secret: os.Getenv("AUTH_SECRET"), Address: os.Getenv("HTTP_ADDR"), Origin: os.Getenv("CORS_ORIGIN"), Environment: os.Getenv("APP_ENV")}
	if c.Address == "" {
		c.Address = "127.0.0.1:8080"
	}
	if c.Environment == "" {
		c.Environment = "development"
	}
	if c.DatabaseURL == "" {
		return c, errors.New("DATABASE_URL is required")
	}
	if len(c.Secret) < 32 {
		return c, errors.New("AUTH_SECRET must contain at least 32 characters")
	}
	if c.Environment != "development" && c.Environment != "production" {
		return c, errors.New("APP_ENV must be development or production")
	}
	if c.Origin == "*" {
		return c, errors.New("CORS_ORIGIN must be an exact origin")
	}
	if c.Environment == "production" {
		host, _, err := net.SplitHostPort(c.Address)
		if err != nil || host == "" {
			return c, errors.New("production HTTP_ADDR must name an explicit interface; use a TLS reverse proxy")
		}
	}
	return c, nil
}
