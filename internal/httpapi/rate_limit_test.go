package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRateLimiter(t *testing.T) {
	t.Parallel()
	now := time.Unix(1_800_000_000, 0)
	limiter := newRateLimiter()
	limiter.now = func() time.Time { return now }
	handler := limiter.middleware("login", 2, time.Minute)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	for index, want := range []int{http.StatusNoContent, http.StatusNoContent, http.StatusTooManyRequests} {
		request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
		request.RemoteAddr = "192.0.2.10:1234"
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != want {
			t.Fatalf("request %d returned %d, want %d", index+1, response.Code, want)
		}
	}

	now = now.Add(time.Minute)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
	request.RemoteAddr = "192.0.2.10:5678"
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("request after reset returned %d", response.Code)
	}
}

func TestCORSAllowlistAndSecurityHeaders(t *testing.T) {
	t.Parallel()
	handler := securityHeaders(cors([]string{"https://app.example.test"})(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})))

	allowed := httptest.NewRequest(http.MethodOptions, "/api/v1/orders", nil)
	allowed.Header.Set("Origin", "https://app.example.test")
	allowedResponse := httptest.NewRecorder()
	handler.ServeHTTP(allowedResponse, allowed)
	if allowedResponse.Code != http.StatusNoContent || allowedResponse.Header().Get("Access-Control-Allow-Origin") != "https://app.example.test" ||
		allowedResponse.Header().Get("Access-Control-Allow-Credentials") != "true" {
		t.Fatalf("allowed preflight was not configured correctly: code=%d headers=%v", allowedResponse.Code, allowedResponse.Header())
	}
	if allowedResponse.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatal("security headers were not applied")
	}

	denied := httptest.NewRequest(http.MethodOptions, "/api/v1/orders", nil)
	denied.Header.Set("Origin", "https://evil.example.test")
	deniedResponse := httptest.NewRecorder()
	handler.ServeHTTP(deniedResponse, denied)
	if deniedResponse.Code != http.StatusForbidden || deniedResponse.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatalf("untrusted origin was not rejected: code=%d headers=%v", deniedResponse.Code, deniedResponse.Header())
	}
}
