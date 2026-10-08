// Package httpapi exposes the application's HTTP transport.
package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"restaurant-delivery-system/internal/domain"
	"restaurant-delivery-system/internal/identity"
	"restaurant-delivery-system/internal/orders"
	"restaurant-delivery-system/internal/store"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type server struct {
	db                 *pgxpool.Pool
	logger             *slog.Logger
	queries            *store.Queries
	orders             *orders.Service
	identity           *identity.Service
	partnerOrderEvents *partnerOrderHub
}

// New creates the HTTP handler for the main API.
func New(db *pgxpool.Pool, logger *slog.Logger, jwtSecret string) http.Handler {
	server := &server{db: db, logger: logger, queries: store.NewQueries(db), orders: orders.New(db), identity: identity.New(db, jwtSecret), partnerOrderEvents: newPartnerOrderHub()}
	router := chi.NewRouter()
	router.Use(middleware.RequestID)
	router.Use(middleware.RealIP)
	router.Use(middleware.Recoverer)
	router.Use(requestTimeout(10 * time.Second))
	router.Use(cors)
	router.Get("/health/live", server.live)
	router.Get("/health/ready", server.ready)
	router.Get("/api/v1/restaurants", server.listRestaurants)
	router.Get("/api/v1/restaurants/{restaurantID}/menu", server.getMenu)
	router.Post("/api/v1/auth/register", server.register)
	router.Post("/api/v1/auth/login", server.login)
	router.Post("/api/v1/auth/refresh", server.refresh)
	router.Get("/api/v1/auth/me", server.me)
	router.Get("/api/v1/addresses", server.listAddresses)
	router.Post("/api/v1/addresses", server.createAddress)
	router.Patch("/api/v1/addresses/{addressID}", server.updateAddress)
	router.Delete("/api/v1/addresses/{addressID}", server.deleteAddress)
	router.Post("/api/v1/orders/quote", server.quoteOrder)
	router.Get("/api/v1/orders", server.listUserOrders)
	router.Post("/api/v1/orders", server.createOrder)
	router.Get("/api/v1/orders/{orderID}", server.getOrder)
	router.Get("/api/v1/orders/{orderID}/events", server.streamOrder)
	router.Get("/partner/v1/menu", server.getPartnerMenu)
	router.Put("/partner/v1/menu", server.replacePartnerMenu)
	router.Get("/partner/v1/orders", server.listPartnerOrders)
	router.Get("/partner/v1/orders/events", server.streamPartnerOrders)
	router.Patch("/partner/v1/orders/{orderID}/status", server.updatePartnerOrderStatus)
	return router
}

func (s *server) live(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *server) ready(w http.ResponseWriter, r *http.Request) {
	if err := s.db.Ping(r.Context()); err != nil {
		s.logger.Error("readiness check failed", "error", err)
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "unavailable"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	if w.Header().Get("Content-Type") == "" {
		w.Header().Set("Content-Type", "application/json")
	}
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func (s *server) listRestaurants(w http.ResponseWriter, r *http.Request) {
	items, err := s.queries.ListRestaurants(r.Context())
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *server) getMenu(w http.ResponseWriter, r *http.Request) {
	restaurantID, err := uuid.Parse(chi.URLParam(r, "restaurantID"))
	if err != nil {
		s.fail(w, r, fmt.Errorf("%w: invalid restaurant id", domain.ErrValidation))
		return
	}
	menu, err := s.queries.GetMenu(r.Context(), restaurantID)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, menu)
}

func (s *server) register(w http.ResponseWriter, r *http.Request) {
	var input identity.Registration
	if err := decodeJSON(r, &input); err != nil {
		s.fail(w, r, err)
		return
	}
	session, err := s.identity.Register(r.Context(), input)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, session)
}

func (s *server) login(w http.ResponseWriter, r *http.Request) {
	var input identity.Login
	if err := decodeJSON(r, &input); err != nil {
		s.fail(w, r, err)
		return
	}
	session, err := s.identity.Login(r.Context(), input)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, session)
}

func (s *server) refresh(w http.ResponseWriter, r *http.Request) {
	var input struct {
		RefreshToken string `json:"refreshToken"`
	}
	if err := decodeJSON(r, &input); err != nil {
		s.fail(w, r, err)
		return
	}
	session, err := s.identity.Refresh(r.Context(), input.RefreshToken)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, session)
}

func (s *server) me(w http.ResponseWriter, r *http.Request) {
	uid, err := s.userID(r)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	user, err := s.identity.GetUser(r.Context(), uid)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, user)
}

