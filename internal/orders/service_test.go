package orders

import (
	"errors"
	"testing"

	"restaurant-delivery-system/internal/domain"

	"github.com/google/uuid"
)

func TestValidateDraft(t *testing.T) {
	t.Parallel()
	restaurantID := uuid.New()
	productID := uuid.New()
	tests := []struct {
		name           string
		draft          domain.OrderDraft
		requireAddress bool
		wantError      bool
	}{
		{name: "valid quote", draft: domain.OrderDraft{RestaurantID: restaurantID, Items: []domain.DraftItem{{ProductID: productID, Quantity: 1}}}},
		{name: "valid order", draft: domain.OrderDraft{RestaurantID: restaurantID, DeliveryAddress: "Самара", Items: []domain.DraftItem{{ProductID: productID, Quantity: 2}}}, requireAddress: true},
		{name: "missing items", draft: domain.OrderDraft{RestaurantID: restaurantID}, wantError: true},
		{name: "zero quantity", draft: domain.OrderDraft{RestaurantID: restaurantID, Items: []domain.DraftItem{{ProductID: productID}}}, wantError: true},
		{name: "missing address", draft: domain.OrderDraft{RestaurantID: restaurantID, Items: []domain.DraftItem{{ProductID: productID, Quantity: 1}}}, requireAddress: true, wantError: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			err := validateDraft(test.draft, test.requireAddress)
			if test.wantError && !errors.Is(err, domain.ErrValidation) {
				t.Fatalf("expected validation error, got %v", err)
			}
			if !test.wantError && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}
