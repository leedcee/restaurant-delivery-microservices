//go:build e2e

package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"sync"
	"testing"
	"time"

	"restaurant-delivery-system/internal/domain"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	demoRestaurantID = "11111111-1111-1111-1111-111111111111"
	demoEndpoint     = "http://demo-restaurant:8081/integration/v1/orders"
	brokenEndpoint   = "http://demo-restaurant:1/integration/v1/orders"
)

func TestNegativeAndReliabilityScenarios(t *testing.T) {
	client := &http.Client{Timeout: 5 * time.Second}
	db := openTestDB(t)

	t.Run("invalid partner API key", func(t *testing.T) {
		status, body := requestJSON(t.Context(), client, http.MethodGet, baseURL+"/partner/v1/orders", nil,
			map[string]string{"X-API-Key": "definitely-wrong"})
		if status != http.StatusUnauthorized {
			t.Fatalf("got %d, want 401: %s", status, body)
		}
	})

	t.Run("closed restaurant rejects quote", func(t *testing.T) {
		productID := publishSingleProduct(t, client, "closed", 2)
		setRestaurantOpen(t, db, false)
		t.Cleanup(func() { setRestaurantOpen(t, db, true) })

		draft := newDraft(productID, 1)
		status, body := requestJSON(t.Context(), client, http.MethodPost, baseURL+"/api/v1/orders/quote", draft,
			map[string]string{"X-User-ID": uuid.NewString()})
		if status != http.StatusConflict {
			t.Fatalf("got %d, want 409: %s", status, body)
		}
	})

	t.Run("unavailable product rejects order", func(t *testing.T) {
		productID := publishSingleProduct(t, client, "out-of-stock", 1)
		draft := newDraft(productID, 2)
		status, body := createOrderRequest(t.Context(), client, draft)
		if status != http.StatusConflict {
			t.Fatalf("got %d, want 409: %s", status, body)
		}
	})

	t.Run("forbidden status transition", func(t *testing.T) {
		productID := publishSingleProduct(t, client, "transition", 1)
		setOrderEndpoint(t, db, brokenEndpoint)
		t.Cleanup(func() { restoreOrderEndpoint(t, db) })
		order := createOrder(t, client, newDraft(productID, 1))

		status, body := requestJSON(t.Context(), client, http.MethodPatch,
			baseURL+"/partner/v1/orders/"+order.ID.String()+"/status",
			map[string]string{"status": string(domain.OrderDelivered)},
			map[string]string{"X-API-Key": "demo-secret"})
		if status != http.StatusConflict {
			t.Fatalf("got %d, want 409: %s", status, body)
		}
	})

	t.Run("outbox retries after restaurant recovery", func(t *testing.T) {
		productID := publishSingleProduct(t, client, "retry", 1)
		setOrderEndpoint(t, db, brokenEndpoint)
		t.Cleanup(func() { restoreOrderEndpoint(t, db) })
		order := createOrder(t, client, newDraft(productID, 1))

		waitFor(t, 8*time.Second, func() bool {
			var attempts int
			var hasError bool
			err := db.QueryRow(t.Context(), `SELECT attempts, last_error IS NOT NULL
				FROM outbox_events WHERE aggregate_id = $1`, order.ID).Scan(&attempts, &hasError)
			return err == nil && attempts >= 1 && hasError
		}, "outbox did not record failed delivery")

		restoreOrderEndpoint(t, db)
		if _, err := db.Exec(t.Context(), `UPDATE outbox_events SET available_at = now()
			WHERE aggregate_id = $1 AND processed_at IS NULL`, order.ID); err != nil {
			t.Fatalf("expedite outbox retry: %v", err)
		}
		waitFor(t, 10*time.Second, func() bool {
			var processed bool
			var status domain.OrderStatus
			err := db.QueryRow(t.Context(), `SELECT e.processed_at IS NOT NULL, o.status
				FROM outbox_events e JOIN orders o ON o.id = e.aggregate_id
				WHERE e.aggregate_id = $1`, order.ID).Scan(&processed, &status)
			return err == nil && processed && status == domain.OrderDelivered
		}, "outbox did not deliver and complete order after restaurant recovery")
	})

	t.Run("concurrent orders cannot oversell last item", func(t *testing.T) {
		productID := publishSingleProduct(t, client, "concurrency", 1)
		draft := newDraft(productID, 1)
		statuses := make([]int, 2)
		bodies := make([]string, 2)
		var wg sync.WaitGroup
		for i := range statuses {
			wg.Add(1)
			go func(index int) {
				defer wg.Done()
				statuses[index], bodies[index] = createOrderRequest(t.Context(), client, draft)
			}(i)
		}
		wg.Wait()
		sort.Ints(statuses)
		if statuses[0] != http.StatusCreated || statuses[1] != http.StatusConflict {
			t.Fatalf("got statuses %v, want [201 409]; bodies: %q", statuses, bodies)
		}
	})
}

func openTestDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	databaseURL := os.Getenv("E2E_DATABASE_URL")
	if databaseURL == "" {
		databaseURL = "postgres://restaurant:restaurant@localhost:5432/restaurant_delivery?sslmode=disable"
	}
	db, err := pgxpool.New(t.Context(), databaseURL)
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	if err := db.Ping(t.Context()); err != nil {
		db.Close()
		t.Fatalf("ping test database: %v", err)
	}
	t.Cleanup(db.Close)
	return db
}

func publishSingleProduct(t *testing.T, client *http.Client, suffix string, quantity int) uuid.UUID {
	t.Helper()
	externalID := suffix + "-" + uuid.NewString()
	payload := domain.PartnerMenu{Categories: []domain.PartnerCategory{{
		ExternalID: externalID, Name: "E2E " + suffix, Position: 1,
		Products: []domain.PartnerProduct{{
			ExternalID: externalID, Name: externalID, PriceMinor: 10000,
			Quantity: quantity, Available: quantity > 0,
		}},
	}}}
	doJSON(t, client, http.MethodPut, baseURL+"/partner/v1/menu", payload,
		map[string]string{"X-API-Key": "demo-secret"}, http.StatusNoContent, nil)

	var menu domain.Menu
	doJSON(t, client, http.MethodGet, baseURL+"/api/v1/restaurants/"+demoRestaurantID+"/menu",
		nil, nil, http.StatusOK, &menu)
	for _, category := range menu.Categories {
		for _, product := range category.Products {
			if product.Name == externalID {
				return product.ID
			}
		}
	}
	t.Fatal("published product was not returned by menu")
	return uuid.Nil
}

func newDraft(productID uuid.UUID, quantity int) domain.OrderDraft {
	return domain.OrderDraft{
		RestaurantID: uuid.MustParse(demoRestaurantID), DeliveryAddress: "Самара, E2E",
		Items: []domain.DraftItem{{ProductID: productID, Quantity: quantity}},
	}
}

func createOrder(t *testing.T, client *http.Client, draft domain.OrderDraft) domain.Order {
	t.Helper()
	status, body := createOrderRequest(t.Context(), client, draft)
	if status != http.StatusCreated {
		t.Fatalf("create order returned %d, want 201: %s", status, body)
	}
	var order domain.Order
	if err := json.Unmarshal([]byte(body), &order); err != nil {
		t.Fatalf("decode created order: %v", err)
	}
	return order
}

func createOrderRequest(ctx context.Context, client *http.Client, draft domain.OrderDraft) (int, string) {
	return requestJSON(ctx, client, http.MethodPost, baseURL+"/api/v1/orders", draft,
		map[string]string{"X-User-ID": uuid.NewString(), "Idempotency-Key": "e2e-" + uuid.NewString()})
}

func requestJSON(ctx context.Context, client *http.Client, method, url string, payload any, headers map[string]string) (int, string) {
	var body io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return 0, fmt.Sprintf("marshal request: %v", err)
		}
		body = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return 0, fmt.Sprintf("create request: %v", err)
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	response, err := client.Do(req)
	if err != nil {
		return 0, fmt.Sprintf("send request: %v", err)
	}
	data, readErr := io.ReadAll(response.Body)
	closeErr := response.Body.Close()
	if readErr != nil {
		return 0, fmt.Sprintf("read response: %v", readErr)
	}
	if closeErr != nil {
		return 0, fmt.Sprintf("close response: %v", closeErr)
	}
	return response.StatusCode, string(data)
}

func setRestaurantOpen(t *testing.T, db *pgxpool.Pool, open bool) {
	t.Helper()
	if _, err := db.Exec(context.Background(), `UPDATE restaurants SET is_open = $1 WHERE id = $2`,
		open, demoRestaurantID); err != nil {
		t.Fatalf("set restaurant open=%v: %v", open, err)
	}
}

func setOrderEndpoint(t *testing.T, db *pgxpool.Pool, endpoint string) {
	t.Helper()
	if _, err := db.Exec(context.Background(), `UPDATE restaurant_integrations SET order_endpoint = $1
		WHERE restaurant_id = $2`, endpoint, demoRestaurantID); err != nil {
		t.Fatalf("set restaurant endpoint: %v", err)
	}
}

func restoreOrderEndpoint(t *testing.T, db *pgxpool.Pool) {
	t.Helper()
	setOrderEndpoint(t, db, demoEndpoint)
}

func waitFor(t *testing.T, timeout time.Duration, condition func() bool, failure string) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatal(failure)
}
