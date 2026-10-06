package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"

	"restaurant-delivery-system/internal/domain"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Queries struct {
	db *pgxpool.Pool
}

func NewQueries(db *pgxpool.Pool) *Queries {
	return &Queries{db: db}
}

func (q *Queries) ListRestaurants(ctx context.Context) ([]domain.Restaurant, error) {
	rows, err := q.db.Query(ctx, `
		SELECT id, name, description, is_open, cuisine, eta, rating::text, art
		FROM restaurants WHERE is_active = true ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("list restaurants: %w", err)
	}
	defer rows.Close()
	items := make([]domain.Restaurant, 0)
	for rows.Next() {
		var item domain.Restaurant
		if err := rows.Scan(&item.ID, &item.Name, &item.Description, &item.IsOpen, &item.Cuisine, &item.ETA, &item.Rating, &item.Art); err != nil {
			return nil, fmt.Errorf("scan restaurant: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (q *Queries) GetMenu(ctx context.Context, restaurantID uuid.UUID) (domain.Menu, error) {
	var menu domain.Menu
	err := q.db.QueryRow(ctx, `
		SELECT id, name, description, is_open, cuisine, eta, rating::text, art
		FROM restaurants WHERE id = $1 AND is_active = true`, restaurantID).
		Scan(&menu.Restaurant.ID, &menu.Restaurant.Name, &menu.Restaurant.Description, &menu.Restaurant.IsOpen,
			&menu.Restaurant.Cuisine, &menu.Restaurant.ETA, &menu.Restaurant.Rating, &menu.Restaurant.Art)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Menu{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Menu{}, fmt.Errorf("get restaurant: %w", err)
	}

	rows, err := q.db.Query(ctx, `
		SELECT c.id, c.name, p.id, p.name, p.description, p.price_minor, p.currency,
		       (p.is_active AND i.is_available AND i.quantity > 0), i.quantity
		FROM menu_categories c
		LEFT JOIN products p ON p.category_id = c.id AND p.is_active = true
		LEFT JOIN product_inventory i ON i.product_id = p.id
		WHERE c.restaurant_id = $1
		  AND EXISTS (SELECT 1 FROM products active_product
		              WHERE active_product.category_id = c.id AND active_product.is_active = true)
		ORDER BY c.position, c.name, p.name`, restaurantID)
	if err != nil {
		return domain.Menu{}, fmt.Errorf("query menu: %w", err)
	}
	defer rows.Close()

	menu.Categories = make([]domain.MenuCategory, 0)
	categoryIndex := make(map[uuid.UUID]int)
	for rows.Next() {
		var category domain.MenuCategory
		var productID *uuid.UUID
		var productName, description, currency *string
		var price *int64
		var available *bool
		var quantity *int
		if err := rows.Scan(&category.ID, &category.Name, &productID, &productName, &description, &price, &currency, &available, &quantity); err != nil {
			return domain.Menu{}, fmt.Errorf("scan menu: %w", err)
		}
		index, exists := categoryIndex[category.ID]
		if !exists {
			category.Products = make([]domain.Product, 0)
			menu.Categories = append(menu.Categories, category)
			index = len(menu.Categories) - 1
			categoryIndex[category.ID] = index
		}
		if productID != nil {
			menu.Categories[index].Products = append(menu.Categories[index].Products, domain.Product{
				ID: *productID, Name: *productName, Description: *description,
				PriceMinor: *price, Currency: *currency, Available: *available, Quantity: *quantity,
			})
		}
	}
	if err := rows.Err(); err != nil {
		return domain.Menu{}, fmt.Errorf("iterate menu: %w", err)
	}
	return menu, nil
}

func (q *Queries) AuthenticatePartner(ctx context.Context, apiKey string) (uuid.UUID, error) {
	hash := sha256.Sum256([]byte(apiKey))
	var restaurantID uuid.UUID
	err := q.db.QueryRow(ctx, `
		SELECT restaurant_id FROM restaurant_integrations WHERE api_key_hash = $1`,
		hex.EncodeToString(hash[:])).Scan(&restaurantID)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, domain.ErrUnauthorized
	}
	if err != nil {
		return uuid.Nil, fmt.Errorf("authenticate partner: %w", err)
	}
	return restaurantID, nil
}

func (q *Queries) ReplaceMenu(ctx context.Context, restaurantID uuid.UUID, menu domain.PartnerMenu) error {
	tx, err := q.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin menu transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err = tx.Exec(ctx, `UPDATE products SET is_active = false, updated_at = now()
		WHERE restaurant_id = $1`, restaurantID); err != nil {
		return fmt.Errorf("deactivate previous menu: %w", err)
	}

	for _, category := range menu.Categories {
		if category.ExternalID == "" || category.Name == "" {
			return fmt.Errorf("%w: category identifiers and names are required", domain.ErrValidation)
		}
		var categoryID uuid.UUID
		err = tx.QueryRow(ctx, `
			INSERT INTO menu_categories (restaurant_id, external_id, name, position)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (restaurant_id, external_id) DO UPDATE
			SET name = EXCLUDED.name, position = EXCLUDED.position, updated_at = now()
			RETURNING id`, restaurantID, category.ExternalID, category.Name, category.Position).Scan(&categoryID)
		if err != nil {
			return fmt.Errorf("upsert category: %w", err)
		}
		for _, product := range category.Products {
			if product.ExternalID == "" || product.Name == "" || product.PriceMinor < 0 || product.Quantity < 0 {
				return fmt.Errorf("%w: invalid product", domain.ErrValidation)
			}
			var productID uuid.UUID
			err = tx.QueryRow(ctx, `
				INSERT INTO products (restaurant_id, category_id, external_id, name, description, price_minor)
				VALUES ($1, $2, $3, $4, $5, $6)
				ON CONFLICT (restaurant_id, external_id) DO UPDATE
				SET category_id = EXCLUDED.category_id, name = EXCLUDED.name,
				    description = EXCLUDED.description, price_minor = EXCLUDED.price_minor,
				    is_active = true, updated_at = now()
				RETURNING id`, restaurantID, categoryID, product.ExternalID, product.Name,
				product.Description, product.PriceMinor).Scan(&productID)
			if err != nil {
				return fmt.Errorf("upsert product: %w", err)
			}
			_, err = tx.Exec(ctx, `
				INSERT INTO product_inventory (product_id, quantity, is_available)
				VALUES ($1, $2, $3)
				ON CONFLICT (product_id) DO UPDATE
				SET quantity = EXCLUDED.quantity, is_available = EXCLUDED.is_available, updated_at = now()`,
				productID, product.Quantity, product.Available)
			if err != nil {
				return fmt.Errorf("upsert inventory: %w", err)
			}
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit menu: %w", err)
	}
	return nil
}
