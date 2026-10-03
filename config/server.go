// Package config reads and validates the ondOTA server's runtime configuration.
package config

import (
	"errors"
	"fmt"
	"strings"

	"github.com/caarlos0/env/v11"
)

// EnvProduction is the APP_ENV value that enables strict validation.
const EnvProduction = "production"

// ServerConfig is the process configuration, read entirely from the
// environment so the same image runs in every environment.
type ServerConfig struct {
	// PublicAddr serves the user API, enrollment and webhooks
	// (ondota.ownerofglory.com). It never trusts forwarded client certificates.
	PublicAddr string `env:"PUBLIC_ADDR" envDefault:"0.0.0.0:8080"`
	// DeviceAddr serves the mTLS-only device API (devices.ondota.ownerofglory.com),
	// reached exclusively through the Traefik router that verifies client certificates.
	DeviceAddr string `env:"DEVICE_ADDR" envDefault:"0.0.0.0:8081"`
	// LogLevel sets the minimum slog level: debug, info, warn or error.
	LogLevel string `env:"LOG_LEVEL" envDefault:"info"`
	// Environment selects environment-specific behaviour; "production" enables strict validation.
	Environment string `env:"APP_ENV" envDefault:"development"`
	// DatabaseURL points at PostgreSQL. It is required in production.
	DatabaseURL string `env:"DATABASE_URL"`
}

// Load parses the configuration from the environment and validates it.
func Load() (ServerConfig, error) {
	cfg, err := env.ParseAs[ServerConfig]()
	if err != nil {
		return ServerConfig{}, fmt.Errorf("parse environment: %w", err)
	}
	return cfg, cfg.Validate()
}

// Validate rejects configurations that cannot run safely.
func (c ServerConfig) Validate() error {
	var errs []error
	if c.PublicAddr == c.DeviceAddr {
		errs = append(errs, errors.New("PUBLIC_ADDR and DEVICE_ADDR must differ"))
	}
	switch strings.ToLower(c.LogLevel) {
	case "debug", "info", "warn", "error":
	default:
		errs = append(errs, fmt.Errorf("LOG_LEVEL %q is not one of debug, info, warn, error", c.LogLevel))
	}
	if c.Environment == EnvProduction && c.DatabaseURL == "" {
		errs = append(errs, errors.New("DATABASE_URL is required in production"))
	}
	return errors.Join(errs...)
}
