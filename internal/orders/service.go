// Package orders implements quoting, order creation and status transitions.
package orders

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"avito-kitchen/internal/domain"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct {
	db *pgxpool.Pool
}

func New(db *pgxpool.Pool) *Service {
	return &Service{db: db}
}

func (s *Service) Quote(ctx context.Context, draft domain.OrderDraft) (domain.OrderQuote, error) {
	if err := validateDraft(draft, false); err != nil {
		return domain.OrderQuote{}, err
	}
	var active, open bool
	err := s.db.QueryRow(ctx, `SELECT is_active, is_open FROM restaurants WHERE id = $1`, draft.RestaurantID).
		Scan(&active, &open)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.OrderQuote{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.OrderQuote{}, fmt.Errorf("load restaurant: %w", err)
	}
	if !active || !open {
		return domain.OrderQuote{}, fmt.Errorf("%w: restaurant is closed", domain.ErrConflict)
	}
	return quoteItems(ctx, s.db, draft)
}

func (s *Service) Create(
	ctx context.Context,
	userID uuid.UUID,
	idempotencyKey string,
	draft domain.OrderDraft,
) (domain.Order, bool, error) {
	if userID == uuid.Nil || strings.TrimSpace(idempotencyKey) == "" || len(idempotencyKey) > 128 {
		return domain.Order{}, false, fmt.Errorf("%w: invalid user or idempotency key", domain.ErrValidation)
	}
	if err := validateDraft(draft, true); err != nil {
		return domain.Order{}, false, err
	}
	requestHash, err := fingerprint(draft)
	if err != nil {
		return domain.Order{}, false, err
	}

	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return domain.Order{}, false, fmt.Errorf("begin order transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	_, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, userID.String()+":"+idempotencyKey)
	if err != nil {
		return domain.Order{}, false, fmt.Errorf("lock idempotency key: %w", err)
	}

	existing, err := getOrderByIdempotency(ctx, tx, userID, idempotencyKey, requestHash)
	if err == nil {
		return existing, true, nil
	}
	if !errors.Is(err, domain.ErrNotFound) {
		return domain.Order{}, false, err
	}

	var active, open bool
	err = tx.QueryRow(ctx, `SELECT is_active, is_open FROM restaurants WHERE id = $1 FOR UPDATE`, draft.RestaurantID).
		Scan(&active, &open)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Order{}, false, domain.ErrNotFound
	}
	if err != nil {
		return domain.Order{}, false, fmt.Errorf("lock restaurant: %w", err)
	}
	if !active || !open {
		return domain.Order{}, false, fmt.Errorf("%w: restaurant is closed", domain.ErrConflict)
	}

	items := append([]domain.DraftItem(nil), draft.Items...)
	sort.Slice(items, func(i, j int) bool { return items[i].ProductID.String() < items[j].ProductID.String() })
	quote := domain.OrderQuote{RestaurantID: draft.RestaurantID, Items: make([]domain.QuoteItem, 0, len(items)), Currency: "RUB"}
	seen := make(map[uuid.UUID]struct{}, len(items))
	for _, requested := range items {
		if _, duplicate := seen[requested.ProductID]; duplicate {
			return domain.Order{}, false, fmt.Errorf("%w: duplicate product", domain.ErrValidation)
		}
		seen[requested.ProductID] = struct{}{}

		var name, currency string
		var price int64
		var quantity int
		var productActive, available bool
		err = tx.QueryRow(ctx, `
			SELECT p.name, p.price_minor, p.currency, p.is_active, i.quantity, i.is_available
			FROM products p JOIN product_inventory i ON i.product_id = p.id
			WHERE p.id = $1 AND p.restaurant_id = $2
			FOR UPDATE OF i`, requested.ProductID, draft.RestaurantID).
			Scan(&name, &price, &currency, &productActive, &quantity, &available)
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Order{}, false, fmt.Errorf("%w: product %s does not exist", domain.ErrValidation, requested.ProductID)
		}
		if err != nil {
			return domain.Order{}, false, fmt.Errorf("lock product: %w", err)
		}
		if !productActive || !available || quantity < requested.Quantity {
			return domain.Order{}, false, fmt.Errorf("%w: product %s is unavailable", domain.ErrConflict, requested.ProductID)
		}
		lineTotal := price * int64(requested.Quantity)
		quote.Items = append(quote.Items, domain.QuoteItem{
			ProductID: requested.ProductID, Name: name, Quantity: requested.Quantity,
			UnitPriceMinor: price, TotalMinor: lineTotal,
		})
		quote.TotalMinor += lineTotal
		quote.Currency = currency
	}

	order := domain.Order{
		ID: uuid.New(), UserID: userID, RestaurantID: draft.RestaurantID, Items: quote.Items,
		TotalMinor: quote.TotalMinor, Currency: quote.Currency, Status: domain.OrderPending,
		DeliveryAddress: strings.TrimSpace(draft.DeliveryAddress),
	}
	err = tx.QueryRow(ctx, `
		INSERT INTO orders (id, user_id, restaurant_id, idempotency_key, request_hash, total_minor, currency, delivery_address)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8) RETURNING created_at`,
		order.ID, order.UserID, order.RestaurantID, idempotencyKey, requestHash, order.TotalMinor,
		order.Currency, order.DeliveryAddress).Scan(&order.CreatedAt)
	if err != nil {
		return domain.Order{}, false, fmt.Errorf("insert order: %w", err)
	}

	for _, item := range order.Items {
		_, err = tx.Exec(ctx, `
			INSERT INTO order_items (order_id, product_id, product_name, unit_price_minor, quantity, total_minor)
			VALUES ($1, $2, $3, $4, $5, $6)`, order.ID, item.ProductID, item.Name,
			item.UnitPriceMinor, item.Quantity, item.TotalMinor)
		if err != nil {
			return domain.Order{}, false, fmt.Errorf("insert order item: %w", err)
		}
		command, err := tx.Exec(ctx, `
			UPDATE product_inventory SET quantity = quantity - $1, updated_at = now()
			WHERE product_id = $2 AND quantity >= $1`, item.Quantity, item.ProductID)
		if err != nil || command.RowsAffected() != 1 {
			return domain.Order{}, false, fmt.Errorf("%w: inventory changed", domain.ErrConflict)
		}
	}

	_, err = tx.Exec(ctx, `INSERT INTO order_status_history (order_id, to_status, source)
		VALUES ($1, 'pending', 'user')`, order.ID)
	if err != nil {
		return domain.Order{}, false, fmt.Errorf("create order history: %w", err)
	}
	_, err = tx.Exec(ctx, `INSERT INTO outbox_events (aggregate_id, event_type, payload)
		VALUES ($1::uuid, 'order.created', jsonb_build_object('orderId', $1::text))`, order.ID)
	if err != nil {
		return domain.Order{}, false, fmt.Errorf("create outbox event: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Order{}, false, fmt.Errorf("commit order: %w", err)
	}
	return order, false, nil
}

