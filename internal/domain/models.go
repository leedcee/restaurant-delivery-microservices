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

type Order struct {
	ID               uuid.UUID   `json:"id"`
	UserID           uuid.UUID   `json:"userId"`
	RestaurantID     uuid.UUID   `json:"restaurantId"`
	Items            []QuoteItem `json:"items"`
	DeliveryFeeMinor int64       `json:"deliveryFeeMinor"`
	TotalMinor       int64       `json:"totalMinor"`
	Currency         string      `json:"currency"`
	Status           OrderStatus `json:"status"`
	DeliveryAddress  string      `json:"deliveryAddress"`
	RejectionReason  *string     `json:"rejectionReason"`
	CreatedAt        time.Time   `json:"createdAt"`
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
