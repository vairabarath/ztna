package config

import (
	"os"
	"strconv"
	"time"
)

// Config holds process-level settings for the controller.
type Config struct {
	ListenAddr      string
	ShutdownTimeout time.Duration
	CAEncKeyB64     string
	DatabaseDSN     string
	AdminToken      string
	TLSEnabled      bool
	TLSCertFile     string
	TLSKeyFile      string
}

func FromEnv() Config {
	return Config{
		ListenAddr:      getEnv("CONTROLLER_LISTEN_ADDR", ":8443"),
		ShutdownTimeout: getDurationEnv("CONTROLLER_SHUTDOWN_TIMEOUT", 10*time.Second),
		// Development default only. Must be overridden in production.
		CAEncKeyB64: getEnv("CONTROLLER_CA_ENC_KEY_B64", "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY="),
		DatabaseDSN: getEnv("CONTROLLER_DB_DSN", "postgres://ztna:ztna@localhost:5432/ztna?sslmode=disable"),
		// Development default only. Must be overridden in production.
		AdminToken:  getEnv("CONTROLLER_ADMIN_TOKEN", "dev-admin-token"),
		TLSEnabled:  getBoolEnv("CONTROLLER_TLS_ENABLED", false),
		TLSCertFile: getEnv("CONTROLLER_TLS_CERT_FILE", "../deploy/tls/controller.crt"),
		TLSKeyFile:  getEnv("CONTROLLER_TLS_KEY_FILE", "../deploy/tls/controller.key"),
	}
}

func getEnv(key, fallback string) string {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	return v
}

func getDurationEnv(key string, fallback time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err == nil {
		return d
	}

	// Allow plain integer seconds for convenience.
	sec, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return time.Duration(sec) * time.Second
}

func getBoolEnv(key string, fallback bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(v)
	if err != nil {
		return fallback
	}
	return parsed
}
