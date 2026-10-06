package handler

import (
	"net/http"

	"school-platform/fees-service/internal/service"
)

type FeesReconciler struct {
	projection *service.StudentProjection
	intakeURL  string
	client     *http.Client
}

func NewFeesReconciler(projection *service.StudentProjection, intakeURL string) *FeesReconciler {
	return &FeesReconciler{projection: projection, intakeURL: intakeURL, client: http.DefaultClient}
}

func (h *FeesReconciler) Reconcile(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "reconciliation is unavailable: students are stored canonically", http.StatusGone)
	return
	/*
	request, err := http.NewRequestWithContext(r.Context(), http.MethodGet, h.intakeURL+"/api/students?include_deleted=true", nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if token := r.Header.Get("Authorization"); token != "" {
		request.Header.Set("Authorization", token)
	}
	response, err := h.client.Do(request)
	if err != nil {
		http.Error(w, "intake unavailable", http.StatusBadGateway)
		return
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		http.Error(w, "intake returned an error", http.StatusBadGateway)
		return
	}
	var students []events.Student
	if err := json.NewDecoder(response.Body).Decode(&students); err != nil {
		http.Error(w, "invalid intake response", http.StatusBadGateway)
		return
	}
	drift, err := h.projection.Reconcile(r.Context(), students)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	for _, item := range drift {
		log.Printf("fees student reconciliation drift student=%d fields=%v", item.StudentID, item.Fields)
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"drift": drift, "count": len(drift), "applied": false})
	*/
}
