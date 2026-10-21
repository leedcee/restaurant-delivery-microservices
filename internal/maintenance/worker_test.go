package maintenance

import (
	"testing"
	"time"
)

func TestConfigFromEnv(t *testing.T) {
	t.Setenv("EVENT_CLEANUP_INTERVAL", "15m")
	t.Setenv("PROCESSED_EVENT_RETENTION", "48h")
	t.Setenv("DEAD_LETTER_RETENTION", "720h")
	t.Setenv("EVENT_CLEANUP_BATCH_SIZE", "250")
	config, err := ConfigFromEnv()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if config.Interval != 15*time.Minute || config.ProcessedRetention != 48*time.Hour || config.DeadLetterRetention != 720*time.Hour || config.BatchSize != 250 {
		t.Fatalf("unexpected config: %+v", config)
	}
}

func TestConfigRejectsInvalidValues(t *testing.T) {
	t.Setenv("EVENT_CLEANUP_INTERVAL", "never")
	if _, err := ConfigFromEnv(); err == nil {
		t.Fatal("expected invalid duration error")
	}
}
