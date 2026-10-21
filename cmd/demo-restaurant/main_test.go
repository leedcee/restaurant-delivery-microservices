package main

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"restaurant-delivery-system/internal/domain"
	"restaurant-delivery-system/internal/eventcontract"

	"github.com/google/uuid"
)

func TestReceiveOrderContractAndDeduplication(t *testing.T) {
	app := &restaurant{logger: slog.New(slog.NewTextHandler(io.Discard, nil)), orders: map[string]domain.Order{}, events: map[string]struct{}{}}
	order := domain.Order{ID: uuid.New(), UserID: uuid.New(), RestaurantID: uuid.New(), Status: domain.OrderDelivered, Version: 1, CreatedAt: time.Now()}
	body, err := json.Marshal(order)
	if err != nil {
		t.Fatalf("marshal order: %v", err)
	}
	metadata := eventcontract.Metadata{ID: uuid.New(), Type: eventcontract.OrderCreated, Version: eventcontract.OrderCreatedVersion, CorrelationID: order.ID, OccurredAt: order.CreatedAt}

	for attempt := 1; attempt <= 2; attempt++ {
		request := httptest.NewRequest(http.MethodPost, "/integration/v1/orders", bytes.NewReader(body))
		metadata.Apply(request.Header)
		response := httptest.NewRecorder()
		app.receiveOrder(response, request)
		if response.Code != http.StatusAccepted {
			t.Fatalf("attempt %d: got status %d", attempt, response.Code)
		}
	}
	if len(app.events) != 1 || len(app.orders) != 1 {
		t.Fatalf("expected one deduplicated event and order, got %d and %d", len(app.events), len(app.orders))
	}
}

func TestReceiveOrderRejectsMissingOccurredAt(t *testing.T) {
	app := &restaurant{logger: slog.New(slog.NewTextHandler(io.Discard, nil)), orders: map[string]domain.Order{}, events: map[string]struct{}{}}
	request := httptest.NewRequest(http.MethodPost, "/integration/v1/orders", bytes.NewReader([]byte(`{}`)))
	request.Header.Set("X-Event-ID", uuid.NewString())
	response := httptest.NewRecorder()
	app.receiveOrder(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("got status %d", response.Code)
	}
}
