package config

import (
	"os"
	"path/filepath"
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

func TestPlatformFromEnvReadsSecretsFromFiles(t *testing.T) {
	directory := t.TempDir()
	databaseFile := filepath.Join(directory, "database-url")
	jwtFile := filepath.Join(directory, "jwt-secret")
	clickHouseFile := filepath.Join(directory, "clickhouse-password")
	for path, value := range map[string]string{
		databaseFile:   "postgres://from-secret-file\n",
		jwtFile:        strings.Repeat("j", 40) + "\n",
		clickHouseFile: "clickhouse-from-file\n",
	} {
		if err := os.WriteFile(path, []byte(value), 0o600); err != nil {
			t.Fatalf("write test secret: %v", err)
		}
	}
	t.Setenv("APP_ENVIRONMENT", "staging")
	t.Setenv("DATABASE_URL", "")
	t.Setenv("DATABASE_URL_FILE", databaseFile)
	t.Setenv("JWT_SECRET", "")
	t.Setenv("JWT_SECRET_FILE", jwtFile)
	t.Setenv("CLICKHOUSE_PASSWORD", "")
	t.Setenv("CLICKHOUSE_PASSWORD_FILE", clickHouseFile)
	t.Setenv("SESSION_COOKIE_SECURE", "true")
	t.Setenv("CORS_ALLOWED_ORIGINS", "https://staging.example.com")

	cfg, err := PlatformFromEnv()
	if err != nil {
		t.Fatalf("PlatformFromEnv() error = %v", err)
	}
	if cfg.DatabaseURL != "postgres://from-secret-file" || cfg.JWTSecret != strings.Repeat("j", 40) || cfg.ClickHousePassword != "clickhouse-from-file" {
		t.Fatalf("unexpected file-backed secrets: %+v", cfg)
	}
}

func TestPlatformFromEnvRejectsAmbiguousSecretSource(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://environment")
	t.Setenv("DATABASE_URL_FILE", "database-url.txt")
	_, err := PlatformFromEnv()
	if err == nil || !strings.Contains(err.Error(), "DATABASE_URL") {
		t.Fatalf("PlatformFromEnv() error = %v, want DATABASE_URL ambiguity", err)
	}
}

func TestDemoRestaurantFromEnvReadsSecretsFromFiles(t *testing.T) {
	directory := t.TempDir()
	apiKeyFile := filepath.Join(directory, "api-key")
	partnerKeysFile := filepath.Join(directory, "partner-keys")
	if err := os.WriteFile(apiKeyFile, []byte("demo-from-file\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(partnerKeysFile, []byte("restaurant-id=partner-from-file\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DEMO_API_KEY", "")
	t.Setenv("DEMO_API_KEY_FILE", apiKeyFile)
	t.Setenv("DEMO_PARTNER_API_KEYS", "")
	t.Setenv("DEMO_PARTNER_API_KEYS_FILE", partnerKeysFile)

	cfg, err := DemoRestaurantFromEnv()
	if err != nil {
		t.Fatalf("DemoRestaurantFromEnv() error = %v", err)
	}
	if cfg.APIKey != "demo-from-file" || cfg.PartnerAPIKeys["restaurant-id"] != "partner-from-file" {
		t.Fatalf("unexpected demo secret configuration: %+v", cfg)
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
