//go:build e2e

package e2e

import (
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"testing"
	"time"

	"restaurant-delivery-system/internal/domain"

	"github.com/google/uuid"
)

func TestCustomerIdentityJourney(t *testing.T) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("create cookie jar: %v", err)
	}
	client := &http.Client{Jar: jar}
	email := "e2e-" + uuid.NewString() + "@example.test"
	registration := map[string]string{"email": email, "password": "strong-password", "name": "E2E Customer"}
	var registered domain.AuthSession
	doJSON(t, client, http.MethodPost, baseURL+"/api/v1/auth/register", registration, nil,
		http.StatusCreated, &registered)
	if registered.User.Email != email || registered.AccessToken == "" || registered.RefreshToken != "" {
		t.Fatalf("invalid registration session: %+v", registered)
	}

	var current domain.User
	doJSON(t, client, http.MethodGet, baseURL+"/api/v1/auth/me", nil,
		map[string]string{"Authorization": "Bearer " + registered.AccessToken}, http.StatusOK, &current)
	if current.ID != registered.User.ID {
		t.Fatalf("me returned user %s, want %s", current.ID, registered.User.ID)
	}

	var loggedIn domain.AuthSession
	doJSON(t, client, http.MethodPost, baseURL+"/api/v1/auth/login",
		map[string]string{"email": email, "password": "strong-password"}, nil,
		http.StatusOK, &loggedIn)
	authURL, err := url.Parse(baseURL + "/api/v1/auth/refresh")
	if err != nil {
		t.Fatalf("parse auth URL: %v", err)
	}
	oldRefresh := cookieValue(jar.Cookies(authURL), "looch_refresh")
	if oldRefresh == "" {
		t.Fatal("login did not set the HttpOnly refresh cookie")
	}

	var refreshed domain.AuthSession
	doJSON(t, client, http.MethodPost, baseURL+"/api/v1/auth/refresh", nil, nil, http.StatusOK, &refreshed)
	newRefresh := cookieValue(jar.Cookies(authURL), "looch_refresh")
	if newRefresh == "" || newRefresh == oldRefresh {
		t.Fatal("refresh cookie was not rotated")
	}
	plainClient := &http.Client{}
	status, _ := requestJSON(t.Context(), plainClient, http.MethodPost, baseURL+"/api/v1/auth/refresh",
		nil, map[string]string{"Cookie": "looch_refresh=" + oldRefresh})
	if status != http.StatusUnauthorized {
		t.Fatalf("reused refresh token returned %d, want 401", status)
	}
	status, _ = requestJSON(t.Context(), plainClient, http.MethodPost, baseURL+"/api/v1/auth/refresh", nil,
		map[string]string{"Cookie": "looch_refresh=" + newRefresh})
	if status != http.StatusUnauthorized {
		t.Fatalf("token family survived reuse detection: got %d, want 401", status)
	}
}

func cookieValue(cookies []*http.Cookie, name string) string {
	for _, cookie := range cookies {
		if cookie.Name == name {
			return cookie.Value
		}
	}
	return ""
}

func TestAuthenticatedOrderSurvivesRelogin(t *testing.T) {
	client := &http.Client{Timeout: 5 * time.Second}
	productID := publishSingleProduct(t, client, "identity-history", 3)
	email := "history-" + uuid.NewString() + "@example.test"
	var registered domain.AuthSession
	doJSON(t, client, http.MethodPost, baseURL+"/api/v1/auth/register", map[string]string{
		"email": email, "password": "strong-password", "name": "History Customer",
	}, nil, http.StatusCreated, &registered)

	draft := newDraft(productID, 1)
	draft.DeliveryAddress = "Самара, история заказов"
	var created domain.Order
	doJSON(t, client, http.MethodPost, baseURL+"/api/v1/orders", draft, map[string]string{
		"Authorization":   "Bearer " + registered.AccessToken,
		"Idempotency-Key": "identity-history-" + uuid.NewString(),
	}, http.StatusCreated, &created)

	var loggedIn domain.AuthSession
	doJSON(t, client, http.MethodPost, baseURL+"/api/v1/auth/login", map[string]string{
		"email": email, "password": "strong-password",
	}, nil, http.StatusOK, &loggedIn)
	var history struct {
		Items []domain.Order `json:"items"`
	}
	doJSON(t, client, http.MethodGet, baseURL+"/api/v1/orders", nil,
		map[string]string{"Authorization": "Bearer " + loggedIn.AccessToken}, http.StatusOK, &history)
	if len(history.Items) != 1 || history.Items[0].ID != created.ID {
		t.Fatalf("relogged customer history does not contain created order: %+v", history.Items)
	}

	var stranger domain.AuthSession
	doJSON(t, client, http.MethodPost, baseURL+"/api/v1/auth/register", map[string]string{
		"email": "stranger-" + uuid.NewString() + "@example.test", "password": "strong-password", "name": "Stranger",
	}, nil, http.StatusCreated, &stranger)
	doJSON(t, client, http.MethodGet, baseURL+"/api/v1/orders", nil,
		map[string]string{"Authorization": "Bearer " + stranger.AccessToken}, http.StatusOK, &history)
	if len(history.Items) != 0 {
		t.Fatalf("stranger can see another customer's orders: %+v", history.Items)
	}
}

func TestCustomerAddressJourney(t *testing.T) {
	client := &http.Client{}
	var session domain.AuthSession
	doJSON(t, client, http.MethodPost, baseURL+"/api/v1/auth/register", map[string]string{
		"email": "address-" + uuid.NewString() + "@example.test", "password": "strong-password", "name": "Address Customer",
	}, nil, http.StatusCreated, &session)
	headers := map[string]string{"Authorization": "Bearer " + session.AccessToken}

	var home domain.Address
	doJSON(t, client, http.MethodPost, baseURL+"/api/v1/addresses", map[string]any{
		"label": "Дом", "address": "Самара, Московское шоссе, 15", "isDefault": false,
	}, headers, http.StatusCreated, &home)
	if !home.IsDefault {
		t.Fatal("first address must become default")
	}

	var work domain.Address
	doJSON(t, client, http.MethodPost, baseURL+"/api/v1/addresses", map[string]any{
		"label": "Работа", "address": "Самара, улица Молодогвардейская, 151", "isDefault": false,
	}, headers, http.StatusCreated, &work)
	doJSON(t, client, http.MethodPatch, baseURL+"/api/v1/addresses/"+work.ID.String(), map[string]any{
		"isDefault": true,
	}, headers, http.StatusOK, &work)
	if !work.IsDefault {
		t.Fatal("updated address must become default")
	}

	var list struct {
		Items []domain.Address `json:"items"`
	}
	doJSON(t, client, http.MethodGet, baseURL+"/api/v1/addresses", nil, headers, http.StatusOK, &list)
	if len(list.Items) != 2 || list.Items[0].ID != work.ID || !list.Items[0].IsDefault {
		t.Fatalf("unexpected address list: %+v", list.Items)
	}

	doJSON(t, client, http.MethodDelete, baseURL+"/api/v1/addresses/"+work.ID.String(), nil, headers, http.StatusNoContent, nil)
	doJSON(t, client, http.MethodGet, baseURL+"/api/v1/addresses", nil, headers, http.StatusOK, &list)
	if len(list.Items) != 1 || list.Items[0].ID != home.ID || !list.Items[0].IsDefault {
		t.Fatalf("remaining address must be promoted to default: %+v", list.Items)
	}
}
