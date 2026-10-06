//go:build e2e

package e2e

import (
	"net/http"
	"testing"

	"restaurant-delivery-system/internal/domain"

	"github.com/google/uuid"
)

func TestCustomerIdentityJourney(t *testing.T) {
	client := &http.Client{}
	email := "e2e-" + uuid.NewString() + "@example.test"
	registration := map[string]string{"email": email, "password": "strong-password", "name": "E2E Customer"}
	var registered domain.AuthSession
	doJSON(t, client, http.MethodPost, baseURL+"/api/v1/auth/register", registration, nil,
		http.StatusCreated, &registered)
	if registered.User.Email != email || registered.AccessToken == "" || registered.RefreshToken == "" {
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

	var refreshed domain.AuthSession
	doJSON(t, client, http.MethodPost, baseURL+"/api/v1/auth/refresh",
		map[string]string{"refreshToken": loggedIn.RefreshToken}, nil, http.StatusOK, &refreshed)
	if refreshed.RefreshToken == loggedIn.RefreshToken {
		t.Fatal("refresh token was not rotated")
	}
	status, _ := requestJSON(t.Context(), client, http.MethodPost, baseURL+"/api/v1/auth/refresh",
		map[string]string{"refreshToken": loggedIn.RefreshToken}, nil)
	if status != http.StatusUnauthorized {
		t.Fatalf("reused refresh token returned %d, want 401", status)
	}
}
