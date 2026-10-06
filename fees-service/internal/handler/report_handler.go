package handler

import (
	"encoding/json"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
)

type ReportHandler struct{ db *pgxpool.Pool }

func NewReportHandler(db *pgxpool.Pool) *ReportHandler { return &ReportHandler{db: db} }

func (h *ReportHandler) Balance(w http.ResponseWriter, r *http.Request) {
	studentID, err := parseTrailingID(r.URL.Path, "/api/reports/balance/")
	if err != nil {
		http.Error(w, "invalid student id", http.StatusBadRequest)
		return
	}
	if authErr := requireStudentOrStaff(r, h.db, studentID); authErr != nil {
		http.Error(w, authErr.Error(), authErr.status)
		return
	}
	var balance struct {
		StudentID int64   `json:"student_id"`
		Charges   float64 `json:"total_charges"`
		Paid      float64 `json:"total_paid"`
		Balance   float64 `json:"balance"`
	}
	if err := h.db.QueryRow(r.Context(), `SELECT student_id, total_charges, total_paid, balance FROM student_balances WHERE student_id = $1`, studentID).Scan(&balance.StudentID, &balance.Charges, &balance.Paid, &balance.Balance); err != nil {
		http.Error(w, "student balance not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(balance)
}

func (h *ReportHandler) Statement(w http.ResponseWriter, r *http.Request) {
	studentID, err := parseTrailingID(r.URL.Path, "/api/reports/statement/")
	if err != nil {
		http.Error(w, "invalid student id", http.StatusBadRequest)
		return
	}
	if authErr := requireStudentOrStaff(r, h.db, studentID); authErr != nil {
		http.Error(w, authErr.Error(), authErr.status)
		return
	}
	rows, err := h.db.Query(r.Context(), `SELECT student_id, academic_term_id, year_label, term_name, term_number, enrollment_status, class_name, charge_status, charged, waived, paid, balance FROM student_term_statement WHERE student_id = $1 ORDER BY academic_term_id`, studentID)
	if err != nil {
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	var statements []map[string]any
	for rows.Next() {
		var studentID int64
		var academicTermID *int64
		var yearLabel, termName, enrollmentStatus, chargeStatus string
		var className *string
		var termNumber int
		var charged, waived, paid, balance float64
		if err := rows.Scan(&studentID, &academicTermID, &yearLabel, &termName, &termNumber, &enrollmentStatus, &className, &chargeStatus, &charged, &waived, &paid, &balance); err != nil {
			http.Error(w, "database error", http.StatusInternalServerError)
			return
		}
		statements = append(statements, map[string]any{"student_id": studentID, "academic_term_id": academicTermID, "year_label": yearLabel, "term_name": termName, "term_number": termNumber, "enrollment_status": enrollmentStatus, "class_name": className, "charge_status": chargeStatus, "charged": charged, "waived": waived, "paid": paid, "balance": balance})
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(statements)
}

func (h *ReportHandler) Marks(w http.ResponseWriter, r *http.Request) {
	studentID, err := parseTrailingID(r.URL.Path, "/api/reports/marks/")
	if err != nil { http.Error(w, "invalid student id", 400); return }
	if authErr := requireStudentOrStaff(r, h.db, studentID); authErr != nil { http.Error(w, authErr.Error(), authErr.status); return }
	rows, err := h.db.Query(r.Context(), `SELECT m.id, a.name, s.name, m.score, a.max_score, m.status FROM marks m JOIN assessments a ON a.id = m.assessment_id JOIN class_subjects cs ON cs.id = a.class_subject_id JOIN subjects s ON s.id = cs.subject_id WHERE m.student_id = $1 AND m.status = 'APPROVED' ORDER BY s.name, a.id`, studentID)
	if err != nil { http.Error(w, "database error", 500); return }
	defer rows.Close()
	var result []map[string]any
	for rows.Next() { var id int64; var assessment, subject, status string; var score, maxScore float64; if err := rows.Scan(&id, &assessment, &subject, &score, &maxScore, &status); err != nil { http.Error(w, "database error", 500); return }; result = append(result, map[string]any{"id": id, "assessment": assessment, "subject": subject, "score": score, "max_score": maxScore, "status": status}) }
	w.Header().Set("Content-Type", "application/json"); _ = json.NewEncoder(w).Encode(result)
}
