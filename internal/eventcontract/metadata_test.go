package eventcontract

import (
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestMetadataRoundTrip(t *testing.T) {
	want := Metadata{
		ID: uuid.New(), Type: OrderCreated, Version: OrderCreatedVersion,
		CorrelationID: uuid.New(), OccurredAt: time.Date(2026, 10, 21, 10, 11, 12, 123, time.FixedZone("test", 4*60*60)),
	}
	header := make(http.Header)
	want.Apply(header)
	got, err := Parse(header)
	if err != nil {
		t.Fatalf("parse applied metadata: %v", err)
	}
	if got.ID != want.ID || got.Type != want.Type || got.Version != want.Version || got.CorrelationID != want.CorrelationID || !got.OccurredAt.Equal(want.OccurredAt) {
		t.Fatalf("metadata mismatch: got %+v, want %+v", got, want)
	}
	if !got.IsOrderCreatedV1() {
		t.Fatal("expected supported order.created v1 event")
	}
}

func TestParseRejectsIncompleteMetadata(t *testing.T) {
	header := make(http.Header)
	header.Set("X-Event-ID", uuid.NewString())
	if _, err := Parse(header); err == nil {
		t.Fatal("expected incomplete metadata to be rejected")
	}
}
