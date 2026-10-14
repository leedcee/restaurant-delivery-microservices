// Package analytics asynchronously copies anonymized domain events to ClickHouse.
package analytics

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const maxAttempts = 10

type Worker struct {
	db       *pgxpool.Pool
	logger   *slog.Logger
	client   *http.Client
	endpoint string
	username string
	password string
}

type event struct {
	ID         uuid.UUID
	Name       string
	Version    int
	TraceID    *string
	Properties json.RawMessage
	OccurredAt time.Time
	Attempts   int
}

func NewWorker(db *pgxpool.Pool, logger *slog.Logger, endpoint, username, password string) *Worker {
	return &Worker{db: db, logger: logger, client: &http.Client{Timeout: 5 * time.Second}, endpoint: strings.TrimRight(endpoint, "/"), username: username, password: password}
}

func (w *Worker) Run(ctx context.Context) {
	if w.endpoint == "" {
		return
	}
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := w.processOne(ctx); err != nil && !errors.Is(err, pgx.ErrNoRows) && !errors.Is(err, context.Canceled) {
				w.logger.WarnContext(ctx, "analytics delivery failed", "error", err)
			}
		}
	}
}

func (w *Worker) processOne(ctx context.Context) error {
	item, err := w.claim(ctx)
	if err != nil {
		return err
	}
	row := struct {
		EventID      string `json:"event_id"`
		EventVersion int    `json:"event_version"`
		EventName    string `json:"event_name"`
		OccurredAt   string `json:"occurred_at"`
		TraceID      string `json:"trace_id"`
		Properties   string `json:"properties"`
	}{EventID: item.ID.String(), EventVersion: item.Version, EventName: item.Name, OccurredAt: item.OccurredAt.UTC().Format("2006-01-02 15:04:05.000"), Properties: string(item.Properties)}
	if item.TraceID != nil {
		row.TraceID = *item.TraceID
	}
	body, err := json.Marshal(row)
	if err != nil {
		return w.fail(ctx, item, err)
	}
	query := url.QueryEscape("INSERT INTO analytics.raw_events FORMAT JSONEachRow")
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, w.endpoint+"/?query="+query, bytes.NewReader(append(body, '\n')))
	if err != nil {
		return w.fail(ctx, item, err)
	}
	req.SetBasicAuth(w.username, w.password)
	response, err := w.client.Do(req)
	if err != nil {
		return w.fail(ctx, item, err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return w.fail(ctx, item, fmt.Errorf("clickhouse returned status %d", response.StatusCode))
	}
	_, err = w.db.Exec(ctx, `UPDATE analytics_events SET processed_at = now(), last_error = NULL WHERE id = $1`, item.ID)
	return err
}

func (w *Worker) claim(ctx context.Context) (event, error) {
	var item event
	err := w.db.QueryRow(ctx, `WITH candidate AS (
		SELECT id FROM analytics_events
		WHERE processed_at IS NULL AND dead_lettered_at IS NULL AND available_at <= now()
		ORDER BY occurred_at FOR UPDATE SKIP LOCKED LIMIT 1
	)
	UPDATE analytics_events e SET attempts = e.attempts + 1, available_at = now() + interval '30 seconds'
	FROM candidate WHERE e.id = candidate.id
	RETURNING e.id, e.event_name, e.event_version, e.trace_id, e.properties, e.occurred_at, e.attempts`).
		Scan(&item.ID, &item.Name, &item.Version, &item.TraceID, &item.Properties, &item.OccurredAt, &item.Attempts)
	return item, err
}

func (w *Worker) fail(ctx context.Context, item event, cause error) error {
	if item.Attempts >= maxAttempts {
		_, err := w.db.Exec(ctx, `UPDATE analytics_events SET last_error = $1, dead_lettered_at = now() WHERE id = $2`, cause.Error(), item.ID)
		return errors.Join(cause, err)
	}
	_, err := w.db.Exec(ctx, `UPDATE analytics_events SET last_error = $1, available_at = now() + make_interval(secs => $2) WHERE id = $3`, cause.Error(), min(item.Attempts*item.Attempts, 60), item.ID)
	return errors.Join(cause, err)
}
