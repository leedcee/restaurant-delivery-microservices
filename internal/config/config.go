// Package config loads service configuration from environment variables.
package config

import (
	"fmt"
	"os"
)

// Platform contains configuration for the main API service.
type Platform struct {
	HTTPAddr    string
	DatabaseURL string
}

// DemoRestaurant contains configuration for the example partner service.
type DemoRestaurant struct {
	HTTPAddr        string
	PlatformBaseURL string
	APIKey          string
}

// PlatformFromEnv returns validated main API configuration.
func PlatformFromEnv() (Platform, error) {
	cfg := Platform{
		HTTPAddr:    valueOrDefault("PLATFORM_HTTP_ADDR", ":8080"),
		DatabaseURL: os.Getenv("DATABASE_URL"),
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
	}
}

func valueOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
