package controllers

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestGetPortraitURLWithClient(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"composers":[{"name":"Bach","complete_name":"Johann Sebastian Bach","safe_name":"johann-sebastian-bach","epoch":"Baroque","portrait":"https://example.test/bach.jpg"}]}`))
	}))
	defer server.Close()

	client := &http.Client{Timeout: time.Second}
	composer := getPortraitURLWithClient(client, server.URL, "Bach")

	if composer.CompleteName != "Johann Sebastian Bach" {
		t.Fatalf("CompleteName = %q", composer.CompleteName)
	}
	if composer.SafeName != "johann-sebastian-bach" {
		t.Fatalf("SafeName = %q", composer.SafeName)
	}
}

func TestGetPortraitURLWithClientFallsBackOnFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		http.Error(writer, "unavailable", http.StatusServiceUnavailable)
	}))
	defer server.Close()

	client := &http.Client{Timeout: time.Second}
	composer := getPortraitURLWithClient(client, server.URL, "Test Composer")

	if composer.CompleteName != "Test Composer" {
		t.Fatalf("CompleteName = %q", composer.CompleteName)
	}
	if composer.Epoch != "Unknown" {
		t.Fatalf("Epoch = %q", composer.Epoch)
	}
	if composer.Portrait != unknownPortraitURL {
		t.Fatalf("Portrait = %q", composer.Portrait)
	}
}