func (s *Service) Get(ctx context.Context, orderID, userID uuid.UUID) (domain.Order, error) {
	return getOrder(ctx, s.db, orderID, userID, uuid.Nil)
}

func (s *Service) ListForRestaurant(ctx context.Context, restaurantID uuid.UUID) ([]domain.Order, error) {
	rows, err := s.db.Query(ctx, `SELECT id FROM orders WHERE restaurant_id = $1 ORDER BY created_at DESC LIMIT 100`, restaurantID)
	if err != nil {
		return nil, fmt.Errorf("list restaurant orders: %w", err)
	}
	ids := make([]uuid.UUID, 0)
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan restaurant order: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, fmt.Errorf("iterate restaurant orders: %w", err)
	}
	rows.Close()
	result := make([]domain.Order, 0, len(ids))
	for _, id := range ids {
		order, err := getOrder(ctx, s.db, id, uuid.Nil, restaurantID)
		if err != nil {
			return nil, err
		}
		result = append(result, order)
	}
	return result, nil
}

func (s *Service) UpdatePartnerStatus(
	ctx context.Context,
	orderID, restaurantID uuid.UUID,
	status domain.OrderStatus,
	reason string,
) (domain.Order, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return domain.Order{}, fmt.Errorf("begin status transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var current domain.OrderStatus
	err = tx.QueryRow(ctx, `SELECT status FROM orders WHERE id = $1 AND restaurant_id = $2 FOR UPDATE`,
		orderID, restaurantID).Scan(&current)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Order{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Order{}, fmt.Errorf("lock order: %w", err)
	}
	if current == status {
		if err := tx.Commit(ctx); err != nil {
			return domain.Order{}, fmt.Errorf("commit idempotent status: %w", err)
		}
		return getOrder(ctx, s.db, orderID, uuid.Nil, restaurantID)
	}
	if !domain.CanTransition(current, status) {
		return domain.Order{}, fmt.Errorf("%w: transition %s -> %s is forbidden", domain.ErrConflict, current, status)
	}
	var rejectionReason *string
	if status == domain.OrderRejected {
		trimmed := strings.TrimSpace(reason)
		if trimmed == "" {
			return domain.Order{}, fmt.Errorf("%w: rejection reason is required", domain.ErrValidation)
		}
		rejectionReason = &trimmed
	}
	_, err = tx.Exec(ctx, `UPDATE orders SET status = $1, rejection_reason = $2, updated_at = now()
		WHERE id = $3`, status, rejectionReason, orderID)
	if err != nil {
		return domain.Order{}, fmt.Errorf("update order status: %w", err)
	}
	_, err = tx.Exec(ctx, `INSERT INTO order_status_history
		(order_id, from_status, to_status, source, reason) VALUES ($1, $2, $3, 'restaurant', $4)`,
		orderID, current, status, rejectionReason)
	if err != nil {
		return domain.Order{}, fmt.Errorf("record order status: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Order{}, fmt.Errorf("commit order status: %w", err)
	}
	return getOrder(ctx, s.db, orderID, uuid.Nil, restaurantID)
}

type rowQuerier interface {
	QueryRow(context.Context, string, ...any) pgx.Row
	Query(context.Context, string, ...any) (pgx.Rows, error)
}

func quoteItems(ctx context.Context, db rowQuerier, draft domain.OrderDraft) (domain.OrderQuote, error) {
	quote := domain.OrderQuote{RestaurantID: draft.RestaurantID, Items: make([]domain.QuoteItem, 0, len(draft.Items)), Currency: "RUB"}
	seen := make(map[uuid.UUID]struct{}, len(draft.Items))
	for _, requested := range draft.Items {
		if _, duplicate := seen[requested.ProductID]; duplicate {
			return domain.OrderQuote{}, fmt.Errorf("%w: duplicate product", domain.ErrValidation)
		}
		seen[requested.ProductID] = struct{}{}
		var item domain.QuoteItem
		var currency string
		var available bool
		var stock int
		err := db.QueryRow(ctx, `
			SELECT p.id, p.name, p.price_minor, p.currency,
			       (p.is_active AND i.is_available), i.quantity
			FROM products p JOIN product_inventory i ON i.product_id = p.id
			WHERE p.id = $1 AND p.restaurant_id = $2`, requested.ProductID, draft.RestaurantID).
			Scan(&item.ProductID, &item.Name, &item.UnitPriceMinor, &currency, &available, &stock)
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.OrderQuote{}, fmt.Errorf("%w: product %s does not exist", domain.ErrValidation, requested.ProductID)
		}
		if err != nil {
			return domain.OrderQuote{}, fmt.Errorf("quote product: %w", err)
		}
		if !available || stock < requested.Quantity {
			return domain.OrderQuote{}, fmt.Errorf("%w: product %s is unavailable", domain.ErrConflict, requested.ProductID)
		}
		item.Quantity = requested.Quantity
		item.TotalMinor = item.UnitPriceMinor * int64(item.Quantity)
		quote.Items = append(quote.Items, item)
		quote.TotalMinor += item.TotalMinor
		quote.Currency = currency
	}
	return quote, nil
}

func validateDraft(draft domain.OrderDraft, requireAddress bool) error {
	if draft.RestaurantID == uuid.Nil || len(draft.Items) == 0 || len(draft.Items) > 100 {
		return fmt.Errorf("%w: restaurant and 1..100 items are required", domain.ErrValidation)
	}
	for _, item := range draft.Items {
		if item.ProductID == uuid.Nil || item.Quantity < 1 || item.Quantity > 100 {
			return fmt.Errorf("%w: invalid order item", domain.ErrValidation)
		}
	}
	if requireAddress && strings.TrimSpace(draft.DeliveryAddress) == "" {
		return fmt.Errorf("%w: delivery address is required", domain.ErrValidation)
	}
	return nil
}

func getOrderByIdempotency(
	ctx context.Context,
	db rowQuerier,
	userID uuid.UUID,
	key, expectedHash string,
) (domain.Order, error) {
	var id uuid.UUID
	var storedHash string
	err := db.QueryRow(ctx, `SELECT id, request_hash FROM orders WHERE user_id = $1 AND idempotency_key = $2`,
		userID, key).Scan(&id, &storedHash)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Order{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Order{}, fmt.Errorf("find idempotent order: %w", err)
	}
	if storedHash != expectedHash {
		return domain.Order{}, fmt.Errorf("%w: idempotency key was used with another request", domain.ErrConflict)
	}
	return getOrder(ctx, db, id, userID, uuid.Nil)
}

func fingerprint(draft domain.OrderDraft) (string, error) {
	normalized := draft
	normalized.DeliveryAddress = strings.TrimSpace(normalized.DeliveryAddress)
	normalized.Items = append([]domain.DraftItem(nil), draft.Items...)
	sort.Slice(normalized.Items, func(i, j int) bool {
		return normalized.Items[i].ProductID.String() < normalized.Items[j].ProductID.String()
	})
	data, err := json.Marshal(normalized)
	if err != nil {
		return "", fmt.Errorf("marshal idempotency fingerprint: %w", err)
	}
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:]), nil
}

