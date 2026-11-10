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

func TestFrontendHandlerInjectsSessionBootstrap(t *testing.T) {
	dir := t.TempDir()
	indexPath := filepath.Join(dir, "index.html")
	if err := os.WriteFile(indexPath, []byte("<!doctype html><html><head></head><body></body></html>"), 0o600); err != nil {
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

	body := recorder.Body.String()
	if !strings.Contains(body, `id="go-js-session"`) {
		t.Fatalf("expected session bootstrap script, got %s", body)
	}
	if !strings.Contains(body, `"session_id":"`) {
		t.Fatalf("expected session id in bootstrap, got %s", body)
	}
	if !strings.Contains(body, `"mcp_path":"/session/`) {
		t.Fatalf("expected MCP path in bootstrap, got %s", body)
	}
	if !strings.Contains(body, "</head>") {
		t.Fatalf("expected head to remain intact, got %s", body)
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
