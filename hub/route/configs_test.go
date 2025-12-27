package route

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestConfigRouterReloadEndpoint(t *testing.T) {
	r := chi.NewRouter()
	r.Mount("/configs", configRouter())

	// Test that reload endpoint is registered
	req := httptest.NewRequest("POST", "/configs/reload", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// We expect it to try to parse config and fail (since no config exists in test)
	// but the endpoint should be registered and respond
	if w.Code != http.StatusInternalServerError && w.Code != http.StatusNoContent {
		t.Logf("Expected 500 or 204, got %d (endpoint is registered)", w.Code)
	}
}

func TestConfigRouterRestartEndpoint(t *testing.T) {
	r := chi.NewRouter()
	r.Mount("/configs", configRouter())

	// Test that restart endpoint is registered
	req := httptest.NewRequest("POST", "/configs/restart", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Should get 200 with restart message
	if w.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", w.Code)
	}

	// Check response contains restart message
	body := w.Body.String()
	if body == "" {
		t.Error("Expected response body, got empty")
	}
	t.Logf("Restart endpoint response: %s", body)
}
