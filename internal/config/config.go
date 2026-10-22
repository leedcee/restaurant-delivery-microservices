// Package config loads service configuration from environment variables.
package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
)

const developmentJWTSecret = "dev-only-change-me"

// Platform contains configuration for the main API service.
type Platform struct {
	Environment    string
	HTTPAddr       string
	DatabaseURL    string
	JWTSecret      string
	CookieSecure   bool
	AllowedOrigins []string
}

// DemoRestaurant contains configuration for the example partner service.
type DemoRestaurant struct {
	HTTPAddr        string
	PlatformBaseURL string
	APIKey          string
	PartnerAPIKeys  map[string]string
}

// PlatformFromEnv returns validated main API configuration.
func PlatformFromEnv() (Platform, error) {
	cfg := Platform{
		Environment:    strings.ToLower(valueOrDefault("APP_ENVIRONMENT", "local")),
		HTTPAddr:       valueOrDefault("PLATFORM_HTTP_ADDR", ":8080"),
		DatabaseURL:    os.Getenv("DATABASE_URL"),
		JWTSecret:      valueOrDefault("JWT_SECRET", developmentJWTSecret),
		AllowedOrigins: splitList(valueOrDefault("CORS_ALLOWED_ORIGINS", "http://localhost:5173,http://127.0.0.1:5173")),
	}
	secure, err := strconv.ParseBool(valueOrDefault("SESSION_COOKIE_SECURE", "false"))
	if err != nil {
		return Platform{}, fmt.Errorf("SESSION_COOKIE_SECURE must be true or false")
	}
	cfg.CookieSecure = secure
	if cfg.DatabaseURL == "" {
		return Platform{}, fmt.Errorf("DATABASE_URL is required")
	}
	if err := cfg.validate(); err != nil {
		return Platform{}, err
	}
	return cfg, nil
}

func (cfg Platform) validate() error {
	switch cfg.Environment {
	case "local", "test":
		return nil
	case "staging", "production":
	default:
		return fmt.Errorf("APP_ENVIRONMENT must be one of local, test, staging, production")
	}

	if cfg.JWTSecret == developmentJWTSecret || len(cfg.JWTSecret) < 32 {
		return fmt.Errorf("JWT_SECRET must contain at least 32 characters and must not use the development default in %s", cfg.Environment)
	}
	if !cfg.CookieSecure {
		return fmt.Errorf("SESSION_COOKIE_SECURE must be true in %s", cfg.Environment)
	}
	if len(cfg.AllowedOrigins) == 0 {
		return fmt.Errorf("CORS_ALLOWED_ORIGINS must contain at least one HTTPS origin in %s", cfg.Environment)
	}
	for _, origin := range cfg.AllowedOrigins {
		parsed, err := url.Parse(origin)
		if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil || parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" {
			return fmt.Errorf("CORS_ALLOWED_ORIGINS must contain only HTTPS origins without paths in %s: %q", cfg.Environment, origin)
		}
	}
	return nil
}

func splitList(value string) []string {
	result := make([]string, 0)
	for _, item := range strings.Split(value, ",") {
		if item = strings.TrimSpace(item); item != "" {
			result = append(result, item)
		}
	}
	return result
}

// DemoRestaurantFromEnv returns demo restaurant configuration.
func DemoRestaurantFromEnv() DemoRestaurant {
	return DemoRestaurant{
		HTTPAddr:        valueOrDefault("DEMO_HTTP_ADDR", ":8081"),
		PlatformBaseURL: valueOrDefault("PLATFORM_BASE_URL", "http://localhost:8080"),
		APIKey:          valueOrDefault("DEMO_API_KEY", "demo-secret"),
		PartnerAPIKeys:  parseKeyMap(os.Getenv("DEMO_PARTNER_API_KEYS")),
	}
}

func parseKeyMap(value string) map[string]string {
	result := make(map[string]string)
	for _, pair := range strings.Split(value, ",") {
		id, key, ok := strings.Cut(pair, "=")
		id, key = strings.TrimSpace(id), strings.TrimSpace(key)
		if ok && id != "" && key != "" {
			result[id] = key
		}
	}
	return result
}

func valueOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
