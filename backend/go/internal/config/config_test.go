package config

import "testing"

func TestRenderPortAndExplicitAddress(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgresql://example.invalid/imaskop")
	t.Setenv("AUTH_SECRET", "test-secret-with-at-least-32-characters")
	t.Setenv("APP_ENV", "production")
	t.Setenv("CORS_ORIGIN", "")
	t.Setenv("HTTP_ADDR", "")
	t.Setenv("PORT", "10000")
	c, err := Load()
	if err != nil || c.Address != "0.0.0.0:10000" {
		t.Fatalf("Render port not selected: %v", err)
	}
	t.Setenv("HTTP_ADDR", "127.0.0.1:9000")
	c, err = Load()
	if err != nil || c.Address != "127.0.0.1:9000" {
		t.Fatalf("explicit address not preserved: %v", err)
	}
}
