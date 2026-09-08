//go:build e2e

package e2e

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"

	"avito-kitchen/internal/domain"

	"github.com/google/uuid"
)

const baseURL = "http://localhost:8080"

func TestOrderJourney(t *testing.T) {
	client := &http.Client{Timeout: 5 * time.Second}
	menuPayload := domain.PartnerMenu{Categories: []domain.PartnerCategory{{
		ExternalID: "e2e", Name: "E2E", Position: 99,
		Products: []domain.PartnerProduct{{
			ExternalID: "e2e-product", Name: "E2E product", PriceMinor: 12300,
			Quantity: 10, Available: true,
		}},
	}}}
	doJSON(t, client, http.MethodPut, baseURL+"/partner/v1/menu", menuPayload,
		map[string]string{"X-API-Key": "demo-secret"}, http.StatusNoContent, nil)

	var menu domain.Menu
	doJSON(t, client, http.MethodGet,
		baseURL+"/api/v1/restaurants/11111111-1111-1111-1111-111111111111/menu",
		nil, nil, http.StatusOK, &menu)
	var productID uuid.UUID
	for _, category := range menu.Categories {
		for _, product := range category.Products {
			if product.Name == "E2E product" {
				productID = product.ID
			}
		}
	}
	if productID == uuid.Nil {
		t.Fatal("published product was not returned by menu")
	}

	userID := uuid.New().String()
	key := "e2e-" + uuid.NewString()
	draft := domain.OrderDraft{
		RestaurantID:    uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		DeliveryAddress: "Самара, E2E",
		Items:           []domain.DraftItem{{ProductID: productID, Quantity: 2}},
	}
	headers := map[string]string{"X-User-ID": userID, "Idempotency-Key": key}
	var created domain.Order
	doJSON(t, client, http.MethodPost, baseURL+"/api/v1/orders", draft, headers, http.StatusCreated, &created)
	var repeated domain.Order
	doJSON(t, client, http.MethodPost, baseURL+"/api/v1/orders", draft, headers, http.StatusOK, &repeated)
	if created.ID != repeated.ID {
		t.Fatalf("idempotency failed: %s != %s", created.ID, repeated.ID)
	}
	changedDraft := draft
	changedDraft.Items = []domain.DraftItem{{ProductID: productID, Quantity: 1}}
	doJSON(t, client, http.MethodPost, baseURL+"/api/v1/orders", changedDraft, headers, http.StatusConflict, nil)

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		var current domain.Order
		doJSON(t, client, http.MethodGet, baseURL+"/api/v1/orders/"+created.ID.String(), nil,
			map[string]string{"X-User-ID": userID}, http.StatusOK, &current)
		if current.Status == domain.OrderDelivered {
			return
		}
		time.Sleep(250 * time.Millisecond)
	}
	t.Fatal("restaurant did not complete the order before timeout")
}

func doJSON(
	t *testing.T,
	client *http.Client,
	method, url string,
	payload any,
	headers map[string]string,
	wantStatus int,
	result any,
) {
	t.Helper()
	var body io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("marshal request: %v", err)
		}
		body = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(t.Context(), method, url, body)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	response, err := client.Do(req)
	if err != nil {
		t.Fatalf("request %s %s: %v", method, url, err)
	}
	data, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	if err := response.Body.Close(); err != nil {
		t.Fatalf("close response: %v", err)
	}
	if response.StatusCode != wantStatus {
		t.Fatalf("%s %s returned %d, want %d: %s", method, url, response.StatusCode, wantStatus, data)
	}
	if result != nil && len(data) > 0 {
		if err := json.Unmarshal(data, result); err != nil {
			t.Fatalf("decode response %q: %v", data, err)
		}
	}
}
