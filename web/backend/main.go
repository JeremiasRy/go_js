package main

import (
	"context"
	"go_js_demo/controller"
	"go_js_demo/db"
	"go_js_demo/microvm"
	"go_js_demo/worker"
	"log"
	"net/http"
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

func Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		lrs := &loggerStatusWriter{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
		}
		log.Printf("// %s %s", r.Method, r.URL)
		next.ServeHTTP(lrs, r)
		log.Printf("%d Done in %.2fms", lrs.statusCode, float64(time.Since(start).Microseconds())/1000)
	})
}

const (
	PORT         = ":8000"
	WORKER_COUNT = 3
)

func main() {
	db := db.NewDatabase()
	mvm := microvm.NewMicroVMHandler()
	sig := make(chan worker.NewQuerySignal)
	ctr := controller.NewUserSriptsController(db, sig)
	ctx := context.Background()
	go worker.SpinUpWorkers(db, mvm, sig, WORKER_COUNT, ctx)
	go func() {
		tick := time.NewTicker(time.Second)
		for range tick.C {
			log.Println("Heartbeat...")
			sig <- worker.NewQuerySignal{}
		}
	}()

	mux := http.NewServeMux()
	dir := http.Dir("./static")

	mux.Handle("/", http.FileServer(dir))
	mux.HandleFunc("/api/interpret", ctr.HandlePostInterpret)
	mux.HandleFunc("/api/jobs/{job_id}", ctr.HandleGetJobById)

	wrap := Logger(mux)
	log.Printf("Listening and serving from localhost%s", PORT)
	err := http.ListenAndServe(PORT, wrap)

	if err != nil {
		log.Fatalf("Something went horribly wrong %s", err.Error())
	}
}
