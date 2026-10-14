package observability

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus"
)

func TestMiddlewareUsesRouteTemplateAndStatus(t *testing.T) {
	registry := prometheus.NewRegistry()
	metrics := &Metrics{
		registry: registry,
		requests: prometheus.NewCounterVec(prometheus.CounterOpts{Name: "http_server_requests_total", Help: "Completed HTTP requests."}, []string{"method", "route", "status"}),
		duration: prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "http_server_request_duration_seconds", Help: "HTTP request duration."}, []string{"method", "route"}),
		active:   prometheus.NewGauge(prometheus.GaugeOpts{Name: "http_server_active_requests", Help: "Currently active HTTP requests."}),
	}
	registry.MustRegister(metrics.requests, metrics.duration, metrics.active)

	router := chi.NewRouter()
	router.Use(metrics.Middleware)
	router.Get("/orders/{orderID}", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/orders/6b6732f2-8f42-4bbb-802d-7f92c30fc090", nil))

	families, err := registry.Gather()
	if err != nil {
		t.Fatalf("gather metrics: %v", err)
	}
	for _, family := range families {
		if family.GetName() != "http_server_requests_total" {
			continue
		}
		for _, metric := range family.GetMetric() {
			labels := map[string]string{}
			for _, label := range metric.GetLabel() {
				labels[label.GetName()] = label.GetValue()
			}
			if labels["method"] == http.MethodGet && labels["route"] == "/orders/{orderID}" && labels["status"] == "204" && metric.GetCounter().GetValue() == 1 {
				return
			}
		}
	}
	t.Fatal("expected counter with normalized route, status 204 and value 1")
}
