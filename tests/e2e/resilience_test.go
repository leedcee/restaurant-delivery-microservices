//go:build e2e

package e2e

import (
	"net/http"
	"os"
	"testing"
	"time"

	"restaurant-delivery-system/internal/domain"

	"github.com/google/uuid"
)

const restartProbeAddress = "Самара, resilience restart probe"

func TestPrepareOutboxBeforeAPIRestart(t *testing.T) {
	requireResiliencePhase(t, "prepare-restart")
	client := &http.Client{Timeout: 5 * time.Second}
	db := openTestDB(t)
	productID := publishSingleProduct(t, client, "restart-resilience", 1)
	draft := newDraft(productID, 1)
	draft.DeliveryAddress = restartProbeAddress
	order := createOrder(t, client, draft)

	waitFor(t, 10*time.Second, func() bool {
		var attempts int
		var processed bool
		var deadLettered bool
		err := db.QueryRow(t.Context(), `SELECT attempts, processed_at IS NOT NULL, dead_lettered_at IS NOT NULL
			FROM outbox_events WHERE aggregate_id = $1`, order.ID).Scan(&attempts, &processed, &deadLettered)
		return err == nil && attempts >= 1 && !processed && !deadLettered
	}, "outbox did not retain the failed delivery before API restart")
}

func TestVerifyOutboxAfterAPIRestart(t *testing.T) {
	requireResiliencePhase(t, "verify-restart")
	db := openTestDB(t)

	var orderID uuid.UUID
	var eventID uuid.UUID
	var attempts int
	if err := db.QueryRow(t.Context(), `SELECT o.id, e.id, e.attempts
		FROM orders o JOIN outbox_events e ON e.aggregate_id = o.id
		WHERE o.delivery_address = $1 ORDER BY o.created_at DESC LIMIT 1`, restartProbeAddress).
		Scan(&orderID, &eventID, &attempts); err != nil {
		t.Fatalf("load restart probe: %v", err)
	}
	if attempts < 1 {
		t.Fatalf("outbox attempts were not persisted across restart: %d", attempts)
	}
	var eventCount int
	if err := db.QueryRow(t.Context(), `SELECT count(*) FROM outbox_events WHERE aggregate_id = $1`, orderID).Scan(&eventCount); err != nil {
		t.Fatalf("count restart probe events: %v", err)
	}
	if eventCount != 1 {
		t.Fatalf("API restart duplicated the logical event: got %d outbox rows", eventCount)
	}

	waitFor(t, 20*time.Second, func() bool {
		var processed bool
		var status domain.OrderStatus
		err := db.QueryRow(t.Context(), `SELECT e.processed_at IS NOT NULL, o.status
			FROM outbox_events e JOIN orders o ON o.id = e.aggregate_id WHERE e.id = $1`, eventID).
			Scan(&processed, &status)
		return err == nil && processed && status == domain.OrderDelivered
	}, "pending outbox event was not delivered after API and restaurant recovery")
}

func TestVerifyDatabaseRecovery(t *testing.T) {
	requireResiliencePhase(t, "verify-database")
	client := &http.Client{Timeout: 5 * time.Second}
	var restaurants struct {
		Items []domain.Restaurant `json:"items"`
	}
	doJSON(t, client, http.MethodGet, baseURL+"/api/v1/restaurants", nil, nil, http.StatusOK, &restaurants)
	if len(restaurants.Items) == 0 {
		t.Fatal("catalog is empty after PostgreSQL recovery")
	}
	db := openTestDB(t)
	if err := db.Ping(t.Context()); err != nil {
		t.Fatalf("database pool did not recover: %v", err)
	}
}

func requireResiliencePhase(t *testing.T, phase string) {
	t.Helper()
	if os.Getenv("E2E_RESILIENCE_PHASE") != phase {
		t.Skip("orchestrated by scripts/e2e-resilience.sh")
	}
}
