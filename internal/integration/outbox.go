// Package integration delivers durable outbox events to partner services.
package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"strconv"
	"sync"
	"time"

	"restaurant-delivery-system/internal/domain"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

type Worker struct {
	db     *pgxpool.Pool
	client *http.Client
	logger *slog.Logger
}

const (
	outboxConcurrency = 4
	outboxMaxAttempts = 8
)

func NewWorker(db *pgxpool.Pool, logger *slog.Logger) *Worker {
	return &Worker{db: db, logger: logger, client: &http.Client{Timeout: 5 * time.Second, Transport: otelhttp.NewTransport(http.DefaultTransport)}}
}

func (w *Worker) Run(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.processBatch(ctx)
		}
	}
}

func (w *Worker) processBatch(ctx context.Context) {
	var group sync.WaitGroup
	for range outboxConcurrency {
		group.Add(1)
		go func() {
			defer group.Done()
			if err := w.processOne(ctx); err != nil && !errors.Is(err, pgx.ErrNoRows) && !errors.Is(err, context.Canceled) {
				w.logger.Warn("outbox delivery failed", "error", err)
			}
		}()
	}
	group.Wait()
}

type delivery struct {
	EventID       uuid.UUID
	EventType     string
	EventVersion  int
	CorrelationID uuid.UUID
	Attempts      int
	Endpoint      string
	Order         domain.Order
}

func (w *Worker) processOne(ctx context.Context) (returnErr error) {
	item, err := w.claim(ctx)
	if err != nil {
		return err
	}
	ctx, span := otel.Tracer("platform-api/outbox").Start(ctx, "deliver "+item.EventType)
	span.SetAttributes(attribute.String("messaging.message.id", item.EventID.String()), attribute.String("messaging.operation.name", "deliver"))
	defer func() {
		if returnErr != nil {
			span.RecordError(returnErr)
			span.SetStatus(codes.Error, returnErr.Error())
		}
		span.End()
	}()
	body, err := json.Marshal(item.Order)
	if err != nil {
		return fmt.Errorf("marshal delivery: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, item.Endpoint, bytes.NewReader(body))
	if err != nil {
		return w.fail(ctx, item, fmt.Errorf("create partner request: %w", err))
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", item.EventID.String())
	req.Header.Set("X-Event-ID", item.EventID.String())
	req.Header.Set("X-Event-Type", item.EventType)
	req.Header.Set("X-Event-Version", strconv.Itoa(item.EventVersion))
	req.Header.Set("X-Correlation-ID", item.CorrelationID.String())
	response, err := w.client.Do(req)
	if err != nil {
		return w.fail(ctx, item, fmt.Errorf("send partner request: %w", err))
	}
	if closeErr := response.Body.Close(); closeErr != nil {
		return w.fail(ctx, item, fmt.Errorf("close partner response: %w", closeErr))
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return w.fail(ctx, item, fmt.Errorf("partner returned status %d", response.StatusCode))
	}
	_, err = w.db.Exec(ctx, `UPDATE outbox_events SET processed_at = now(), last_error = NULL WHERE id = $1`, item.EventID)
	if err != nil {
		return fmt.Errorf("complete outbox event: %w", err)
	}
	w.logger.InfoContext(ctx, "order delivered to restaurant", "orderId", item.Order.ID)
	return nil
}

func (w *Worker) claim(ctx context.Context) (delivery, error) {
	tx, err := w.db.Begin(ctx)
	if err != nil {
		return delivery{}, fmt.Errorf("begin outbox claim: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var item delivery
	err = tx.QueryRow(ctx, `
		SELECT e.id, e.event_type, e.event_version, e.correlation_id, e.attempts,
		       e.aggregate_id, i.order_endpoint
		FROM outbox_events e
		JOIN orders o ON o.id = e.aggregate_id
		JOIN restaurant_integrations i ON i.restaurant_id = o.restaurant_id
		WHERE e.processed_at IS NULL AND e.dead_lettered_at IS NULL
		  AND e.available_at <= now() AND e.event_type = 'order.created'
		ORDER BY e.created_at FOR UPDATE OF e SKIP LOCKED LIMIT 1`).
		Scan(&item.EventID, &item.EventType, &item.EventVersion, &item.CorrelationID, &item.Attempts,
			&item.Order.ID, &item.Endpoint)
	if err != nil {
		return delivery{}, err
	}
	item.Attempts++
	_, err = tx.Exec(ctx, `UPDATE outbox_events SET attempts = $2,
		available_at = now() + interval '30 seconds' WHERE id = $1`, item.EventID, item.Attempts)
	if err != nil {
		return delivery{}, fmt.Errorf("claim outbox event: %w", err)
	}
	err = tx.QueryRow(ctx, `SELECT id, user_id, restaurant_id, delivery_fee_minor, total_minor, currency, status,
		version, delivery_address, rejection_reason, status_reason_code, created_at FROM orders WHERE id = $1`, item.Order.ID).
		Scan(&item.Order.ID, &item.Order.UserID, &item.Order.RestaurantID, &item.Order.DeliveryFeeMinor, &item.Order.TotalMinor,
			&item.Order.Currency, &item.Order.Status, &item.Order.Version, &item.Order.DeliveryAddress,
			&item.Order.RejectionReason, &item.Order.StatusReasonCode, &item.Order.CreatedAt)
	if err != nil {
		return delivery{}, fmt.Errorf("load outbox order: %w", err)
	}
	rows, err := tx.Query(ctx, `SELECT product_id, product_name, quantity, unit_price_minor, total_minor
		FROM order_items WHERE order_id = $1`, item.Order.ID)
	if err != nil {
		return delivery{}, fmt.Errorf("load outbox items: %w", err)
	}
	defer rows.Close()
	item.Order.Items = make([]domain.QuoteItem, 0)
	for rows.Next() {
		var orderItem domain.QuoteItem
		if err := rows.Scan(&orderItem.ProductID, &orderItem.Name, &orderItem.Quantity,
			&orderItem.UnitPriceMinor, &orderItem.TotalMinor); err != nil {
			return delivery{}, fmt.Errorf("scan outbox item: %w", err)
		}
		item.Order.Items = append(item.Order.Items, orderItem)
	}
	if err := rows.Err(); err != nil {
		return delivery{}, fmt.Errorf("iterate outbox items: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return delivery{}, fmt.Errorf("commit outbox claim: %w", err)
	}
	return item, nil
}

func (w *Worker) fail(ctx context.Context, item delivery, cause error) error {
	var err error
	if item.Attempts >= outboxMaxAttempts {
		_, err = w.db.Exec(ctx, `UPDATE outbox_events
			SET last_error = $1, dead_lettered_at = now() WHERE id = $2`, cause.Error(), item.EventID)
		if err == nil {
			w.logger.ErrorContext(ctx, "outbox event moved to dead letter", "eventId", item.EventID,
				"eventType", item.EventType, "attempts", item.Attempts, "error", cause)
		}
	} else {
		delay := retryDelay(item.Attempts, rand.Float64())
		_, err = w.db.Exec(ctx, `UPDATE outbox_events
			SET last_error = $1, available_at = now() + make_interval(secs => $3)
			WHERE id = $2`, cause.Error(), item.EventID, delay.Seconds())
	}
	if err != nil {
		return errors.Join(cause, fmt.Errorf("record outbox failure: %w", err))
	}
	return cause
}

func retryDelay(attempt int, jitter float64) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	if attempt > 6 {
		attempt = 6
	}
	base := time.Second * time.Duration(1<<uint(attempt))
	return base + time.Duration(float64(base/2)*jitter)
}
