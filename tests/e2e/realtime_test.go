//go:build e2e

package e2e

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
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
