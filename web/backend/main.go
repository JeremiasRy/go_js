package main

import (
	"bufio"
	"context"
	"go_js_demo/db"
	"go_js_demo/mcpserver"
	"go_js_demo/microvm"
	"go_js_demo/worker"
	"log"
	"net"
	"net/http"
	"regexp"
	"time"
)

const (
	RUNTIME_NAME = "go_js"
	RUNTIME_DIR  = "runtime"
)

type loggerStatusWriter struct {
	http.ResponseWriter
	statusCode int
}

func (lsw *loggerStatusWriter) WriteHeader(code int) {
	lsw.statusCode = code
	lsw.ResponseWriter.WriteHeader(code)
}

func (lsw *loggerStatusWriter) Unwrap() http.ResponseWriter {
	return lsw.ResponseWriter
}

func (lsw *loggerStatusWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return http.NewResponseController(lsw.ResponseWriter).Hijack()
}

func (lsw *loggerStatusWriter) Flush() {
	_ = http.NewResponseController(lsw.ResponseWriter).Flush()
}

var sessionPathPattern = regexp.MustCompile(`/(session|ws)/[0-9a-fA-F-]{32,36}`)

func redactSessionPath(path string) string {
	return sessionPathPattern.ReplaceAllString(path, "/$1/{session}")
}

func Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		lrs := &loggerStatusWriter{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
		}
		log.Printf("Incoming request: Method=%s, Path=%s, RemoteAddr=%s, User-Agent=%s",
			r.Method, redactSessionPath(r.URL.Path), r.RemoteAddr, r.Header.Get("User-Agent"))
		next.ServeHTTP(lrs, r)
		log.Printf("%d Done in %.2fms", lrs.statusCode, float64(time.Since(start).Microseconds())/1000)
	})
}

const (
	PORT         = ":8000"
	WORKER_COUNT = 10
	QUEUE_MAX    = 100
)

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, Accept, Mcp-Session-Id, MCP-Protocol-Version, Last-Event-ID")
		w.Header().Set("Access-Control-Expose-Headers", "Mcp-Session-Id")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func main() {
	database := db.NewDatabase()
	mvm := microvm.NewMicroVMHandler()
	sig := make(chan worker.HeartbeatSignal, QUEUE_MAX)
	results := make(chan worker.JobResult, QUEUE_MAX)
	mgr := mcpserver.NewManager(database, sig, mcpserver.DefaultSessionTTL)
	defer mgr.Close()

	ctx := context.Background()
	go func() {
		for result := range results {
			mgr.DeliverJobResult(result.JobID, result.Status, result.Result)
		}
	}()
	go worker.SpinUpWorkers(database, mvm, sig, results, WORKER_COUNT, ctx)

	mux := http.NewServeMux()
	dir := http.Dir("./static")
	static := http.FileServer(dir)

	mux.Handle("/", newFrontendHandler(mgr, static, "./static/index.html"))
	mux.HandleFunc("POST /session", mgr.HandleCreateSession)
	mux.HandleFunc("GET /ws/{session_id}", mgr.HandleWS)
	mux.HandleFunc("/session/{session_id}/mcp", mgr.HandleMCP)

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Ok!"))
	})

	wrap := corsMiddleware(Logger(mux))
	log.Printf("Listening and serving from localhost%s", PORT)
	err := http.ListenAndServe(PORT, wrap)

	if err != nil {
		log.Fatalf("Something went horribly wrong %s", err.Error())
	}
}
