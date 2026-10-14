// Package domain contains business entities shared by application components.
package domain

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrNotFound     = errors.New("resource not found")
	ErrConflict     = errors.New("resource state conflict")
	ErrValidation   = errors.New("validation failed")
	ErrUnauthorized = errors.New("unauthorized")
)

type Restaurant struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	IsOpen      bool      `json:"isOpen"`
	Cuisine     string    `json:"cuisine"`
	ETA         string    `json:"eta"`
	Rating      string    `json:"rating"`
	Art         string    `json:"art"`
}

type Product struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	PriceMinor  int64     `json:"priceMinor"`
	Currency    string    `json:"currency"`
	Available   bool      `json:"available"`
	Quantity    int       `json:"quantity"`
}

type MenuCategory struct {
	ID       uuid.UUID `json:"id"`
	Name     string    `json:"name"`
	Products []Product `json:"products"`
}

type Menu struct {
	Restaurant Restaurant     `json:"restaurant"`
	Categories []MenuCategory `json:"categories"`
}

type DraftItem struct {
	ProductID uuid.UUID `json:"productId"`
	Quantity  int       `json:"quantity"`
}

type OrderDraft struct {
	RestaurantID    uuid.UUID   `json:"restaurantId"`
	Items           []DraftItem `json:"items"`
	DeliveryAddress string      `json:"deliveryAddress,omitempty"`
}

type QuoteItem struct {
	ProductID      uuid.UUID `json:"productId"`
	Name           string    `json:"name"`
	Quantity       int       `json:"quantity"`
	UnitPriceMinor int64     `json:"unitPriceMinor"`
	TotalMinor     int64     `json:"totalMinor"`
}

type OrderQuote struct {
	RestaurantID     uuid.UUID   `json:"restaurantId"`
	Items            []QuoteItem `json:"items"`
	DeliveryFeeMinor int64       `json:"deliveryFeeMinor"`
	TotalMinor       int64       `json:"totalMinor"`
	Currency         string      `json:"currency"`
}

type OrderStatus string

type OrderReasonCode string

const (
	OrderPending    OrderStatus = "pending"
	OrderAccepted   OrderStatus = "accepted"
	OrderRejected   OrderStatus = "rejected"
	OrderPreparing  OrderStatus = "preparing"
	OrderReady      OrderStatus = "ready"
	OrderDelivering OrderStatus = "delivering"
	OrderDelivered  OrderStatus = "delivered"
	OrderCancelled  OrderStatus = "cancelled"
)

const (
	ReasonCustomerChangedMind  OrderReasonCode = "customer_changed_mind"
	ReasonWrongDeliveryAddress OrderReasonCode = "wrong_delivery_address"
	ReasonRestaurantBusy       OrderReasonCode = "restaurant_busy"
	ReasonItemUnavailable      OrderReasonCode = "item_unavailable"
	ReasonRestaurantClosing    OrderReasonCode = "restaurant_closing"
	ReasonOther                OrderReasonCode = "other"
)

type Order struct {
	ID               uuid.UUID        `json:"id"`
	UserID           uuid.UUID        `json:"userId"`
	RestaurantID     uuid.UUID        `json:"restaurantId"`
	Items            []QuoteItem      `json:"items"`
	DeliveryFeeMinor int64            `json:"deliveryFeeMinor"`
	TotalMinor       int64            `json:"totalMinor"`
	Currency         string           `json:"currency"`
	Status           OrderStatus      `json:"status"`
	Version          int64            `json:"version"`
	DeliveryAddress  string           `json:"deliveryAddress"`
	RejectionReason  *string          `json:"rejectionReason"`
	StatusReasonCode *OrderReasonCode `json:"statusReasonCode,omitempty"`
	CreatedAt        time.Time        `json:"createdAt"`
}

type OutboxFailure struct {
	EventID        uuid.UUID `json:"eventId"`
	OrderID        uuid.UUID `json:"orderId"`
	EventType      string    `json:"eventType"`
	EventVersion   int       `json:"eventVersion"`
	Attempts       int       `json:"attempts"`
	LastError      string    `json:"lastError"`
	DeadLetteredAt time.Time `json:"deadLetteredAt"`
}

func CanTransition(from, to OrderStatus) bool {
	allowed := map[OrderStatus]map[OrderStatus]bool{
		OrderPending:    {OrderAccepted: true, OrderRejected: true, OrderCancelled: true},
		OrderAccepted:   {OrderPreparing: true, OrderCancelled: true},
		OrderPreparing:  {OrderReady: true, OrderCancelled: true},
		OrderReady:      {OrderDelivering: true},
		OrderDelivering: {OrderDelivered: true},
	}
	return allowed[from][to]
}

func CanCustomerCancel(status OrderStatus) bool {
	return status == OrderPending || status == OrderAccepted
}

func ValidReasonCode(status OrderStatus, code OrderReasonCode) bool {
	switch status {
	case OrderRejected:
		return code == ReasonRestaurantBusy || code == ReasonItemUnavailable ||
			code == ReasonRestaurantClosing || code == ReasonOther
	case OrderCancelled:
		return code == ReasonCustomerChangedMind || code == ReasonWrongDeliveryAddress || code == ReasonOther
	default:
		return code == ""
	}
}

type PartnerMenu struct {
	Categories []PartnerCategory `json:"categories"`
}

type PartnerCategory struct {
	ExternalID string           `json:"externalId"`
	Name       string           `json:"name"`
	Position   int              `json:"position"`
	Products   []PartnerProduct `json:"products"`
}

type PartnerProduct struct {
	ExternalID  string `json:"externalId"`
	Name        string `json:"name"`
	Description string `json:"description"`
	PriceMinor  int64  `json:"priceMinor"`
	Quantity    int    `json:"quantity"`
	Available   bool   `json:"available"`
}

type User struct {
	ID        uuid.UUID `json:"id"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"createdAt"`
}

type AuthSession struct {
	User         User   `json:"user"`
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"-"`
	ExpiresIn    int64  `json:"expiresIn"`
}

type Address struct {
	ID        uuid.UUID `json:"id"`
	UserID    uuid.UUID `json:"userId"`
	Label     string    `json:"label"`
	Address   string    `json:"address"`
	IsDefault bool      `json:"isDefault"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}
