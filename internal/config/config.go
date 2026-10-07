// Package config loads service configuration from environment variables.
package config

import (
	"fmt"
	"os"
	"strings"
)

// Platform contains configuration for the main API service.
type Platform struct {
	HTTPAddr    string
	DatabaseURL string
	JWTSecret   string
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
		HTTPAddr:    valueOrDefault("PLATFORM_HTTP_ADDR", ":8080"),
		DatabaseURL: os.Getenv("DATABASE_URL"),
		JWTSecret:   valueOrDefault("JWT_SECRET", "dev-only-change-me"),
	}
	if cfg.DatabaseURL == "" {
		return Platform{}, fmt.Errorf("DATABASE_URL is required")
	}
	return cfg, nil
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
