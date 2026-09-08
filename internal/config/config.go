// Package config loads service configuration from environment variables.
package config

import (
	"fmt"
	"os"
)

// Kitchen contains configuration for the main API service.
type Kitchen struct {
	HTTPAddr    string
	DatabaseURL string
}

// DemoRestaurant contains configuration for the example partner service.
type DemoRestaurant struct {
	HTTPAddr       string
	KitchenBaseURL string
	APIKey         string
}

// KitchenFromEnv returns validated main API configuration.
func KitchenFromEnv() (Kitchen, error) {
	cfg := Kitchen{
		HTTPAddr:    valueOrDefault("KITCHEN_HTTP_ADDR", ":8080"),
		DatabaseURL: os.Getenv("DATABASE_URL"),
	}
	if cfg.DatabaseURL == "" {
		return Kitchen{}, fmt.Errorf("DATABASE_URL is required")
	}
	return cfg, nil
}

// DemoRestaurantFromEnv returns demo restaurant configuration.
func DemoRestaurantFromEnv() DemoRestaurant {
	return DemoRestaurant{
		HTTPAddr:       valueOrDefault("DEMO_HTTP_ADDR", ":8081"),
		KitchenBaseURL: valueOrDefault("KITCHEN_BASE_URL", "http://localhost:8080"),
		APIKey:         valueOrDefault("DEMO_API_KEY", "demo-secret"),
	}
}

func valueOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
