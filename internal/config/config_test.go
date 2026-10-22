package config

import (
	"strings"
	"testing"
)

func TestPlatformFromEnvAllowsLocalDevelopmentDefaults(t *testing.T) {
	t.Setenv("APP_ENVIRONMENT", "local")
	t.Setenv("DATABASE_URL", "postgres://local")
	t.Setenv("JWT_SECRET", "")
	t.Setenv("SESSION_COOKIE_SECURE", "")
	t.Setenv("CORS_ALLOWED_ORIGINS", "")

	cfg, err := PlatformFromEnv()
	if err != nil {
		t.Fatalf("PlatformFromEnv() error = %v", err)
	}
	if cfg.Environment != "local" || cfg.JWTSecret != developmentJWTSecret || cfg.CookieSecure {
		t.Fatalf("unexpected local configuration: %+v", cfg)
	}
}

func TestPlatformFromEnvRejectsUnsafeExternalConfiguration(t *testing.T) {
	tests := []struct {
		name     string
		secret   string
		secure   string
		origins  string
		contains string
	}{
		{name: "development secret", secret: developmentJWTSecret, secure: "true", origins: "https://staging.example.com", contains: "JWT_SECRET"},
		{name: "short secret", secret: "too-short", secure: "true", origins: "https://staging.example.com", contains: "JWT_SECRET"},
		{name: "insecure cookie", secret: strings.Repeat("s", 32), secure: "false", origins: "https://staging.example.com", contains: "SESSION_COOKIE_SECURE"},
		{name: "http origin", secret: strings.Repeat("s", 32), secure: "true", origins: "http://staging.example.com", contains: "CORS_ALLOWED_ORIGINS"},
		{name: "origin with path", secret: strings.Repeat("s", 32), secure: "true", origins: "https://staging.example.com/app", contains: "CORS_ALLOWED_ORIGINS"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("APP_ENVIRONMENT", "staging")
			t.Setenv("DATABASE_URL", "postgres://staging")
			t.Setenv("JWT_SECRET", tt.secret)
			t.Setenv("SESSION_COOKIE_SECURE", tt.secure)
			t.Setenv("CORS_ALLOWED_ORIGINS", tt.origins)

			_, err := PlatformFromEnv()
			if err == nil || !strings.Contains(err.Error(), tt.contains) {
				t.Fatalf("PlatformFromEnv() error = %v, want error containing %q", err, tt.contains)
			}
		})
	}
}

func TestPlatformFromEnvAcceptsHardenedExternalConfiguration(t *testing.T) {
	t.Setenv("APP_ENVIRONMENT", "production")
	t.Setenv("DATABASE_URL", "postgres://production")
	t.Setenv("JWT_SECRET", strings.Repeat("p", 32))
	t.Setenv("SESSION_COOKIE_SECURE", "true")
	t.Setenv("CORS_ALLOWED_ORIGINS", "https://app.example.com, https://partner.example.com")

	cfg, err := PlatformFromEnv()
	if err != nil {
		t.Fatalf("PlatformFromEnv() error = %v", err)
	}
	if !cfg.CookieSecure || len(cfg.AllowedOrigins) != 2 {
		t.Fatalf("unexpected production configuration: %+v", cfg)
	}
}

func TestPlatformFromEnvRejectsUnknownEnvironment(t *testing.T) {
	t.Setenv("APP_ENVIRONMENT", "prod")
	t.Setenv("DATABASE_URL", "postgres://production")

	_, err := PlatformFromEnv()
	if err == nil || !strings.Contains(err.Error(), "APP_ENVIRONMENT") {
		t.Fatalf("PlatformFromEnv() error = %v, want APP_ENVIRONMENT validation error", err)
	}
}
