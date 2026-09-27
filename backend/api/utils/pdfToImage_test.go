package utils

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

var testPNG = []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a}

func TestRequestToPDFToImage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", request.Method)
		}
		if err := request.ParseMultipartForm(1 << 20); err != nil {
			t.Fatalf("parse multipart form: %v", err)
		}
		if got := request.FormValue("name"); got != "test-sheet" {
			t.Errorf("name = %q, want test-sheet", got)
		}

		file, _, err := request.FormFile("file")
		if err != nil {
			t.Fatalf("read PDF form field: %v", err)
		}
		defer file.Close()
		contents, err := io.ReadAll(file)
		if err != nil {
			t.Fatalf("read uploaded PDF: %v", err)
		}
		if !bytes.Equal(contents, []byte("%PDF-test")) {
			t.Errorf("uploaded PDF = %q", contents)
		}

		writer.Header().Set("Content-Type", "image/png")
		writer.WriteHeader(http.StatusOK)
		_, _ = writer.Write(testPNG)
	}))
	defer server.Close()

	dataDir := t.TempDir()
	pdfPath := filepath.Join(t.TempDir(), "input.pdf")
	if err := os.WriteFile(pdfPath, []byte("%PDF-test"), 0600); err != nil {
		t.Fatal(err)
	}

	client := &http.Client{Timeout: time.Second}
	if err := requestToPDFToImage(client, server.URL, dataDir, pdfPath, "test-sheet"); err != nil {
		t.Fatalf("requestToPDFToImage returned error: %v", err)
	}

	thumbnail, err := os.ReadFile(filepath.Join(dataDir, "sheets", "thumbnails", "test-sheet.png"))
	if err != nil {
		t.Fatalf("read generated thumbnail: %v", err)
	}
	if !bytes.Equal(thumbnail, testPNG) {
		t.Errorf("thumbnail = %v, want %v", thumbnail, testPNG)
	}
}

func TestRequestToPDFToImageRejectsFailureResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		http.Error(writer, "conversion failed", http.StatusUnprocessableEntity)
	}))
	defer server.Close()

	pdfPath := filepath.Join(t.TempDir(), "input.pdf")
	if err := os.WriteFile(pdfPath, []byte("%PDF-test"), 0600); err != nil {
		t.Fatal(err)
	}

	err := requestToPDFToImage(&http.Client{Timeout: time.Second}, server.URL, t.TempDir(), pdfPath, "test-sheet")
	if err == nil {
		t.Fatal("requestToPDFToImage returned nil error for a failed response")
	}
}

func TestRequestToPDFToImageRejectsInvalidURL(t *testing.T) {
	err := requestToPDFToImage(&http.Client{Timeout: time.Second}, "pdf2png:5000", t.TempDir(), "missing.pdf", "test-sheet")
	if err == nil {
		t.Fatal("requestToPDFToImage returned nil error for an invalid URL")
	}
}

func TestRequestToPDFToImageVerifiesTLSCertificates(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "image/png")
		_, _ = writer.Write(testPNG)
	}))
	defer server.Close()

	pdfPath := filepath.Join(t.TempDir(), "input.pdf")
	if err := os.WriteFile(pdfPath, []byte("%PDF-test"), 0600); err != nil {
		t.Fatal(err)
	}

	err := requestToPDFToImage(&http.Client{Timeout: time.Second}, server.URL, t.TempDir(), pdfPath, "test-sheet")
	if err == nil {
		t.Fatal("requestToPDFToImage accepted an untrusted TLS certificate")
	}
}