func getOrder(ctx context.Context, db rowQuerier, orderID, userID, restaurantID uuid.UUID) (domain.Order, error) {
	query := `SELECT id, user_id, restaurant_id, total_minor, currency, status, delivery_address,
	                 rejection_reason, created_at FROM orders WHERE id = $1`
	args := []any{orderID}
	if userID != uuid.Nil {
		query += ` AND user_id = $2`
		args = append(args, userID)
	} else if restaurantID != uuid.Nil {
		query += ` AND restaurant_id = $2`
		args = append(args, restaurantID)
	}
	var order domain.Order
	err := db.QueryRow(ctx, query, args...).Scan(&order.ID, &order.UserID, &order.RestaurantID,
		&order.TotalMinor, &order.Currency, &order.Status, &order.DeliveryAddress,
		&order.RejectionReason, &order.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Order{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Order{}, fmt.Errorf("get order: %w", err)
	}
	rows, err := db.Query(ctx, `SELECT product_id, product_name, quantity, unit_price_minor, total_minor
		FROM order_items WHERE order_id = $1 ORDER BY product_name`, orderID)
	if err != nil {
		return domain.Order{}, fmt.Errorf("get order items: %w", err)
	}
	defer rows.Close()
	order.Items = make([]domain.QuoteItem, 0)
	for rows.Next() {
		var item domain.QuoteItem
		if err := rows.Scan(&item.ProductID, &item.Name, &item.Quantity, &item.UnitPriceMinor, &item.TotalMinor); err != nil {
			return domain.Order{}, fmt.Errorf("scan order item: %w", err)
		}
		order.Items = append(order.Items, item)
	}
	if err := rows.Err(); err != nil {
		return domain.Order{}, fmt.Errorf("iterate order items: %w", err)
	}
	return order, nil
}
