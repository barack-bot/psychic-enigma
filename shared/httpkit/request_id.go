package httpkit

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
)

const RequestIDHeader = "X-Request-ID"

func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get(RequestIDHeader)
		if id == "" {
			buffer := make([]byte, 16)
			_, _ = rand.Read(buffer)
			id = hex.EncodeToString(buffer)
		}
		r.Header.Set(RequestIDHeader, id)
		w.Header().Set(RequestIDHeader, id)
		next.ServeHTTP(w, r)
	})
}
