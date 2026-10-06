package main

import (
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func securityHeaders(next http.Handler) http.Handler { return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Header().Set("X-Content-Type-Options", "nosniff"); w.Header().Set("X-Frame-Options", "DENY"); w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin"); w.Header().Set("Content-Security-Policy", "default-src 'self'; img-src 'self' data: https://images.unsplash.com; style-src 'self' 'unsafe-inline' https://fonts.googleapis.com https://fonts.gstatic.com; font-src 'self' data: https://fonts.gstatic.com https://fonts.googleapis.com; script-src 'self' 'unsafe-inline'; connect-src 'self' http://localhost:8080 http://localhost:8081 http://localhost:8082"); next.ServeHTTP(w, r) }) }
func limitBody(next http.Handler) http.Handler { return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { r.Body = http.MaxBytesReader(w, r.Body, 1<<20); next.ServeHTTP(w, r) }) }

func requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &responseWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rw, r)
		log.Printf("%s %s -> %d in %s", r.Method, r.URL.Path, rw.status, time.Since(start))
	})
}

type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

func main() {
	port := os.Getenv("SCHOOL_PORT")
	if port == "" {
		port = "5500"
	}
	root, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	fileServer := requestLogger(http.FileServer(http.Dir(filepath.Clean(root))))
	intakeURL := os.Getenv("INTAKE_URL")
	if intakeURL == "" {
		intakeURL = "http://intake:8080"
	}
	intakeTarget, err := url.Parse(intakeURL)
	if err != nil {
		log.Fatal(err)
	}
	intakeProxy := requestLogger(httputil.NewSingleHostReverseProxy(intakeTarget))
	academicURL := os.Getenv("ACADEMIC_URL")
	if academicURL == "" { academicURL = "http://academic:8082" }
	academicTarget, err := url.Parse(academicURL)
	if err != nil { log.Fatal(err) }
	academicProxy := requestLogger(httputil.NewSingleHostReverseProxy(academicTarget))
	feesURL := os.Getenv("FEES_URL")
	if feesURL == "" { feesURL = "http://fees:8081" }
	feesTarget, err := url.Parse(feesURL)
	if err != nil { log.Fatal(err) }
	feesProxy := requestLogger(httputil.NewSingleHostReverseProxy(feesTarget))
	mux := http.NewServeMux()
	mux.Handle("/intake-form.html", intakeProxy)
	mux.Handle("/students.html", intakeProxy)
	mux.Handle("/static/", intakeProxy)
	mux.Handle("/api/", intakeProxy)
	mux.Handle("/portal/", http.StripPrefix("/portal", feesProxy))
	mux.Handle("/admin/", feesProxy)
	mux.Handle("/admin", feesProxy)
	mux.Handle("/assets/", feesProxy)
	mux.Handle("/staff/", academicProxy)
	mux.Handle("/staff", academicProxy)
	mux.Handle("/fees-api/", http.StripPrefix("/fees-api", feesProxy))
	mux.Handle("/academic-api/", http.StripPrefix("/academic-api", academicProxy))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/intake") {
			intakeProxy.ServeHTTP(w, r)
			return
		}
		fileServer.ServeHTTP(w, r)
	})
	server := http.Server{
		Addr:    ":" + port,
		Handler: securityHeaders(limitBody(mux)),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout: 15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout: 60 * time.Second,
	}
	log.Printf("school website listening on http://localhost:%s", port)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Printf("school website failed to serve: %v", err)
	}
}
