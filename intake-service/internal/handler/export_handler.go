package handler

import (
	"encoding/csv"
	"database/sql"
	"net/http"
	"strconv"
)

// ExportHandler exports student and application data as CSV.
type ExportHandler struct{ db *sql.DB }

func NewExportHandler(db *sql.DB) *ExportHandler { return &ExportHandler{db: db} }

func (h *ExportHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("/api/export/students.csv", h.exportStudents)
	mux.HandleFunc("/api/export/applications.csv", h.exportApplications)
}

func (h *ExportHandler) exportStudents(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", "attachment; filename=students.csv")
	writer := csv.NewWriter(w); _ = writer.Write([]string{"student_id", "name", "admission_number", "status"})
	rows, err := h.db.QueryContext(r.Context(), `SELECT student_id,name,COALESCE(admission_number,''),status FROM students ORDER BY student_id`); if err == nil { defer rows.Close(); for rows.Next() { var id int64; var name, admission, status string; if rows.Scan(&id, &name, &admission, &status) == nil { _ = writer.Write([]string{strconv.FormatInt(id, 10), name, admission, status}) } } }; writer.Flush()
}

func (h *ExportHandler) exportApplications(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", "attachment; filename=applications.csv")
	writer := csv.NewWriter(w)
	_ = writer.Write([]string{"id", "full_name", "email", "course", "status"})
	rows, err := h.db.QueryContext(r.Context(), `SELECT id,full_name,email,course,status FROM applications ORDER BY created_at DESC`); if err == nil { defer rows.Close(); for rows.Next() { var id int64; var name, email, course, status string; if rows.Scan(&id, &name, &email, &course, &status) == nil { _ = writer.Write([]string{strconv.FormatInt(id, 10), name, email, course, status}) } } }
	writer.Flush()
}