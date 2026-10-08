package httpapi

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestPartnerOrderHubIsolatesRestaurantsAndCoalescesUpdates(t *testing.T) {
	hub := newPartnerOrderHub()
	restaurantA, restaurantB := uuid.New(), uuid.New()
	updatesA, unsubscribeA := hub.subscribe(restaurantA)
	updatesB, unsubscribeB := hub.subscribe(restaurantB)
	defer unsubscribeB()

	hub.publish(restaurantA)
	hub.publish(restaurantA)
	select {
	case <-updatesA:
	case <-time.After(time.Second):
		t.Fatal("restaurant A did not receive an update")
	}
	select {
	case <-updatesB:
		t.Fatal("restaurant B received another restaurant's update")
	default:
	}

	unsubscribeA()
	hub.publish(restaurantA)
	select {
	case <-updatesA:
		t.Fatal("unsubscribed restaurant received an update")
	default:
	}
}
