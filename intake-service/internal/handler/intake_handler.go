package handler

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

type IntakeHandler struct{ db *sql.DB }

type ApplicationInput struct {
	FullName       string `json:"full_name"`
	Email          string `json:"email"`
	Phone          string `json:"phone"`
	DateOfBirth    string `json:"date_of_birth"`
	Course         string `json:"course"`
	PreviousSchool string `json:"previous_school"`
	Message        string `json:"message"`
}

func NewIntakeHandler(db *sql.DB) *IntakeHandler { return &IntakeHandler{db: db} }

func (h *IntakeHandler) CreateApplication(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var input ApplicationInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(input.FullName) == "" || strings.TrimSpace(input.Email) == "" || strings.TrimSpace(input.Phone) == "" || strings.TrimSpace(input.Course) == "" {
		http.Error(w, "full_name, email, phone and course are required", http.StatusBadRequest)
		return
	}
	var id int64
	err := h.db.QueryRowContext(r.Context(), `INSERT INTO applications (full_name, email, phone, date_of_birth, course, previous_school, message) VALUES ($1,$2,$3,NULLIF($4,'')::date,$5,NULLIF($6,''),NULLIF($7,'')) RETURNING id`, input.FullName, input.Email, input.Phone, input.DateOfBirth, input.Course, input.PreviousSchool, input.Message).Scan(&id)
	if err != nil {
		http.Error(w, "application could not be submitted", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": id, "status": "RECEIVED"})
}

func (h *IntakeHandler) ListApplications(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	query := `SELECT id, full_name, email, phone, COALESCE(date_of_birth::text,''), course, COALESCE(previous_school,''), COALESCE(message,''), status, created_at FROM applications`
	args := []any{}
	if status != "" { query += ` WHERE status = $1`; args = append(args, status) }
	query += ` ORDER BY created_at DESC`
	rows, err := h.db.QueryContext(r.Context(), query, args...)
	if err != nil { http.Error(w, "database error", 500); return }
	defer rows.Close()
	var result []map[string]any
	for rows.Next() { var id int64; var fullName, email, phone, dob, course, previous, message, state string; var created any; if err := rows.Scan(&id, &fullName, &email, &phone, &dob, &course, &previous, &message, &state, &created); err != nil { http.Error(w, "database error", 500); return }; result = append(result, map[string]any{"id": id, "full_name": fullName, "email": email, "phone": phone, "date_of_birth": dob, "course": course, "previous_school": previous, "message": message, "status": state, "created_at": created}) }
	writeJSON(w, http.StatusOK, result)
}

func (h *IntakeHandler) ReviewApplication(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil { http.Error(w, "invalid application id", 400); return }
	var input struct { Status string `json:"status"` }
	if json.NewDecoder(r.Body).Decode(&input) != nil || (input.Status != "REVIEWING" && input.Status != "ACCEPTED" && input.Status != "DECLINED") { http.Error(w, "status must be REVIEWING, ACCEPTED or DECLINED", 400); return }
	result, err := h.db.ExecContext(r.Context(), `UPDATE applications SET status = $1 WHERE id = $2`, input.Status, id)
	rowsAffected, rowsErr := result.RowsAffected()
	if err != nil || rowsErr != nil || rowsAffected == 0 { http.Error(w, "application not found", 404); return }
	writeJSON(w, 200, map[string]any{"id": id, "status": input.Status})
}

func (h *IntakeHandler) AdmitApplication(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil { http.Error(w, "invalid application id", 400); return }
	tx, err := h.db.BeginTx(r.Context(), nil); if err != nil { http.Error(w, "database error", 500); return }; defer tx.Rollback()
	var name, email, phone, dob string
	var status string
	if err := tx.QueryRowContext(r.Context(), `SELECT full_name,email,phone,COALESCE(date_of_birth::text,''),status FROM applications WHERE id=$1 AND status IN ('ACCEPTED','ADMITTED')`, id).Scan(&name, &email, &phone, &dob, &status); err != nil {
		http.Error(w, "accepted application not found", 404)
		return
	}
	var studentID int64
	admission := fmt.Sprintf("ADM-%06d", id)
	if status == "ADMITTED" {
		if err := tx.QueryRowContext(r.Context(), `SELECT student_id FROM students WHERE id_number=$1`, "APP-"+strconv.FormatInt(id, 10)).Scan(&studentID); err != nil {
			http.Error(w, "student not found", 404)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"student_id": studentID, "admission_number": admission})
		return
	}
	if err := tx.QueryRowContext(r.Context(), `INSERT INTO students(name,id_number,email,date_of_birth,phone_number,admission_number) VALUES($1,$2,$3,COALESCE(NULLIF($4,'')::date,CURRENT_DATE),$5,$6) RETURNING student_id`, name, "APP-"+strconv.FormatInt(id, 10), email, dob, phone, admission).Scan(&studentID); err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "student could not be created", 409)
			return
		}
		if strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "unique") {
			if err := tx.QueryRowContext(r.Context(), `SELECT student_id, admission_number FROM students WHERE id_number=$1`, "APP-"+strconv.FormatInt(id, 10)).Scan(&studentID, &admission); err != nil {
				http.Error(w, "student could not be created", 409)
				return
			}
			if _, err := tx.ExecContext(r.Context(), `UPDATE applications SET status='ADMITTED' WHERE id=$1`, id); err != nil {
				http.Error(w, "application could not be updated", 500)
				return
			}
			if err := tx.Commit(); err != nil { http.Error(w, "database error", 500); return }
			writeJSON(w, http.StatusOK, map[string]any{"student_id": studentID, "admission_number": admission})
			return
		}
		http.Error(w, "student could not be created", 409)
		return
	}
	if _, err := tx.ExecContext(r.Context(), `UPDATE applications SET status='ADMITTED' WHERE id=$1`, id); err != nil { http.Error(w, "application could not be updated", 500); return }
	if err := tx.Commit(); err != nil { http.Error(w, "database error", 500); return }
	writeJSON(w, http.StatusCreated, map[string]any{"student_id": studentID, "admission_number": admission})
}
