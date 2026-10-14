// Command demo-restaurant runs the example partner restaurant service.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"restaurant-delivery-system/internal/config"
	"restaurant-delivery-system/internal/domain"
)

type restaurant struct {
	config config.DemoRestaurant
	logger *slog.Logger
	client *http.Client
	mu     sync.RWMutex
	orders map[string]domain.Order
	events map[string]struct{}
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg := config.DemoRestaurantFromEnv()
	app := &restaurant{
		config: cfg,
		logger: logger,
		client: &http.Client{Timeout: 5 * time.Second},
		orders: make(map[string]domain.Order),
		events: make(map[string]struct{}),
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health/live", app.health)
	mux.HandleFunc("POST /integration/v1/orders", app.receiveOrder)
	server := &http.Server{
		Addr: cfg.HTTPAddr, Handler: mux, ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 1 << 20,
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	go app.publishMenuUntilReady(ctx)
	go func() {
		logger.Info("demo restaurant started", "address", cfg.HTTPAddr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("HTTP server failed", "error", err)
			stop()
		}
	}()
	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = server.Shutdown(shutdownCtx)
}

func (a *restaurant) health(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (a *restaurant) receiveOrder(w http.ResponseWriter, r *http.Request) {
	eventID := strings.TrimSpace(r.Header.Get("X-Event-ID"))
	eventType := strings.TrimSpace(r.Header.Get("X-Event-Type"))
	eventVersion, versionErr := strconv.Atoi(r.Header.Get("X-Event-Version"))
	if eventID == "" || r.Header.Get("Idempotency-Key") != eventID || eventType != "order.created" || versionErr != nil || eventVersion != 1 {
		http.Error(w, "invalid event metadata", http.StatusBadRequest)
		return
	}
	var order domain.Order
	if err := json.NewDecoder(r.Body).Decode(&order); err != nil {
		http.Error(w, "invalid order", http.StatusBadRequest)
		return
	}
	a.mu.Lock()
	_, duplicateEvent := a.events[eventID]
	if duplicateEvent {
		a.mu.Unlock()
		w.WriteHeader(http.StatusAccepted)
		return
	}
	a.events[eventID] = struct{}{}
	_, alreadyReceived := a.orders[order.ID.String()]
	a.orders[order.ID.String()] = order
	a.mu.Unlock()
	a.logger.Info("order received", "orderId", order.ID)
	w.WriteHeader(http.StatusAccepted)
	if !alreadyReceived && order.Status != domain.OrderRejected && order.Status != domain.OrderCancelled && order.Status != domain.OrderDelivered {
		go a.progressOrder(order.ID.String(), order.RestaurantID.String())
	}
}

func (a *restaurant) progressOrder(orderID, restaurantID string) {
	statuses := []domain.OrderStatus{
		domain.OrderAccepted,
		domain.OrderPreparing,
		domain.OrderReady,
		domain.OrderDelivering,
		domain.OrderDelivered,
	}
	for _, status := range statuses {
		time.Sleep(300 * time.Millisecond)
		a.mu.RLock()
		current := a.orders[orderID]
		a.mu.RUnlock()
		updated, err := a.sendStatus(orderID, restaurantID, current.Version, status)
		if err != nil {
			a.logger.Error("order status callback failed", "error", err, "orderId", orderID, "status", status)
			return
		}
		a.mu.Lock()
		a.orders[orderID] = updated
		a.mu.Unlock()
		a.logger.Info("order status updated", "orderId", orderID, "status", status)
	}
}

func (a *restaurant) sendStatus(orderID, restaurantID string, expectedVersion int64, status domain.OrderStatus) (domain.Order, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	body, err := json.Marshal(map[string]any{"status": status, "expectedVersion": expectedVersion})
	if err != nil {
		return domain.Order{}, fmt.Errorf("marshal callback: %w", err)
	}
	url := fmt.Sprintf("%s/partner/v1/orders/%s/status", a.config.PlatformBaseURL, orderID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, url, bytes.NewReader(body))
	if err != nil {
		return domain.Order{}, fmt.Errorf("create callback: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	apiKey := a.config.APIKey
	if partnerKey := a.config.PartnerAPIKeys[restaurantID]; partnerKey != "" {
		apiKey = partnerKey
	}
	req.Header.Set("X-API-Key", apiKey)
	response, err := a.client.Do(req)
	if err != nil {
		return domain.Order{}, fmt.Errorf("send callback: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return domain.Order{}, fmt.Errorf("callback returned status %d", response.StatusCode)
	}
	var updated domain.Order
	if err := json.NewDecoder(response.Body).Decode(&updated); err != nil {
		return domain.Order{}, fmt.Errorf("decode callback response: %w", err)
	}
	return updated, nil
}

func (a *restaurant) publishMenuUntilReady(ctx context.Context) {
	menu := domain.PartnerMenu{Categories: []domain.PartnerCategory{
		{ExternalID: "popular", Name: "Популярное", Position: 1, Products: []domain.PartnerProduct{
			{ExternalID: "chicken-pasta", Name: "Паста с курицей", Description: "Свежие продукты и фирменный соус", PriceMinor: 49000, Quantity: 20, Available: true},
			{ExternalID: "braised-beef", Name: "Томлёная говядина", Description: "Свежие продукты и фирменный соус", PriceMinor: 62000, Quantity: 15, Available: true},
			{ExternalID: "syrniki", Name: "Сырники", Description: "Свежие продукты и фирменный соус", PriceMinor: 35000, Quantity: 30, Available: true},
		}},
		{ExternalID: "drinks", Name: "Напитки", Position: 2, Products: []domain.PartnerProduct{
			{ExternalID: "mors", Name: "Домашний морс", Description: "Ягодный морс, 0.5 л", PriceMinor: 19000, Quantity: 30, Available: true},
		}},
	}}
	payload, err := json.Marshal(menu)
	if err != nil {
		return
	}
	for {
		if err := a.publishMenu(ctx, payload); err == nil {
			a.logger.Info("demo menu published")
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(2 * time.Second):
		}
	}
}

func (a *restaurant) publishMenu(ctx context.Context, payload []byte) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, a.config.PlatformBaseURL+"/partner/v1/menu", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", a.config.APIKey)
	response, err := a.client.Do(req)
	if err != nil {
		return err
	}
	if closeErr := response.Body.Close(); closeErr != nil {
		return closeErr
	}
	if response.StatusCode != http.StatusNoContent {
		return fmt.Errorf("menu endpoint returned %d", response.StatusCode)
	}
	return nil
}