func (s *server) listAddresses(w http.ResponseWriter, r *http.Request) {
	uid, err := s.userID(r)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	items, err := s.identity.ListAddresses(r.Context(), uid)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *server) createAddress(w http.ResponseWriter, r *http.Request) {
	uid, err := s.userID(r)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	var input identity.AddressInput
	if err = decodeJSON(r, &input); err != nil {
		s.fail(w, r, err)
		return
	}
	item, err := s.identity.CreateAddress(r.Context(), uid, input)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func (s *server) updateAddress(w http.ResponseWriter, r *http.Request) {
	uid, err := s.userID(r)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	addressID, err := uuid.Parse(chi.URLParam(r, "addressID"))
	if err != nil {
		s.fail(w, r, fmt.Errorf("%w: invalid address id", domain.ErrValidation))
		return
	}
	var patch identity.AddressPatch
	if err = decodeJSON(r, &patch); err != nil {
		s.fail(w, r, err)
		return
	}
	item, err := s.identity.UpdateAddress(r.Context(), uid, addressID, patch)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *server) deleteAddress(w http.ResponseWriter, r *http.Request) {
	uid, err := s.userID(r)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	addressID, err := uuid.Parse(chi.URLParam(r, "addressID"))
	if err != nil {
		s.fail(w, r, fmt.Errorf("%w: invalid address id", domain.ErrValidation))
		return
	}
	if err = s.identity.DeleteAddress(r.Context(), uid, addressID); err != nil {
		s.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) quoteOrder(w http.ResponseWriter, r *http.Request) {
	var draft domain.OrderDraft
	if err := decodeJSON(r, &draft); err != nil {
		s.fail(w, r, err)
		return
	}
	quote, err := s.orders.Quote(r.Context(), draft)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, quote)
}

func (s *server) createOrder(w http.ResponseWriter, r *http.Request) {
	uid, err := s.userID(r)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	var draft domain.OrderDraft
	if err := decodeJSON(r, &draft); err != nil {
		s.fail(w, r, err)
		return
	}
	order, existing, err := s.orders.Create(r.Context(), uid, r.Header.Get("Idempotency-Key"), draft)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	status := http.StatusCreated
	if existing {
		status = http.StatusOK
	} else {
		s.partnerOrderEvents.publish(order.RestaurantID)
	}
	writeJSON(w, status, order)
}

func (s *server) getOrder(w http.ResponseWriter, r *http.Request) {
	uid, err := s.userID(r)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	orderID, err := uuid.Parse(chi.URLParam(r, "orderID"))
	if err != nil {
		s.fail(w, r, fmt.Errorf("%w: invalid order id", domain.ErrValidation))
		return
	}
	order, err := s.orders.Get(r.Context(), orderID, uid)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, order)
}

func (s *server) listUserOrders(w http.ResponseWriter, r *http.Request) {
	uid, err := s.userID(r)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	items, err := s.orders.ListForUser(r.Context(), uid)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *server) streamOrder(w http.ResponseWriter, r *http.Request) {
	uid, err := s.userID(r)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	orderID, err := uuid.Parse(chi.URLParam(r, "orderID"))
	if err != nil {
		s.fail(w, r, fmt.Errorf("%w: invalid order id", domain.ErrValidation))
		return
	}
	order, err := s.orders.Get(r.Context(), orderID, uid)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		s.fail(w, r, errors.New("streaming is unsupported"))
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	sendOrderEvent(w, flusher, order)
	if terminalStatus(order.Status) {
		return
	}

	ticker := time.NewTicker(750 * time.Millisecond)
	defer ticker.Stop()
	lastStatus := order.Status
	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			current, getErr := s.orders.Get(r.Context(), orderID, uid)
			if getErr != nil {
				return
			}
			if current.Status == lastStatus {
				continue
			}
			lastStatus = current.Status
			sendOrderEvent(w, flusher, current)
			if terminalStatus(current.Status) {
				return
			}
		}
	}
}

func sendOrderEvent(w http.ResponseWriter, flusher http.Flusher, order domain.Order) {
	data, err := json.Marshal(order)
	if err != nil {
		return
	}
	_, _ = fmt.Fprintf(w, "event: order\ndata: %s\n\n", data)
	flusher.Flush()
}

func terminalStatus(status domain.OrderStatus) bool {
	return status == domain.OrderDelivered || status == domain.OrderCancelled || status == domain.OrderRejected
}

func (s *server) getPartnerMenu(w http.ResponseWriter, r *http.Request) {
	restaurantID, err := s.queries.AuthenticatePartner(r.Context(), r.Header.Get("X-API-Key"))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	menu, err := s.queries.GetPartnerMenu(r.Context(), restaurantID)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, menu)
}

