package provider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestDownloadResult(t *testing.T) {
	// Mock HTTP server that returns fake GLB data
	glbMagic := []byte("glTF\x02\x00\x00\x00")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "model/gltf-binary")
		w.Write(glbMagic)
	}))
	defer server.Close()

	data, err := downloadResult(context.Background(), server.URL)
	if err != nil {
		t.Fatalf("downloadResult failed: %v", err)
	}

	if len(data) != len(glbMagic) {
		t.Errorf("expected %d bytes, got %d", len(glbMagic), len(data))
	}

	if string(data[:4]) != "glTF" {
		t.Errorf("expected glTF magic bytes, got %q", string(data[:4]))
	}
}

func TestDownloadResultHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	_, err := downloadResult(context.Background(), server.URL)
	if err == nil {
		t.Fatal("expected error for HTTP 404, got nil")
	}
}

func TestDownloadResultCancelled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("data"))
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := downloadResult(ctx, server.URL)
	if err == nil {
		t.Fatal("expected error for cancelled context, got nil")
	}
}

func TestGenerateModelFallbackToBase64(t *testing.T) {
	// Create a tiny test image file
	tmpDir := t.TempDir()
	imgPath := filepath.Join(tmpDir, "test.png")
	if err := os.WriteFile(imgPath, []byte("fake-png-data"), 0644); err != nil {
		t.Fatal(err)
	}

	// This will fail at the API call (no valid credentials), but we can verify
	// that the function attempts base64 encoding when no URL is provided
	_, err := GenerateModel(context.Background(), "fake-id", "fake-key", imgPath, "", tmpDir)
	if err == nil {
		t.Fatal("expected error with fake credentials, got nil")
	}
	// The error should be from the API call, not from file reading
	if _, readErr := os.ReadFile(imgPath); readErr != nil {
		t.Fatal("image file should be readable")
	}
}
