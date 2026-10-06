package identity

import (
	"errors"
	"testing"
	"time"

	"restaurant-delivery-system/internal/domain"

	"github.com/google/uuid"
)

func TestAccessTokenRoundTrip(t *testing.T) {
	t.Parallel()
	service := New(nil, "test-secret")
	service.now = func() time.Time { return time.Unix(1_800_000_000, 0) }
	want := uuid.New()
	token, err := service.signAccess(want)
	if err != nil {
		t.Fatalf("sign access token: %v", err)
	}
	got, err := service.AuthenticateAccess(token)
	if err != nil {
		t.Fatalf("authenticate access token: %v", err)
	}
	if got != want {
		t.Fatalf("got user %s, want %s", got, want)
	}
}

func TestAccessTokenRejectsTamperingAndExpiry(t *testing.T) {
	t.Parallel()
	service := New(nil, "test-secret")
	issued := time.Unix(1_800_000_000, 0)
	service.now = func() time.Time { return issued }
	token, err := service.signAccess(uuid.New())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.AuthenticateAccess(token + "x"); !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("tampered token: got %v, want unauthorized", err)
	}
	service.now = func() time.Time { return issued.Add(accessTTL + time.Second) }
	if _, err = service.AuthenticateAccess(token); !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("expired token: got %v, want unauthorized", err)
	}
}
