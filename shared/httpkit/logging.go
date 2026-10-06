package httpkit

import (
	"log"
	"net/http"
	"time"
)

func Logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("request_id=%s method=%s path=%s duration=%s", r.Header.Get(RequestIDHeader), r.Method, r.URL.Path, time.Since(started))
	})
}
