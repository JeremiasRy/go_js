package main

import (
	"go_js_demo/mcpserver"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestFrontendHandlerServesIndexWithoutSessionBootstrap(t *testing.T) {
	dir := t.TempDir()
	indexPath := filepath.Join(dir, "index.html")
	index := "<!doctype html><html><head></head><body></body></html>"
	if err := os.WriteFile(indexPath, []byte(index), 0o600); err != nil {
		t.Fatal(err)
	}

	manager := mcpserver.NewManager(nil, nil, time.Minute)
	t.Cleanup(manager.Close)

	handler := newFrontendHandler(manager, http.FileServer(http.Dir(dir)), indexPath)
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", recorder.Code)
	}
	if got := recorder.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("expected no-store cache control, got %q", got)
	}
	if body := recorder.Body.String(); body != index {
		t.Fatalf("expected index to be served unchanged, got %s", body)
	}
	if strings.Contains(recorder.Body.String(), "go-js-session") {
		t.Fatal("page loads must not create or inject a server-side session")
	}
}

func TestFrontendHandlerServesStaticAssets(t *testing.T) {
	dir := t.TempDir()
	indexPath := filepath.Join(dir, "index.html")
	if err := os.WriteFile(indexPath, []byte("<html></html>"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "app.js"), []byte("console.log(1)"), 0o600); err != nil {
		t.Fatal(err)
	}

	manager := mcpserver.NewManager(nil, nil, time.Minute)
	t.Cleanup(manager.Close)

	handler := newFrontendHandler(manager, http.FileServer(http.Dir(dir)), indexPath)
	request := httptest.NewRequest(http.MethodGet, "/app.js", nil)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", recorder.Code)
	}
	if strings.Contains(recorder.Body.String(), "go-js-session") {
		t.Fatal("static assets must not receive the session bootstrap")
	}
}
