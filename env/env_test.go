package env_test

import (
	"errors"
	"testing"

	"github.com/mbriggs/go-bootstrap/env"
)

func TestLoadDefaultsToDevelopment(t *testing.T) {
	t.Setenv("APP_ENV", "")
	t.Setenv("PORT", "")

	cfg, err := env.Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.AppEnv != env.Development || !cfg.Dev() {
		t.Fatalf("AppEnv = %q, want development", cfg.AppEnv)
	}
	if cfg.Port != "8080" {
		t.Fatalf("Port = %q, want 8080", cfg.Port)
	}
}

func TestLoadRejectsUnknownAppEnv(t *testing.T) {
	t.Setenv("APP_ENV", "staging")

	_, err := env.Load()
	if !errors.Is(err, env.ErrBadAppEnv) {
		t.Fatalf("err = %v, want ErrBadAppEnv", err)
	}
}

func TestLoadReadsOverrides(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("PORT", "9999")
	t.Setenv("APP_URL", "https://example.com")

	cfg, err := env.Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if !cfg.Production() || cfg.Port != "9999" {
		t.Fatalf("cfg = %+v, want production on 9999", cfg)
	}
	if cfg.BaseURL != "https://example.com" {
		t.Fatalf("BaseURL = %q, want https://example.com", cfg.BaseURL)
	}
}

func TestLoadDefaultsBaseURLToLocalhostInDev(t *testing.T) {
	t.Setenv("APP_ENV", "")
	t.Setenv("PORT", "7777")
	t.Setenv("APP_URL", "")

	cfg, err := env.Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.BaseURL != "http://localhost:7777" {
		t.Fatalf("BaseURL = %q, want localhost fallback on PORT", cfg.BaseURL)
	}
}

func TestLoadRequiresAppURLInProduction(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("APP_URL", "")

	_, err := env.Load()
	if !errors.Is(err, env.ErrAppURLRequired) {
		t.Fatalf("err = %v, want ErrAppURLRequired", err)
	}
}
