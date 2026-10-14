package integration

import (
	"testing"
	"time"
)

func TestRetryDelay(t *testing.T) {
	t.Parallel()
	tests := []struct {
		attempt int
		min     time.Duration
		max     time.Duration
	}{
		{attempt: 1, min: 2 * time.Second, max: 3 * time.Second},
		{attempt: 3, min: 8 * time.Second, max: 12 * time.Second},
		{attempt: 8, min: 64 * time.Second, max: 96 * time.Second},
	}
	for _, test := range tests {
		if got := retryDelay(test.attempt, 0.5); got < test.min || got > test.max {
			t.Fatalf("retryDelay(%d) = %s, want %s..%s", test.attempt, got, test.min, test.max)
		}
	}
}
