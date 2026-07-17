package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHealthHandler_returnsOKForKubernetesProbe(t *testing.T) {
	// Given
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	response := httptest.NewRecorder()

	// When
	healthHandler().ServeHTTP(response, request)

	// Then
	if response.Code != http.StatusOK {
		t.Fatalf("expected health probe status %d, got %d", http.StatusOK, response.Code)
	}

	body := strings.TrimSpace(response.Body.String())
	if body != "ok" {
		t.Fatalf("expected health probe body %q, got %q", "ok", body)
	}
}
