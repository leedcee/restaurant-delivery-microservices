// Package eventcontract defines the versioned HTTP contract shared with partner services.
package eventcontract

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	OrderCreated        = "order.created"
	OrderCreatedVersion = 1
)

type Metadata struct {
	ID            uuid.UUID
	Type          string
	Version       int
	CorrelationID uuid.UUID
	OccurredAt    time.Time
}

func (m Metadata) Apply(header http.Header) {
	header.Set("Idempotency-Key", m.ID.String())
	header.Set("X-Event-ID", m.ID.String())
	header.Set("X-Event-Type", m.Type)
	header.Set("X-Event-Version", strconv.Itoa(m.Version))
	header.Set("X-Correlation-ID", m.CorrelationID.String())
	header.Set("X-Event-Occurred-At", m.OccurredAt.UTC().Format(time.RFC3339Nano))
}

func Parse(header http.Header) (Metadata, error) {
	id, err := uuid.Parse(strings.TrimSpace(header.Get("X-Event-ID")))
	if err != nil {
		return Metadata{}, fmt.Errorf("invalid event id: %w", err)
	}
	if header.Get("Idempotency-Key") != id.String() {
		return Metadata{}, fmt.Errorf("idempotency key must match event id")
	}
	eventType := strings.TrimSpace(header.Get("X-Event-Type"))
	if eventType == "" {
		return Metadata{}, fmt.Errorf("event type is required")
	}
	version, err := strconv.Atoi(header.Get("X-Event-Version"))
	if err != nil || version < 1 {
		return Metadata{}, fmt.Errorf("event version must be a positive integer")
	}
	correlationID, err := uuid.Parse(strings.TrimSpace(header.Get("X-Correlation-ID")))
	if err != nil {
		return Metadata{}, fmt.Errorf("invalid correlation id: %w", err)
	}
	occurredAt, err := time.Parse(time.RFC3339Nano, header.Get("X-Event-Occurred-At"))
	if err != nil {
		return Metadata{}, fmt.Errorf("invalid occurred at: %w", err)
	}
	return Metadata{ID: id, Type: eventType, Version: version, CorrelationID: correlationID, OccurredAt: occurredAt}, nil
}

func (m Metadata) IsOrderCreatedV1() bool {
	return m.Type == OrderCreated && m.Version == OrderCreatedVersion
}
