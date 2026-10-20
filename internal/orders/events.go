package orders

import (
	"context"
	"fmt"

	"restaurant-delivery-system/internal/domain"

	"github.com/google/uuid"
)

// StatusEvent is a durable order status change addressable by its SSE event ID.
type StatusEvent struct {
	ID    int64
	Order domain.Order
}

// StatusEvents returns status changes after afterID for an order owned by userID.
func (s *Service) StatusEvents(ctx context.Context, orderID, userID uuid.UUID, afterID int64) ([]StatusEvent, error) {
	rows, err := s.db.Query(ctx, `SELECT h.id, h.to_status, h.order_version, h.reason, h.reason_code
		FROM order_status_history h
		JOIN orders o ON o.id = h.order_id
		WHERE h.order_id = $1 AND o.user_id = $2 AND h.id > $3
		ORDER BY h.id`, orderID, userID, afterID)
	if err != nil {
		return nil, fmt.Errorf("list order status events: %w", err)
	}
	defer rows.Close()
	type statusRecord struct {
		id         int64
		status     domain.OrderStatus
		version    int64
		reason     *string
		reasonCode *domain.OrderReasonCode
	}
	records := make([]statusRecord, 0)
	for rows.Next() {
		var record statusRecord
		if err := rows.Scan(&record.id, &record.status, &record.version, &record.reason, &record.reasonCode); err != nil {
			return nil, fmt.Errorf("scan order status event: %w", err)
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate order status events: %w", err)
	}
	rows.Close()
	if len(records) == 0 {
		return nil, nil
	}
	current, err := getOrder(ctx, s.db, orderID, userID, uuid.Nil)
	if err != nil {
		return nil, err
	}
	events := make([]StatusEvent, 0, len(records))
	for _, record := range records {
		order := current
		order.Status = record.status
		order.Version = record.version
		order.RejectionReason = record.reason
		order.StatusReasonCode = record.reasonCode
		events = append(events, StatusEvent{ID: record.id, Order: order})
	}
	return events, nil
}

func (s *Service) LatestStatusEventID(ctx context.Context, orderID, userID uuid.UUID) (int64, error) {
	var id int64
	err := s.db.QueryRow(ctx, `SELECT COALESCE(max(h.id), 0)
		FROM order_status_history h JOIN orders o ON o.id = h.order_id
		WHERE h.order_id = $1 AND o.user_id = $2`, orderID, userID).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("get latest order status event: %w", err)
	}
	return id, nil
}

func (s *Service) LatestRestaurantStatusEventID(ctx context.Context, restaurantID uuid.UUID) (int64, error) {
	var id int64
	err := s.db.QueryRow(ctx, `SELECT COALESCE(max(h.id), 0)
		FROM order_status_history h JOIN orders o ON o.id = h.order_id
		WHERE o.restaurant_id = $1`, restaurantID).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("get latest restaurant status event: %w", err)
	}
	return id, nil
}
