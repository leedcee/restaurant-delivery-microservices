package store

import (
	"testing"

	"restaurant-delivery-system/internal/domain"
)

func TestValidatePartnerMenu(t *testing.T) {
	t.Parallel()
	valid := domain.PartnerMenu{Categories: []domain.PartnerCategory{{
		ExternalID: "main", Name: "Основное", Products: []domain.PartnerProduct{{
			ExternalID: "soup", Name: "Суп", PriceMinor: 35000, Quantity: 0, Available: false,
		}},
	}}}
	tests := []struct {
		name    string
		menu    domain.PartnerMenu
		wantErr bool
	}{
		{name: "valid stop-listed product", menu: valid},
		{name: "empty menu", menu: domain.PartnerMenu{}},
		{name: "missing category name", menu: domain.PartnerMenu{Categories: []domain.PartnerCategory{{ExternalID: "main"}}}, wantErr: true},
		{name: "duplicate category", menu: domain.PartnerMenu{Categories: []domain.PartnerCategory{{ExternalID: "same", Name: "One"}, {ExternalID: "same", Name: "Two"}}}, wantErr: true},
		{name: "negative price", menu: domain.PartnerMenu{Categories: []domain.PartnerCategory{{ExternalID: "main", Name: "Main", Products: []domain.PartnerProduct{{ExternalID: "bad", Name: "Bad", PriceMinor: -1}}}}}, wantErr: true},
		{name: "duplicate product across categories", menu: domain.PartnerMenu{Categories: []domain.PartnerCategory{{ExternalID: "one", Name: "One", Products: []domain.PartnerProduct{{ExternalID: "same", Name: "One"}}}, {ExternalID: "two", Name: "Two", Products: []domain.PartnerProduct{{ExternalID: "same", Name: "Two"}}}}}, wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			err := validatePartnerMenu(test.menu)
			if (err != nil) != test.wantErr {
				t.Fatalf("validatePartnerMenu() error = %v, wantErr %v", err, test.wantErr)
			}
		})
	}
}
