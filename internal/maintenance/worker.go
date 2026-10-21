// Package maintenance removes expired operational event records in bounded batches.
package maintenance

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Config struct {
	Interval            time.Duration
	ProcessedRetention  time.Duration
	DeadLetterRetention time.Duration
	BatchSize           int
}

func ConfigFromEnv() (Config, error) {
	interval, err := durationFromEnv("EVENT_CLEANUP_INTERVAL", time.Hour)
	if err != nil {
		return Config{}, err
	}
	processed, err := durationFromEnv("PROCESSED_EVENT_RETENTION", 7*24*time.Hour)
	if err != nil {
		return Config{}, err
	}
	deadLetter, err := durationFromEnv("DEAD_LETTER_RETENTION", 30*24*time.Hour)
	if err != nil {
		return Config{}, err
	}
	batchSize := 1000
	if value := os.Getenv("EVENT_CLEANUP_BATCH_SIZE"); value != "" {
		batchSize, err = strconv.Atoi(value)
		if err != nil || batchSize < 1 || batchSize > 10000 {
			return Config{}, fmt.Errorf("EVENT_CLEANUP_BATCH_SIZE must be between 1 and 10000")
		}
	}
	return Config{Interval: interval, ProcessedRetention: processed, DeadLetterRetention: deadLetter, BatchSize: batchSize}, nil
}

func durationFromEnv(key string, fallback time.Duration) (time.Duration, error) {
	value := os.Getenv(key)
	if value == "" {
		return fallback, nil
	}
	duration, err := time.ParseDuration(value)
	if err != nil || duration <= 0 {
		return 0, fmt.Errorf("%s must be a positive duration", key)
	}
	return duration, nil
}

type Recorder interface {
	RecordEventCleanup(queue, disposition string, deleted int)
	RecordEventCleanupRun(success bool)
}

type Worker struct {
	db       *pgxpool.Pool
	logger   *slog.Logger
	config   Config
	recorder Recorder
}

func NewWorker(db *pgxpool.Pool, logger *slog.Logger, config Config, recorder Recorder) *Worker {
	return &Worker{db: db, logger: logger, config: config, recorder: recorder}
}

func (w *Worker) Run(ctx context.Context) {
	w.cleanup(ctx)
	ticker := time.NewTicker(w.config.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.cleanup(ctx)
		}
	}
}

type cleanupTarget struct {
	queue       string
	disposition string
	retention   time.Duration
	query       string
}

func (w *Worker) cleanup(ctx context.Context) {
	targets := []cleanupTarget{
		{queue: "outbox", disposition: "processed", retention: w.config.ProcessedRetention, query: deleteProcessedOutbox},
		{queue: "outbox", disposition: "dead_letter", retention: w.config.DeadLetterRetention, query: deleteDeadLetterOutbox},
		{queue: "analytics", disposition: "processed", retention: w.config.ProcessedRetention, query: deleteProcessedAnalytics},
		{queue: "analytics", disposition: "dead_letter", retention: w.config.DeadLetterRetention, query: deleteDeadLetterAnalytics},
	}
	total := 0
	for _, target := range targets {
		deleted, err := w.deleteBatches(ctx, target)
		if err != nil {
			w.recorder.RecordEventCleanupRun(false)
			w.logger.ErrorContext(ctx, "event cleanup failed", "queue", target.queue, "disposition", target.disposition, "error", err)
			return
		}
		total += deleted
		w.recorder.RecordEventCleanup(target.queue, target.disposition, deleted)
	}
	w.recorder.RecordEventCleanupRun(true)
	w.logger.InfoContext(ctx, "event cleanup completed", "deleted", total)
}

func (w *Worker) deleteBatches(ctx context.Context, target cleanupTarget) (int, error) {
	total := 0
	for range 100 {
		result, err := w.db.Exec(ctx, target.query, target.retention.Seconds(), w.config.BatchSize)
		if err != nil {
			return total, err
		}
		deleted := int(result.RowsAffected())
		total += deleted
		if deleted < w.config.BatchSize {
			return total, nil
		}
	}
	return total, nil
}

const deleteProcessedOutbox = `WITH expired AS (
	SELECT id FROM outbox_events WHERE processed_at < now() - make_interval(secs => $1)
	ORDER BY processed_at FOR UPDATE SKIP LOCKED LIMIT $2)
DELETE FROM outbox_events e USING expired WHERE e.id = expired.id`
const deleteDeadLetterOutbox = `WITH expired AS (
	SELECT id FROM outbox_events WHERE dead_lettered_at < now() - make_interval(secs => $1)
	ORDER BY dead_lettered_at FOR UPDATE SKIP LOCKED LIMIT $2)
DELETE FROM outbox_events e USING expired WHERE e.id = expired.id`
const deleteProcessedAnalytics = `WITH expired AS (
	SELECT id FROM analytics_events WHERE processed_at < now() - make_interval(secs => $1)
	ORDER BY processed_at FOR UPDATE SKIP LOCKED LIMIT $2)
DELETE FROM analytics_events e USING expired WHERE e.id = expired.id`
const deleteDeadLetterAnalytics = `WITH expired AS (
	SELECT id FROM analytics_events WHERE dead_lettered_at < now() - make_interval(secs => $1)
	ORDER BY dead_lettered_at FOR UPDATE SKIP LOCKED LIMIT $2)
DELETE FROM analytics_events e USING expired WHERE e.id = expired.id`
