package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"

	"go_js_demo/mcpserver"
)

const sessionBootstrapElementID = "go-js-session"

type frontendHandler struct {
	manager   *mcpserver.Manager
	static    http.Handler
	indexPath string
}

func newFrontendHandler(manager *mcpserver.Manager, static http.Handler, indexPath string) http.Handler {
	return &frontendHandler{
		manager:   manager,
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

	session := h.manager.CreateSession()
	html = injectSessionBootstrap(html, session)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(html)
}

func injectSessionBootstrap(html []byte, session mcpserver.SessionInfo) []byte {
	payload, err := json.Marshal(session)
	if err != nil {
		return html
	}

	script := []byte(`<script id="` + sessionBootstrapElementID + `" type="application/json">` + string(payload) + `</script>`)
	marker := []byte("</head>")
	if index := bytes.Index(html, marker); index >= 0 {
		result := make([]byte, 0, len(html)+len(script))
		result = append(result, html[:index]...)
		result = append(result, script...)
		result = append(result, html[index:]...)
		return result
	}

	return append(html, script...)
}
