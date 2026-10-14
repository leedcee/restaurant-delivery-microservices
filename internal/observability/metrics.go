package observability

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Metrics struct {
	registry *prometheus.Registry
	requests *prometheus.CounterVec
	duration *prometheus.HistogramVec
	active   prometheus.Gauge
}

func New(db *pgxpool.Pool) *Metrics {
	registry := prometheus.NewRegistry()
	m := &Metrics{
		registry: registry,
		requests: prometheus.NewCounterVec(prometheus.CounterOpts{Name: "http_server_requests_total", Help: "Completed HTTP requests."}, []string{"method", "route", "status"}),
		duration: prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "http_server_request_duration_seconds", Help: "HTTP request duration.", Buckets: prometheus.DefBuckets}, []string{"method", "route"}),
		active:   prometheus.NewGauge(prometheus.GaugeOpts{Name: "http_server_active_requests", Help: "Currently active HTTP requests."}),
	}
	registry.MustRegister(collectors.NewGoCollector(), collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}), m.requests, m.duration, m.active)
	m.registerDatabase(db)
	m.registerOutbox(db)
	m.registerAnalytics(db)
	return m
}

func (m *Metrics) registerAnalytics(db *pgxpool.Pool) {
	m.registry.MustRegister(
		prometheus.NewGaugeFunc(prometheus.GaugeOpts{Name: "analytics_backlog_events", Help: "Analytics events waiting for ClickHouse ingestion."}, func() float64 {
			return queryFloat(db, `SELECT count(*)::double precision FROM analytics_events WHERE processed_at IS NULL AND dead_lettered_at IS NULL`)
		}),
		prometheus.NewGaugeFunc(prometheus.GaugeOpts{Name: "analytics_dead_letter_events", Help: "Analytics events that exhausted delivery attempts."}, func() float64 {
			return queryFloat(db, `SELECT count(*)::double precision FROM analytics_events WHERE dead_lettered_at IS NOT NULL`)
		}),
		prometheus.NewGaugeFunc(prometheus.GaugeOpts{Name: "analytics_ingestion_lag_seconds", Help: "Age of the oldest analytics event waiting for ingestion."}, func() float64 {
			return queryFloat(db, `SELECT COALESCE(EXTRACT(EPOCH FROM now() - min(occurred_at)), 0) FROM analytics_events WHERE processed_at IS NULL AND dead_lettered_at IS NULL`)
		}),
	)
}

func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{})
}

func (m *Metrics) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		m.active.Inc()
		defer m.active.Dec()
		wrapped := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(wrapped, r)
		route := chi.RouteContext(r.Context()).RoutePattern()
		if route == "" {
			route = "unmatched"
		}
		status := wrapped.Status()
		if status == 0 {
			status = http.StatusOK
		}
		m.requests.WithLabelValues(r.Method, route, strconv.Itoa(status)).Inc()
		m.duration.WithLabelValues(r.Method, route).Observe(time.Since(started).Seconds())
	})
}

func (m *Metrics) registerDatabase(db *pgxpool.Pool) {
	gauges := map[string]func() float64{
		"postgres_pool_total_connections":    func() float64 { return float64(db.Stat().TotalConns()) },
		"postgres_pool_idle_connections":     func() float64 { return float64(db.Stat().IdleConns()) },
		"postgres_pool_acquired_connections": func() float64 { return float64(db.Stat().AcquiredConns()) },
		"postgres_pool_empty_acquire_count":  func() float64 { return float64(db.Stat().EmptyAcquireCount()) },
	}
	for name, value := range gauges {
		m.registry.MustRegister(prometheus.NewGaugeFunc(prometheus.GaugeOpts{Name: name, Help: "PostgreSQL connection pool metric."}, value))
	}
}

func (m *Metrics) registerOutbox(db *pgxpool.Pool) {
	m.registry.MustRegister(
		prometheus.NewGaugeFunc(prometheus.GaugeOpts{Name: "outbox_backlog_events", Help: "Outbox events waiting for delivery."}, func() float64 {
			return queryFloat(db, `SELECT count(*)::double precision FROM outbox_events WHERE processed_at IS NULL AND dead_lettered_at IS NULL`)
		}),
		prometheus.NewGaugeFunc(prometheus.GaugeOpts{Name: "outbox_dead_letter_events", Help: "Outbox events in dead letter."}, func() float64 {
			return queryFloat(db, `SELECT count(*)::double precision FROM outbox_events WHERE dead_lettered_at IS NOT NULL`)
		}),
		prometheus.NewGaugeFunc(prometheus.GaugeOpts{Name: "outbox_oldest_event_age_seconds", Help: "Age of the oldest deliverable outbox event."}, func() float64 {
			return queryFloat(db, `SELECT COALESCE(EXTRACT(EPOCH FROM now() - min(created_at)), 0) FROM outbox_events WHERE processed_at IS NULL AND dead_lettered_at IS NULL`)
		}),
	)
}

func queryFloat(db *pgxpool.Pool, query string) float64 {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	var value float64
	if err := db.QueryRow(ctx, query).Scan(&value); err != nil {
		return 0
	}
	return value
}
