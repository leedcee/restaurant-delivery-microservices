package httpapi

import (
	"net/http/httptest"
	"strings"
	"testing"

	"restaurant-delivery-system/internal/domain"
)

func TestParseLastEventID(t *testing.T) {
	tests := []struct {
		value   string
		wantID  int64
		wantSet bool
		wantErr bool
	}{
		{value: "", wantID: 0, wantSet: false},
		{value: " 42 ", wantID: 42, wantSet: true},
		{value: "-1", wantErr: true},
		{value: "event-1", wantErr: true},
	}
	for _, test := range tests {
		id, set, err := parseLastEventID(test.value)
		if (err != nil) != test.wantErr || id != test.wantID || set != test.wantSet {
			t.Errorf("parseLastEventID(%q) = (%d, %t, %v)", test.value, id, set, err)
		}
	}
}

func TestSendOrderEventIncludesStableID(t *testing.T) {
	recorder := httptest.NewRecorder()
	if !sendOrderEvent(recorder, recorder, 17, domain.Order{Status: domain.OrderPreparing, Version: 3}) {
		t.Fatal("sendOrderEvent failed")
	}
	body := recorder.Body.String()
	for _, expected := range []string{"id: 17\n", "event: order\n", `"status":"preparing"`, `"version":3`} {
		if !strings.Contains(body, expected) {
			t.Fatalf("SSE body %q does not contain %q", body, expected)
		}
	}
}
