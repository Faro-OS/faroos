package webui

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandlerFallsBackToSPAIndex(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/login", nil)
	response := httptest.NewRecorder()

	Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("GET /login status = %d, want %d", response.Code, http.StatusOK)
	}
	if !strings.Contains(response.Body.String(), "<!doctype html>") {
		t.Fatal("GET /login did not serve the SPA index")
	}
}

func TestHandlerDoesNotMaskMissingAPI(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/api/does-not-exist", nil)
	response := httptest.NewRecorder()

	Handler().ServeHTTP(response, request)

	if response.Code != http.StatusNotFound {
		t.Fatalf("GET missing API status = %d, want %d", response.Code, http.StatusNotFound)
	}
}

func TestHandlerDoesNotCacheAppShellOrVersion(t *testing.T) {
	tests := []string{"/", "/login", "/_app/version.json"}
	for _, requestPath := range tests {
		t.Run(requestPath, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, requestPath, nil)
			response := httptest.NewRecorder()

			Handler().ServeHTTP(response, request)

			if response.Code != http.StatusOK {
				t.Fatalf("GET %s status = %d, want %d", requestPath, response.Code, http.StatusOK)
			}
			if got := response.Header().Get("Cache-Control"); got != "no-cache, no-store, must-revalidate" {
				t.Fatalf("GET %s Cache-Control = %q", requestPath, got)
			}
		})
	}
}
