package main

import (
	"net/http"
	"os"

	"go_js_demo/mcpserver"
)

type frontendHandler struct {
	static    http.Handler
	indexPath string
}

func newFrontendHandler(_ *mcpserver.Manager, static http.Handler, indexPath string) http.Handler {
	return &frontendHandler{
		static:    static,
		indexPath: indexPath,
	}
}

func (h *frontendHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet || (r.URL.Path != "/" && r.URL.Path != "/index.html") {
		h.static.ServeHTTP(w, r)
		return
	}

	html, err := os.ReadFile(h.indexPath)
	if err != nil {
		h.static.ServeHTTP(w, r)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(html)
}
