package utils

import (
	"bytes"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	. "github.com/SheetAble/SheetAble/backend/api/config"
)

const pdfToPNGRequestTimeout = 30 * time.Second

// RequestToPdfToImage sends a PDF to the configured thumbnail service and
// atomically stores the returned PNG in the application data directory.
func RequestToPdfToImage(pdfPath string, name string) error {
	config := Config()
	client := &http.Client{Timeout: pdfToPNGRequestTimeout}

	return requestToPDFToImage(client, config.PDF2PNGURL, config.ConfigPath, pdfPath, name)
}

func requestToPDFToImage(client *http.Client, remoteURL string, configPath string, pdfPath string, name string) error {
	if client == nil {
		return fmt.Errorf("pdf2png HTTP client is nil")
	}

	parsedURL, err := url.ParseRequestURI(remoteURL)
	if err != nil || parsedURL.Host == "" || (parsedURL.Scheme != "http" && parsedURL.Scheme != "https") {
		return fmt.Errorf("invalid PDF2PNG_URL %q", remoteURL)
	}

	pdf, err := os.Open(pdfPath)
	if err != nil {
		return fmt.Errorf("open PDF for thumbnail generation: %w", err)
	}
	defer pdf.Close()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	filePart, err := writer.CreateFormFile("file", filepath.Base(pdfPath))
	if err != nil {
		return fmt.Errorf("create PDF multipart field: %w", err)
	}
	if _, err = io.Copy(filePart, pdf); err != nil {
		return fmt.Errorf("copy PDF into multipart request: %w", err)
	}
	if err = writer.WriteField("name", name); err != nil {
		return fmt.Errorf("create thumbnail name field: %w", err)
	}
	if err = writer.Close(); err != nil {
		return fmt.Errorf("close PDF multipart request: %w", err)
	}

	request, err := http.NewRequest(http.MethodPost, remoteURL, &body)
	if err != nil {
		return fmt.Errorf("create pdf2png request: %w", err)
	}
	request.Header.Set("Content-Type", writer.FormDataContentType())

	response, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("request thumbnail from pdf2png: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		message, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		return fmt.Errorf("pdf2png returned %s: %s", response.Status, strings.TrimSpace(string(message)))
	}

	mediaType, _, err := mime.ParseMediaType(response.Header.Get("Content-Type"))
	if err != nil || mediaType != "image/png" {
		return fmt.Errorf("pdf2png returned unexpected content type %q", response.Header.Get("Content-Type"))
	}

	thumbnailDir := filepath.Join(configPath, "sheets", "thumbnails")
	if err = os.MkdirAll(thumbnailDir, 0755); err != nil {
		return fmt.Errorf("create thumbnail directory: %w", err)
	}

	temporaryFile, err := os.CreateTemp(thumbnailDir, ".thumbnail-*.png")
	if err != nil {
		return fmt.Errorf("create temporary thumbnail: %w", err)
	}
	temporaryPath := temporaryFile.Name()
	published := false
	defer func() {
		_ = temporaryFile.Close()
		if !published {
			_ = os.Remove(temporaryPath)
		}
	}()

	if _, err = io.Copy(temporaryFile, response.Body); err != nil {
		return fmt.Errorf("save thumbnail response: %w", err)
	}
	if err = temporaryFile.Close(); err != nil {
		return fmt.Errorf("close temporary thumbnail: %w", err)
	}

	thumbnailPath := filepath.Join(thumbnailDir, name+".png")
	if err = os.Rename(temporaryPath, thumbnailPath); err != nil {
		return fmt.Errorf("publish thumbnail: %w", err)
	}
	published = true

	return nil
}
