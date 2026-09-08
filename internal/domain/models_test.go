package domain

import "testing"

func TestCanTransition(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		from OrderStatus
		to   OrderStatus
		want bool
	}{
		{name: "restaurant accepts pending", from: OrderPending, to: OrderAccepted, want: true},
		{name: "restaurant rejects pending", from: OrderPending, to: OrderRejected, want: true},
		{name: "accepted starts preparing", from: OrderAccepted, to: OrderPreparing, want: true},
		{name: "cannot deliver pending", from: OrderPending, to: OrderDelivered, want: false},
		{name: "cannot reopen delivered", from: OrderDelivered, to: OrderAccepted, want: false},
		{name: "same status is not a transition", from: OrderAccepted, to: OrderAccepted, want: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got := CanTransition(test.from, test.to); got != test.want {
				t.Fatalf("CanTransition(%q, %q) = %v, want %v", test.from, test.to, got, test.want)
			}
		})
	}
}
