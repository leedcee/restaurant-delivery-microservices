//go:build e2e

package e2e

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"restaurant-delivery-system/internal/domain"
)

func TestPartnerOrderRealtime(t *testing.T) {
	client := &http.Client{Timeout: 5 * time.Second}
	productID := publishSingleProduct(t, client, "realtime", 3)
	accessToken := registerAccessToken(t, client)

	streamContext, cancel := context.WithTimeout(t.Context(), 8*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(streamContext, http.MethodGet, baseURL+"/partner/v1/orders/events", nil)
	if err != nil {
		t.Fatalf("create partner stream request: %v", err)
	}
	request.Header.Set("X-API-Key", "demo-secret")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatalf("open partner stream: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("partner stream returned %d, want 200", response.StatusCode)
	}
	if contentType := response.Header.Get("Content-Type"); !strings.HasPrefix(contentType, "text/event-stream") {
		t.Fatalf("partner stream content type = %q", contentType)
	}

	reader := bufio.NewReader(response.Body)
	_ = readPartnerOrdersEvent(t, reader) // initial authoritative snapshot confirms the subscription is active

	draft := newDraft(productID, 1)
	status, body := createOrderRequest(t.Context(), client, draft, accessToken)
	if status != http.StatusCreated {
		t.Fatalf("create realtime order returned %d: %s", status, body)
	}
	var created domain.Order
	if err := json.Unmarshal([]byte(body), &created); err != nil {
		t.Fatalf("decode realtime order: %v", err)
	}

	for {
		orders := readPartnerOrdersEvent(t, reader)
		for _, order := range orders {
			if order.ID == created.ID {
				return
			}
		}
	}
}

func TestCustomerOrderRealtimeReplaysMissedStatuses(t *testing.T) {
	client := &http.Client{Timeout: 5 * time.Second}
	db := openTestDB(t)
	defer db.Close()
	setRestaurantEndpoint(t, db, demoRestaurantID, brokenEndpoint)
	defer restoreOrderEndpoint(t, db)

	productID := publishSingleProduct(t, client, "replay", 3)
	accessToken := registerAccessToken(t, client)
	status, body := createOrderRequest(t.Context(), client, newDraft(productID, 1), accessToken)
	if status != http.StatusCreated {
		t.Fatalf("create replay order returned %d: %s", status, body)
	}
	var created domain.Order
	if err := json.Unmarshal([]byte(body), &created); err != nil {
		t.Fatalf("decode replay order: %v", err)
	}
	var pendingEventID int64
	if err := db.QueryRow(t.Context(), `SELECT max(id) FROM order_status_history WHERE order_id = $1`, created.ID).Scan(&pendingEventID); err != nil {
		t.Fatalf("load pending event id: %v", err)
	}

	var accepted domain.Order
	doJSON(t, client, http.MethodPatch, baseURL+"/partner/v1/orders/"+created.ID.String()+"/status",
		map[string]any{"status": domain.OrderAccepted, "expectedVersion": created.Version},
		map[string]string{"X-API-Key": "demo-secret"}, http.StatusOK, &accepted)
	var preparing domain.Order
	doJSON(t, client, http.MethodPatch, baseURL+"/partner/v1/orders/"+created.ID.String()+"/status",
		map[string]any{"status": domain.OrderPreparing, "expectedVersion": accepted.Version},
		map[string]string{"X-API-Key": "demo-secret"}, http.StatusOK, &preparing)

	streamContext, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(streamContext, http.MethodGet, baseURL+"/api/v1/orders/"+created.ID.String()+"/events", nil)
	if err != nil {
		t.Fatalf("create customer stream request: %v", err)
	}
	request.Header.Set("Authorization", "Bearer "+accessToken)
	request.Header.Set("Last-Event-ID", strconv.FormatInt(pendingEventID, 10))
	response, err := client.Do(request)
	if err != nil {
		t.Fatalf("open customer stream: %v", err)
	}
	defer response.Body.Close()
	reader := bufio.NewReader(response.Body)
	acceptedID, acceptedReplay := readOrderEvent(t, reader)
	preparingID, preparingReplay := readOrderEvent(t, reader)
	if acceptedReplay.Status != domain.OrderAccepted || preparingReplay.Status != domain.OrderPreparing {
		t.Fatalf("unexpected replay sequence: %s -> %s", acceptedReplay.Status, preparingReplay.Status)
	}
	if acceptedID <= pendingEventID || preparingID <= acceptedID {
		t.Fatalf("SSE event IDs are not increasing: pending=%d accepted=%d preparing=%d", pendingEventID, acceptedID, preparingID)
	}
}

func readOrderEvent(t *testing.T, reader *bufio.Reader) (int64, domain.Order) {
	t.Helper()
	var eventID int64
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read customer SSE event: %v", err)
		}
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "id: ") {
			eventID, err = strconv.ParseInt(strings.TrimPrefix(line, "id: "), 10, 64)
			if err != nil {
				t.Fatalf("parse SSE event id: %v", err)
			}
		}
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		var order domain.Order
		if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &order); err != nil {
			t.Fatalf("decode customer SSE event: %v", err)
		}
		return eventID, order
	}
}

func readPartnerOrdersEvent(t *testing.T, reader *bufio.Reader) []domain.Order {
	t.Helper()
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read partner SSE event: %v", err)
		}
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		var payload struct {
			Items []domain.Order `json:"items"`
		}
		if err := json.Unmarshal([]byte(strings.TrimSpace(strings.TrimPrefix(line, "data: "))), &payload); err != nil {
			t.Fatalf("decode partner SSE event: %v", err)
		}
		return payload.Items
	}
}
