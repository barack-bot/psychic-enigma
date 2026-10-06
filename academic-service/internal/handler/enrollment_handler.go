package handler

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"school-platform/academic-service/internal/domain"
	"school-platform/academic-service/internal/service"
)

type EnrollmentHandler struct{ svc *service.EnrollmentService }

type enrollmentInput struct {
	ID              int64  `json:"id"`
	StudentID       int64  `json:"student_id"`
	AcademicTermID  int64  `json:"academic_term_id"`
	ClassOfferingID *int64 `json:"class_offering_id"`
	Status          string `json:"status"`
	Reason          string `json:"reason"`
}

func NewEnrollmentHandler(svc *service.EnrollmentService) *EnrollmentHandler {
	return &EnrollmentHandler{svc: svc}
}

func (h *EnrollmentHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("/api/enrollments/student", h.listForStudent)
	mux.HandleFunc("/api/enrollments/enroll", h.enroll)
	mux.HandleFunc("/api/enrollments/defer", h.deferEnrollment)
	mux.HandleFunc("/api/enrollments/withdraw", h.withdraw)
}

func (h *EnrollmentHandler) listForStudent(w http.ResponseWriter, r *http.Request) {
	studentID, err := strconv.ParseInt(r.URL.Query().Get("student_id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid student_id", http.StatusBadRequest)
		return
	}
	items, err := h.svc.ListForStudent(r.Context(), studentID)
	if err != nil {
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(items)
}

func (h *EnrollmentHandler) enroll(w http.ResponseWriter, r *http.Request) {
	var in enrollmentInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	id, err := h.svc.Enroll(r.Context(), domain.Enrollment{StudentID: in.StudentID, AcademicTermID: in.AcademicTermID, ClassOfferingID: in.ClassOfferingID, Status: in.Status})
	if err != nil {
		log.Printf("enroll failed: %v", err); http.Error(w, "enrollment could not be saved", http.StatusConflict)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": id})
}

func (h *EnrollmentHandler) deferEnrollment(w http.ResponseWriter, r *http.Request) {
	var in enrollmentInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	if err := h.svc.Defer(r.Context(), in.ID, in.Reason); err != nil {
		log.Printf("defer enrollment failed: %v", err); http.Error(w, "enrollment could not be deferred", http.StatusConflict)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{"status": "deferred"})
}

func (h *EnrollmentHandler) withdraw(w http.ResponseWriter, r *http.Request) {
	var in enrollmentInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	if err := h.svc.Withdraw(r.Context(), in.ID); err != nil {
		log.Printf("withdraw enrollment failed: %v", err); http.Error(w, "enrollment could not be withdrawn", http.StatusConflict)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{"status": "withdrawn"})
}