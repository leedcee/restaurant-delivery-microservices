//go:build e2e

package e2e

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"restaurant-delivery-system/internal/domain"

	"github.com/google/uuid"
)

const (
	pastaRestaurantID = "22222222-2222-2222-2222-222222222222"
	sushiRestaurantID = "33333333-3333-3333-3333-333333333333"
	pastaAPIKey       = "pasta-secret"
	sushiAPIKey       = "sushi-secret"
)

func TestPartnerFixtures(t *testing.T) {
	client := &http.Client{}

	t.Run("catalog exposes complete restaurant fixtures", func(t *testing.T) {
		var catalog struct {
			Items []domain.Restaurant `json:"items"`
		}
		doJSON(t, client, http.MethodGet, baseURL+"/api/v1/restaurants", nil, nil, http.StatusOK, &catalog)
		if len(catalog.Items) != 3 {
			t.Fatalf("catalog contains %d restaurants, want 3: %+v", len(catalog.Items), catalog.Items)
		}
		for _, id := range []string{demoRestaurantID, pastaRestaurantID, sushiRestaurantID} {
			restaurant := findRestaurant(catalog.Items, uuid.MustParse(id))
			if restaurant == nil {
				t.Fatalf("restaurant %s is absent from catalog", id)
			}
			if restaurant.Name == "" || restaurant.Cuisine == "" || restaurant.ETA == "" || restaurant.Rating == "" || restaurant.Art == "" {
				t.Fatalf("restaurant fixture is incomplete: %+v", *restaurant)
			}
		}
	})

	t.Run("fixture menus contain available products", func(t *testing.T) {
		for _, id := range []string{pastaRestaurantID, sushiRestaurantID} {
			menu := fetchRestaurantMenu(t, client, id)
			products := menuProducts(menu)
			if len(menu.Categories) < 2 || len(products) < 3 {
				t.Fatalf("restaurant %s has an incomplete menu: %+v", id, menu.Categories)
			}
			for _, product := range products {
				if !product.Available || product.Quantity < 1 || product.PriceMinor < 1 {
					t.Fatalf("invalid fixture product for restaurant %s: %+v", id, product)
				}
			}
		}
	})

	t.Run("partner menu update is isolated by API key", func(t *testing.T) {
		sushiBefore := fetchRestaurantMenu(t, client, sushiRestaurantID)
		externalID := "partner-isolation-" + uuid.NewString()
		payload := domain.PartnerMenu{Categories: []domain.PartnerCategory{{
			ExternalID: externalID, Name: "Изолированное меню", Position: 1,
			Products: []domain.PartnerProduct{
				{ExternalID: externalID, Name: "Тестовая паста", PriceMinor: 44000, Quantity: 7, Available: true},
				{ExternalID: externalID + "-stop", Name: "Паста в стоп-листе", PriceMinor: 51000, Quantity: 3, Available: false},
			},
		}}}
		doJSON(t, client, http.MethodPut, baseURL+"/partner/v1/menu", payload,
			map[string]string{"X-API-Key": pastaAPIKey}, http.StatusNoContent, nil)

		pastaAfter := fetchRestaurantMenu(t, client, pastaRestaurantID)
		products := menuProducts(pastaAfter)
		if len(products) != 2 || firstAvailableProduct(products).Name != "Тестовая паста" {
			t.Fatalf("pasta partner menu was not replaced: %+v", products)
		}
		var editable domain.PartnerMenu
		doJSON(t, client, http.MethodGet, baseURL+"/partner/v1/menu", nil,
			map[string]string{"X-API-Key": pastaAPIKey}, http.StatusOK, &editable)
		stopListed := false
		if len(editable.Categories) == 1 {
			for _, product := range editable.Categories[0].Products {
				if product.ExternalID == externalID+"-stop" && !product.Available && product.Quantity == 3 {
					stopListed = true
				}
			}
		}
		if len(editable.Categories) != 1 || len(editable.Categories[0].Products) != 2 || !stopListed {
			t.Fatalf("editable menu did not preserve identifiers or stop-list state: %+v", editable)
		}
		sushiAfter := fetchRestaurantMenu(t, client, sushiRestaurantID)
		if len(menuProducts(sushiAfter)) != len(menuProducts(sushiBefore)) {
			t.Fatalf("pasta partner changed sushi menu: before=%+v after=%+v", sushiBefore.Categories, sushiAfter.Categories)
		}
	})

	t.Run("products cannot be used across restaurants", func(t *testing.T) {
		sushiMenu := fetchRestaurantMenu(t, client, sushiRestaurantID)
		sushiProduct := menuProducts(sushiMenu)[0]
		draft := domain.OrderDraft{
			RestaurantID: uuid.MustParse(pastaRestaurantID),
			Items:        []domain.DraftItem{{ProductID: sushiProduct.ID, Quantity: 1}},
		}
		status, body := requestJSON(t.Context(), client, http.MethodPost, baseURL+"/api/v1/orders/quote", draft, nil)
		if status != http.StatusUnprocessableEntity {
			t.Fatalf("cross-restaurant product returned %d, want 422: %s", status, body)
		}
	})

	t.Run("partner cannot read or update another restaurant order", func(t *testing.T) {
		db := openTestDB(t)
		menu := fetchRestaurantMenu(t, client, pastaRestaurantID)
		product := firstAvailableProduct(menuProducts(menu))
		setRestaurantEndpoint(t, db, pastaRestaurantID, brokenEndpoint)
		t.Cleanup(func() { setRestaurantEndpoint(t, db, pastaRestaurantID, demoEndpoint) })

		draft := domain.OrderDraft{
			RestaurantID:    uuid.MustParse(pastaRestaurantID),
			DeliveryAddress: "Самара, партнёрская изоляция",
			Items:           []domain.DraftItem{{ProductID: product.ID, Quantity: 1}},
		}
		status, body := createOrderRequest(t.Context(), client, draft, registerAccessToken(t, client))
		if status != http.StatusCreated {
			t.Fatalf("create fixture restaurant order returned %d: %s", status, body)
		}
		var order domain.Order
		if err := json.Unmarshal([]byte(body), &order); err != nil {
			t.Fatalf("decode fixture restaurant order: %v", err)
		}
		t.Cleanup(func() {
			_, _ = db.Exec(context.Background(), `DELETE FROM outbox_events WHERE aggregate_id = $1`, order.ID)
			_, _ = db.Exec(context.Background(), `DELETE FROM orders WHERE id = $1`, order.ID)
		})

		var pastaOrders, sushiOrders struct {
			Items []domain.Order `json:"items"`
		}
		doJSON(t, client, http.MethodGet, baseURL+"/partner/v1/orders", nil,
			map[string]string{"X-API-Key": pastaAPIKey}, http.StatusOK, &pastaOrders)
		doJSON(t, client, http.MethodGet, baseURL+"/partner/v1/orders", nil,
			map[string]string{"X-API-Key": sushiAPIKey}, http.StatusOK, &sushiOrders)
		if !containsOrder(pastaOrders.Items, order.ID) || containsOrder(sushiOrders.Items, order.ID) {
			t.Fatalf("partner order isolation failed: pasta=%+v sushi=%+v", pastaOrders.Items, sushiOrders.Items)
		}

		status, body = requestJSON(t.Context(), client, http.MethodPatch,
			baseURL+"/partner/v1/orders/"+order.ID.String()+"/status",
			map[string]string{"status": string(domain.OrderAccepted)}, map[string]string{"X-API-Key": sushiAPIKey})
		if status != http.StatusNotFound {
			t.Fatalf("foreign partner update returned %d, want 404: %s", status, body)
		}
	})
}

func fetchRestaurantMenu(t *testing.T, client *http.Client, restaurantID string) domain.Menu {
	t.Helper()
	var menu domain.Menu
	doJSON(t, client, http.MethodGet, baseURL+"/api/v1/restaurants/"+restaurantID+"/menu",
		nil, nil, http.StatusOK, &menu)
	return menu
}

func menuProducts(menu domain.Menu) []domain.Product {
	products := make([]domain.Product, 0)
	for _, category := range menu.Categories {
		products = append(products, category.Products...)
	}
	return products
}

func firstAvailableProduct(items []domain.Product) domain.Product {
	for _, item := range items {
		if item.Available {
			return item
		}
	}
	return domain.Product{}
}
func findRestaurant(items []domain.Restaurant, id uuid.UUID) *domain.Restaurant {
	for index := range items {
		if items[index].ID == id {
			return &items[index]
		}
	}
	return nil
}

func containsOrder(items []domain.Order, id uuid.UUID) bool {
	for _, item := range items {
		if item.ID == id {
			return true
		}
	}
	return false
}
