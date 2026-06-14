package main

import (
	"encoding/json"
	"go_js_demo/microvm"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

const (
	USER_SCRIPTS_DIR = "user-scripts"
	RUNTIME_NAME     = "go_js"
	RUNTIME_DIR      = "runtime"
)

type loggerStatusWriter struct {
	http.ResponseWriter
	statusCode int
}

func (lsw *loggerStatusWriter) WriteHeader(code int) {
	lsw.statusCode = code
	lsw.ResponseWriter.WriteHeader(code)
}

type InterpretRequestBody struct {
	Src string `json:"src"`
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

func handlePostInterpret(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	var requestBody InterpretRequestBody
	err := json.NewDecoder(r.Body).Decode(&requestBody)

	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	output := microvm.RunCode(requestBody.Src, r.Context())

	w.Header().Set("Content-Type", "application/json")
	res, err := json.Marshal(output)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Write(res)
}

func handleGetScripts(w http.ResponseWriter, r *http.Request) {
	entries, err := os.ReadDir(filepath.Join(USER_SCRIPTS_DIR))

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	files := []string{}

	for _, entry := range entries {
		files = append(files, entry.Name())
	}

	w.Header().Set("Content-Type", "application/json")
	res, err := json.Marshal(files)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Write(res)
}

const (
	PORT = ":8000"
)

func main() {
	microvm.Init()
	err := os.Mkdir(filepath.Join(USER_SCRIPTS_DIR), 0750)

	if err != nil && !os.IsExist(err) {
		log.Fatal(err)
	}

	mux := http.NewServeMux()
	dir := http.Dir("./static")
	mux.Handle("/", http.FileServer(dir))
	mux.HandleFunc("/api/interpret", handlePostInterpret)
	mux.HandleFunc("/api/user-scripts", handleGetScripts)

	wrap := Logger(mux)
	log.Printf("Listening and serving from localhost%s", PORT)
	http.ListenAndServe(":8000", wrap)
}