func (s *server) replacePartnerMenu(w http.ResponseWriter, r *http.Request) {
	restaurantID, err := s.queries.AuthenticatePartner(r.Context(), r.Header.Get("X-API-Key"))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	var menu domain.PartnerMenu
	if err := decodeJSON(r, &menu); err != nil {
		s.fail(w, r, err)
		return
	}
	if err := s.queries.ReplaceMenu(r.Context(), restaurantID, menu); err != nil {
		s.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) streamPartnerOrders(w http.ResponseWriter, r *http.Request) {
	restaurantID, err := s.queries.AuthenticatePartner(r.Context(), r.Header.Get("X-API-Key"))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		s.fail(w, r, errors.New("streaming is unsupported"))
		return
	}
	updates, unsubscribe := s.partnerOrderEvents.subscribe(restaurantID)
	defer unsubscribe()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	if err := s.sendPartnerOrdersEvent(r, w, flusher, restaurantID); err != nil {
		s.logger.Error("send initial partner order event", "error", err, "restaurantId", restaurantID)
		return
	}

	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-updates:
			if err := s.sendPartnerOrdersEvent(r, w, flusher, restaurantID); err != nil {
				s.logger.Error("send partner order event", "error", err, "restaurantId", restaurantID)
				return
			}
		case <-heartbeat.C:
			if _, err := io.WriteString(w, ": keep-alive\n\n"); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

func (s *server) sendPartnerOrdersEvent(r *http.Request, w io.Writer, flusher http.Flusher, restaurantID uuid.UUID) error {
	items, err := s.orders.ListForRestaurant(r.Context(), restaurantID)
	if err != nil {
		return err
	}
	data, err := json.Marshal(map[string]any{"items": items})
	if err != nil {
		return fmt.Errorf("marshal partner orders: %w", err)
	}
	if _, err := fmt.Fprintf(w, "event: orders\ndata: %s\n\n", data); err != nil {
		return fmt.Errorf("write partner orders: %w", err)
	}
	flusher.Flush()
	return nil
}
func (s *server) updatePartnerOrderStatus(w http.ResponseWriter, r *http.Request) {
	restaurantID, err := s.queries.AuthenticatePartner(r.Context(), r.Header.Get("X-API-Key"))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	orderID, err := uuid.Parse(chi.URLParam(r, "orderID"))
	if err != nil {
		s.fail(w, r, fmt.Errorf("%w: invalid order id", domain.ErrValidation))
		return
	}
	var input struct {
		Status domain.OrderStatus `json:"status"`
		Reason string             `json:"reason"`
	}
	if err := decodeJSON(r, &input); err != nil {
		s.fail(w, r, err)
		return
	}
	order, err := s.orders.UpdatePartnerStatus(r.Context(), orderID, restaurantID, input.Status, input.Reason)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.partnerOrderEvents.publish(restaurantID)
	writeJSON(w, http.StatusOK, order)
}

func (s *server) listPartnerOrders(w http.ResponseWriter, r *http.Request) {
	restaurantID, err := s.queries.AuthenticatePartner(r.Context(), r.Header.Get("X-API-Key"))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	items, err := s.orders.ListForRestaurant(r.Context(), restaurantID)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *server) userID(r *http.Request) (uuid.UUID, error) {
	authorization := strings.TrimSpace(r.Header.Get("Authorization"))
	if strings.HasPrefix(strings.ToLower(authorization), "bearer ") {
		return s.identity.AuthenticateAccess(strings.TrimSpace(authorization[7:]))
	}
	return uuid.Nil, fmt.Errorf("%w: bearer token is required", domain.ErrUnauthorized)
}

func decodeJSON(r *http.Request, target any) error {
	decoder := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("%w: invalid JSON: %w", domain.ErrValidation, err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return fmt.Errorf("%w: request must contain one JSON object", domain.ErrValidation)
	}
	return nil
}

func (s *server) fail(w http.ResponseWriter, r *http.Request, err error) {
	status := http.StatusInternalServerError
	title := "Internal server error"
	typeName := "about:blank"
	switch {
	case errors.Is(err, domain.ErrValidation):
		status, title, typeName = http.StatusUnprocessableEntity, "Validation failed", "/problems/validation"
	case errors.Is(err, domain.ErrUnauthorized):
		status, title, typeName = http.StatusUnauthorized, "Unauthorized", "/problems/unauthorized"
	case errors.Is(err, domain.ErrNotFound):
		status, title, typeName = http.StatusNotFound, "Not found", "/problems/not-found"
	case errors.Is(err, domain.ErrConflict):
		status, title, typeName = http.StatusConflict, "Conflict", "/problems/conflict"
	default:
		s.logger.Error("request failed", "error", err, "requestId", middleware.GetReqID(r.Context()))
	}
	w.Header().Set("Content-Type", "application/problem+json")
	writeJSON(w, status, map[string]any{
		"type": typeName, "title": title, "status": status,
		"detail": err.Error(), "requestId": middleware.GetReqID(r.Context()),
	})
}

func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Request-ID", middleware.GetReqID(r.Context()))
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-API-Key, Idempotency-Key")
		w.Header().Set("Access-Control-Expose-Headers", "X-Request-ID")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func requestTimeout(duration time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		regular := middleware.Timeout(duration)(next)
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasSuffix(r.URL.Path, "/events") {
				next.ServeHTTP(w, r)
				return
			}
			regular.ServeHTTP(w, r)
		})
	}
}
