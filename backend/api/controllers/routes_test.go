package controllers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/sqlite"
)

func TestHealthEndpointsSeparateLivenessAndReadiness(t *testing.T) {
	database, err := gorm.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}

	server := Server{DB: database}
	server.SetupRouter()

	assertStatus(t, server.Router, "/health", http.StatusOK)
	assertStatus(t, server.Router, "/health/live", http.StatusOK)
	assertStatus(t, server.Router, "/health/ready", http.StatusOK)

	if err = database.Close(); err != nil {
		t.Fatalf("close test database: %v", err)
	}
	assertStatus(t, server.Router, "/health", http.StatusOK)
	assertStatus(t, server.Router, "/health/live", http.StatusOK)
	assertStatus(t, server.Router, "/health/ready", http.StatusServiceUnavailable)
}

func assertStatus(t *testing.T, handler http.Handler, path string, expected int) {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, path, nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != expected {
		t.Fatalf("GET %s status = %d, want %d; body = %s", path, response.Code, expected, response.Body.String())
	}
}
