// Package integration delivers durable outbox events to partner services.
package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"avito-kitchen/internal/domain"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Worker struct {
	db     *pgxpool.Pool
	client *http.Client
	logger *slog.Logger
}

func NewWorker(db *pgxpool.Pool, logger *slog.Logger) *Worker {
	return &Worker{db: db, logger: logger, client: &http.Client{Timeout: 5 * time.Second}}
}

func (w *Worker) Run(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := w.processOne(ctx); err != nil && !errors.Is(err, pgx.ErrNoRows) {
				w.logger.Warn("outbox delivery failed", "error", err)
			}
		}
	}
}

type delivery struct {
	EventID  uuid.UUID
	Endpoint string
	Order    domain.Order
}

func (w *Worker) processOne(ctx context.Context) error {
	item, err := w.claim(ctx)
	if err != nil {
		return err
	}
	body, err := json.Marshal(item.Order)
	if err != nil {
		return fmt.Errorf("marshal delivery: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, item.Endpoint, bytes.NewReader(body))
	if err != nil {
		return w.fail(ctx, item.EventID, fmt.Errorf("create partner request: %w", err))
	}
	req.Header.Set("Content-Type", "application/json")
	response, err := w.client.Do(req)
	if err != nil {
		return w.fail(ctx, item.EventID, fmt.Errorf("send partner request: %w", err))
	}
	if closeErr := response.Body.Close(); closeErr != nil {
		return w.fail(ctx, item.EventID, fmt.Errorf("close partner response: %w", closeErr))
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return w.fail(ctx, item.EventID, fmt.Errorf("partner returned status %d", response.StatusCode))
	}
	_, err = w.db.Exec(ctx, `UPDATE outbox_events SET processed_at = now(), last_error = NULL WHERE id = $1`, item.EventID)
	if err != nil {
		return fmt.Errorf("complete outbox event: %w", err)
	}
	w.logger.Info("order delivered to restaurant", "orderId", item.Order.ID)
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
		SELECT e.id, e.aggregate_id, i.order_endpoint
		FROM outbox_events e
		JOIN orders o ON o.id = e.aggregate_id
		JOIN restaurant_integrations i ON i.restaurant_id = o.restaurant_id
		WHERE e.processed_at IS NULL AND e.available_at <= now() AND e.event_type = 'order.created'
		ORDER BY e.created_at FOR UPDATE OF e SKIP LOCKED LIMIT 1`).
		Scan(&item.EventID, &item.Order.ID, &item.Endpoint)
	if err != nil {
		return delivery{}, err
	}
	_, err = tx.Exec(ctx, `UPDATE outbox_events SET attempts = attempts + 1,
		available_at = now() + interval '15 seconds' WHERE id = $1`, item.EventID)
	if err != nil {
		return delivery{}, fmt.Errorf("claim outbox event: %w", err)
	}
	err = tx.QueryRow(ctx, `SELECT id, user_id, restaurant_id, total_minor, currency, status,
		delivery_address, rejection_reason, created_at FROM orders WHERE id = $1`, item.Order.ID).
		Scan(&item.Order.ID, &item.Order.UserID, &item.Order.RestaurantID, &item.Order.TotalMinor,
			&item.Order.Currency, &item.Order.Status, &item.Order.DeliveryAddress,
			&item.Order.RejectionReason, &item.Order.CreatedAt)
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

func (w *Worker) fail(ctx context.Context, eventID uuid.UUID, cause error) error {
	_, err := w.db.Exec(ctx, `UPDATE outbox_events SET last_error = $1 WHERE id = $2`, cause.Error(), eventID)
	if err != nil {
		return errors.Join(cause, fmt.Errorf("record outbox failure: %w", err))
	}
	return cause
}
