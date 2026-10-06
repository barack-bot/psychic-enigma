package httpkit

import (
	"encoding/json"
	"net/http"
)

func JSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func Error(w http.ResponseWriter, status int, err error) {
	message := http.StatusText(status)
	if err != nil {
		message = err.Error()
	}
	JSON(w, status, map[string]string{"error": message})
}
