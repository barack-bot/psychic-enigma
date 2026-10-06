package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
	"school-platform/academic-service/internal/service"
	"school-platform/shared/identity"
)

type pgxSessionChecker struct{ db *pgxpool.Pool }

func (c pgxSessionChecker) SessionExists(ctx context.Context, tokenHash string) bool {
	if c.db == nil {
		return false
	}
	var exists bool
	if err := c.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM auth_sessions WHERE token_hash=$1 AND expires_at > CURRENT_TIMESTAMP)`, tokenHash).Scan(&exists); err != nil {
		return false
	}
	return exists
}

type AdminHandler struct{ db *pgxpool.Pool; assignments *service.TeacherAssignmentService }

func NewAdminHandler(db *pgxpool.Pool, assignments *service.TeacherAssignmentService) *AdminHandler { return &AdminHandler{db: db, assignments: assignments} }

func (h *AdminHandler) Register(mux *http.ServeMux) {
	for _, resource := range []string{"departments", "levels", "courses", "years", "terms", "classes", "class-subjects", "staff"} {
		mux.HandleFunc("/api/admin/"+resource, h.dispatch)
		mux.HandleFunc("/api/admin/"+resource+"/", h.dispatch)
	}
}

func (h *AdminHandler) dispatch(w http.ResponseWriter, r *http.Request) {
	if !h.adminActor(r) { http.Error(w, "academic administrator authorization required", http.StatusForbidden); return }
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) == 5 && parts[2] == "staff" && parts[4] == "password" {
		h.staffPassword(w, r, parts[3]); return
	}
	if len(parts) == 5 && parts[2] == "class-subjects" && parts[4] == "teacher" {
		h.assignTeacher(w, r, parts[3]); return
	}
	resource := parts[len(parts)-1]
	if len(parts) >= 4 && (r.Method == http.MethodPut || r.Method == http.MethodDelete) {
		resource = parts[2]
		id, err := strconv.ParseInt(parts[3], 10, 64); if err != nil { http.Error(w, "invalid resource id", 400); return }
		h.mutate(w, r, resource, id)
		return
	}
	if parts[len(parts)-1] == "teacher" { resource = "class-subjects" }
	switch resource {
	case "departments": h.departments(w, r)
	case "levels": h.levels(w, r)
	case "courses": h.courses(w, r)
	case "years": h.years(w, r)
	case "terms": h.terms(w, r)
	case "classes": h.classes(w, r)
	case "class-subjects": h.classSubjects(w, r)
	case "staff": h.staff(w, r)
	default: http.NotFound(w, r)
	}
}

func (h *AdminHandler) mutate(w http.ResponseWriter, r *http.Request, resource string, id int64) {
	if r.Method == http.MethodDelete {
		if resource == "staff" {
			result, err := h.db.Exec(r.Context(), `UPDATE staff_users SET status='SUSPENDED' WHERE id=$1`, id)
			if err != nil { adminDBError(w); return }
			if result.RowsAffected() == 0 { http.Error(w, "staff user not found", 404); return }
			if _, err := h.db.Exec(r.Context(), `DELETE FROM auth_sessions WHERE staff_user_id=$1`, id); err != nil { adminDBError(w); return }
			w.WriteHeader(http.StatusNoContent); return
		}
		tables := map[string]string{"departments": "departments", "levels": "course_levels", "courses": "courses", "years": "academic_years", "terms": "academic_terms"}
		table, ok := tables[resource]; if !ok { http.Error(w, "resource does not support delete", 405); return }
		if _, err := h.db.Exec(r.Context(), "DELETE FROM "+table+" WHERE id=$1", id); err != nil { adminDBError(w); return }; w.WriteHeader(http.StatusNoContent); return
	}
	var input map[string]any
	if decodeAdminBody(r, &input) != nil { http.Error(w, "invalid JSON", 400); return }
	var query string; var args []any
	switch resource {
	case "departments": query = `UPDATE departments SET name=$1 WHERE id=$2`; args = []any{input["name"], id}
	case "levels": query = `UPDATE course_levels SET name=$1,total_terms=$2 WHERE id=$3`; args = []any{input["name"], input["total_terms"], id}
	case "courses": query = `UPDATE courses SET name=$1,level_id=$2,department_id=$3 WHERE id=$4`; args = []any{input["name"], input["level_id"], input["department_id"], id}
	case "years": query = `UPDATE academic_years SET year_label=$1,start_date=$2,end_date=$3 WHERE id=$4`; args = []any{input["year_label"], input["start_date"], input["end_date"], id}
	case "terms": query = `UPDATE academic_terms SET academic_year_id=$1,term_number=$2,term_type=$3,name=$4,starts_on=$5,ends_on=$6,version=version+1 WHERE id=$7`; args = []any{input["academic_year_id"], input["term_number"], input["term_type"], input["name"], input["starts_on"], input["ends_on"], id}
	case "staff": query = `UPDATE staff_users SET name=COALESCE(NULLIF($1,''),name), email=NULLIF($2,''), role=COALESCE(NULLIF($3,''),role), department_id=$4, status=COALESCE(NULLIF($5,''),status) WHERE id=$6`; args = []any{input["name"], input["email"], input["role"], input["department_id"], input["status"], id}
	default: http.Error(w, "resource does not support update", 405); return
	}
	if resource == "staff" && (input["role"] != nil || input["status"] != nil) {
		if _, err := h.db.Exec(r.Context(), `DELETE FROM auth_sessions WHERE staff_user_id=$1`, id); err != nil { adminDBError(w); return }
	}
	result, err := h.db.Exec(r.Context(), query, args...); if err != nil { adminDBError(w); return }; if result.RowsAffected() == 0 { http.Error(w, "resource not found", 404); return }; adminJSON(w, 200, map[string]any{"id": id, "status": "updated"})
}

func (h *AdminHandler) adminActor(r *http.Request) bool {
	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if err := identity.ParseSession(r, pgxSessionChecker{db: h.db}, token); err != nil {
		return false
	}
	claims, err := identity.ParseJWT(token, strings.TrimSpace(os.Getenv("JWT_SECRET")), time.Now())
	return err == nil && (claims.Role == identity.RoleAcademicAdmin || claims.Role == identity.RoleSuperAdmin)
}

func decodeAdminBody(r *http.Request, target any) error { return json.NewDecoder(r.Body).Decode(target) }
func adminJSON(w http.ResponseWriter, status int, value any) { w.Header().Set("Content-Type", "application/json"); w.WriteHeader(status); _ = json.NewEncoder(w).Encode(value) }
func adminDBError(w http.ResponseWriter) { http.Error(w, "database error", http.StatusConflict) }

func (h *AdminHandler) departments(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet { rows, err := h.db.Query(r.Context(), `SELECT id,name FROM departments ORDER BY name`); if err != nil { adminDBError(w); return }; defer rows.Close(); var result []map[string]any; for rows.Next() { var id int64; var name string; if rows.Scan(&id, &name) != nil { adminDBError(w); return }; result = append(result, map[string]any{"id": id, "name": name}) }; adminJSON(w, 200, result); return }
	if r.Method != http.MethodPost { http.Error(w, "method not allowed", 405); return }; var in struct{ Name string `json:"name"` }; if decodeAdminBody(r, &in) != nil || strings.TrimSpace(in.Name) == "" { http.Error(w, "name is required", 400); return }; var id int64; if h.db.QueryRow(r.Context(), `INSERT INTO departments(name) VALUES($1) RETURNING id`, in.Name).Scan(&id) != nil { adminDBError(w); return }; adminJSON(w, 201, map[string]any{"id": id, "name": in.Name})
}

func (h *AdminHandler) levels(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet { rows, err := h.db.Query(r.Context(), `SELECT id,name,total_terms FROM course_levels ORDER BY id`); if err != nil { adminDBError(w); return }; defer rows.Close(); var result []map[string]any; for rows.Next() { var id, terms int64; var name string; if rows.Scan(&id, &name, &terms) != nil { adminDBError(w); return }; result = append(result, map[string]any{"id": id, "name": name, "total_terms": terms}) }; adminJSON(w, 200, result); return }
	if r.Method != http.MethodPost { http.Error(w, "method not allowed", 405); return }; var in struct{Name string `json:"name"`; TotalTerms int `json:"total_terms"`}; if decodeAdminBody(r, &in) != nil || in.Name == "" || in.TotalTerms <= 0 { http.Error(w, "name and positive total_terms are required", 400); return }; var id int64; if h.db.QueryRow(r.Context(), `INSERT INTO course_levels(name,total_terms) VALUES($1,$2) RETURNING id`, in.Name, in.TotalTerms).Scan(&id) != nil { adminDBError(w); return }; adminJSON(w, 201, map[string]any{"id": id})
}

func (h *AdminHandler) courses(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet { rows, err := h.db.Query(r.Context(), `SELECT id,name,level_id,department_id FROM courses ORDER BY name`); if err != nil { adminDBError(w); return }; defer rows.Close(); var result []map[string]any; for rows.Next() { var id, level int64; var name string; var department *int64; if rows.Scan(&id, &name, &level, &department) != nil { adminDBError(w); return }; result = append(result, map[string]any{"id": id, "name": name, "level_id": level, "department_id": department}) }; adminJSON(w, 200, result); return }
	if r.Method != http.MethodPost { http.Error(w, "method not allowed", 405); return }; var in struct{Name string `json:"name"`; LevelID int64 `json:"level_id"`; DepartmentID *int64 `json:"department_id"`}; if decodeAdminBody(r, &in) != nil || in.Name == "" || in.LevelID <= 0 { http.Error(w, "name and level_id are required", 400); return }; var id int64; if h.db.QueryRow(r.Context(), `INSERT INTO courses(name,level_id,department_id) VALUES($1,$2,$3) RETURNING id`, in.Name, in.LevelID, in.DepartmentID).Scan(&id) != nil { adminDBError(w); return }; adminJSON(w, 201, map[string]any{"id": id})
}

func (h *AdminHandler) years(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet { rows, err := h.db.Query(r.Context(), `SELECT id,year_label,start_date,end_date FROM academic_years ORDER BY start_date DESC`); if err != nil { adminDBError(w); return }; defer rows.Close(); var result []map[string]any; for rows.Next() { var id int64; var label, start, end string; if rows.Scan(&id, &label, &start, &end) != nil { adminDBError(w); return }; result = append(result, map[string]any{"id": id, "year_label": label, "start_date": start, "end_date": end}) }; adminJSON(w, 200, result); return }
	if r.Method != http.MethodPost { http.Error(w, "method not allowed", 405); return }; var in struct{ Label string `json:"year_label"`; StartDate string `json:"start_date"`; EndDate string `json:"end_date"` }; if decodeAdminBody(r, &in) != nil || in.Label == "" || in.StartDate == "" || in.EndDate == "" { http.Error(w, "year_label, start_date and end_date are required", 400); return }; var id int64; if h.db.QueryRow(r.Context(), `INSERT INTO academic_years(year_label,start_date,end_date) VALUES($1,$2,$3) RETURNING id`, in.Label, in.StartDate, in.EndDate).Scan(&id) != nil { adminDBError(w); return }; adminJSON(w, 201, map[string]any{"id": id})
}

func (h *AdminHandler) terms(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet { rows, err := h.db.Query(r.Context(), `SELECT id,academic_year_id,term_number,term_type,name,starts_on,ends_on FROM academic_terms ORDER BY starts_on DESC`); if err != nil { adminDBError(w); return }; defer rows.Close(); var result []map[string]any; for rows.Next() { var id, year int64; var number int; var typ, name, start, end string; if rows.Scan(&id, &year, &number, &typ, &name, &start, &end) != nil { adminDBError(w); return }; result = append(result, map[string]any{"id": id, "academic_year_id": year, "term_number": number, "term_type": typ, "name": name, "starts_on": start, "ends_on": end}) }; adminJSON(w, 200, result); return }
	if r.Method != http.MethodPost { http.Error(w, "method not allowed", 405); return }; var in struct{ AcademicYearID int64 `json:"academic_year_id"`; TermNumber int `json:"term_number"`; TermType string `json:"term_type"`; Name string `json:"name"`; StartsOn string `json:"starts_on"`; EndsOn string `json:"ends_on"` }; if decodeAdminBody(r, &in) != nil || in.AcademicYearID <= 0 || in.TermNumber < 1 || in.TermNumber > 3 || in.Name == "" || in.StartsOn == "" || in.EndsOn == "" { http.Error(w, "invalid term fields", 400); return }; if in.TermType == "" { in.TermType = "NORMAL" }; var id int64; if h.db.QueryRow(r.Context(), `INSERT INTO academic_terms(academic_year_id,term_number,term_type,name,starts_on,ends_on) VALUES($1,$2,$3,$4,$5,$6) RETURNING id`, in.AcademicYearID, in.TermNumber, in.TermType, in.Name, in.StartsOn, in.EndsOn).Scan(&id) != nil { adminDBError(w); return }; adminJSON(w, 201, map[string]any{"id": id})
}

func (h *AdminHandler) classes(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost { http.Error(w, "method not allowed", 405); return }; var in struct{ CourseID int64 `json:"course_id"`; AcademicTermID int64 `json:"academic_term_id"`; Name string `json:"name"`; ClassTeacherID *int64 `json:"class_teacher_id"`; Capacity *int `json:"capacity"` }; if decodeAdminBody(r, &in) != nil || in.CourseID <= 0 || in.AcademicTermID <= 0 || in.Name == "" { http.Error(w, "course_id, academic_term_id and name are required", 400); return }; var id int64; if h.db.QueryRow(r.Context(), `INSERT INTO class_offerings(course_id,academic_term_id,name,class_teacher_id,capacity) VALUES($1,$2,$3,$4,$5) RETURNING id`, in.CourseID, in.AcademicTermID, in.Name, in.ClassTeacherID, in.Capacity).Scan(&id) != nil { adminDBError(w); return }; adminJSON(w, 201, map[string]any{"id": id})
}

func (h *AdminHandler) assignTeacher(w http.ResponseWriter, r *http.Request, classSubjectIDText string) {
	if r.Method != http.MethodPut {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	classSubjectID, err := strconv.ParseInt(classSubjectIDText, 10, 64)
	if err != nil || classSubjectID <= 0 {
		http.Error(w, "invalid class subject id", http.StatusBadRequest)
		return
	}
	var in struct{ TeacherID int64 `json:"teacher_id"` }
	if decodeAdminBody(r, &in) != nil || in.TeacherID <= 0 {
		http.Error(w, "teacher_id is required", http.StatusBadRequest)
		return
	}
	if err := h.assignments.Assign(r.Context(), classSubjectID, in.TeacherID); err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}
	adminJSON(w, http.StatusOK, map[string]any{"id": classSubjectID, "teacher_id": in.TeacherID})
}

func (h *AdminHandler) classSubjects(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet { id, err := strconv.ParseInt(r.URL.Query().Get("class_offering_id"), 10, 64); if err != nil { http.Error(w, "class_offering_id is required", 400); return }; rows, err := h.db.Query(r.Context(), `SELECT id,class_offering_id,subject_id,teacher_id FROM class_subjects WHERE class_offering_id=$1`, id); if err != nil { adminDBError(w); return }; defer rows.Close(); var result []map[string]any; for rows.Next() { var sid, cid, subject int64; var teacher *int64; if rows.Scan(&sid, &cid, &subject, &teacher) != nil { adminDBError(w); return }; result = append(result, map[string]any{"id": sid, "class_offering_id": cid, "subject_id": subject, "teacher_id": teacher}) }; adminJSON(w, 200, result); return }
	if r.Method != http.MethodPost { http.Error(w, "method not allowed", 405); return }; var in struct{ ClassOfferingID int64 `json:"class_offering_id"`; SubjectID int64 `json:"subject_id"`; TeacherID *int64 `json:"teacher_id"` }; if decodeAdminBody(r, &in) != nil || in.ClassOfferingID <= 0 || in.SubjectID <= 0 { http.Error(w, "class_offering_id and subject_id are required", 400); return }; var id int64; if h.db.QueryRow(r.Context(), `INSERT INTO class_subjects(class_offering_id,subject_id,teacher_id) VALUES($1,$2,$3) RETURNING id`, in.ClassOfferingID, in.SubjectID, in.TeacherID).Scan(&id) != nil { adminDBError(w); return }; adminJSON(w, 201, map[string]any{"id": id})
}

func (h *AdminHandler) staff(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet { rows, err := h.db.Query(r.Context(), `SELECT id,COALESCE(employee_no,''),name,COALESCE(email,''),role,department_id,status FROM staff_users ORDER BY name`); if err != nil { adminDBError(w); return }; defer rows.Close(); var result []map[string]any; for rows.Next() { var id int64; var employee, name, email, role, status string; var dept *int64; if rows.Scan(&id, &employee, &name, &email, &role, &dept, &status) != nil { adminDBError(w); return }; result = append(result, map[string]any{"id": id, "employee_no": employee, "name": name, "email": email, "role": role, "department_id": dept, "status": status}) }; adminJSON(w, 200, result); return }
	if r.Method != http.MethodPost { http.Error(w, "method not allowed", 405); return }; var in struct{ EmployeeNo string `json:"employee_no"`; Name string `json:"name"`; Email string `json:"email"`; Username string `json:"username"`; Password string `json:"password"`; Role string `json:"role"`; DepartmentID *int64 `json:"department_id"` }; if decodeAdminBody(r, &in) != nil || in.Name == "" || in.Username == "" || len(in.Password) < 8 || !identity.IsStaffRole(in.Role) { http.Error(w, "name, username, role and password of at least 8 characters are required", 400); return }; hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost); if err != nil { http.Error(w, "could not hash password", 500); return }; var id int64; if h.db.QueryRow(r.Context(), `INSERT INTO staff_users(employee_no,name,email,username,password_hash,role,department_id) VALUES(NULLIF($1,''),$2,NULLIF($3,''),$4,$5,$6,$7) RETURNING id`, in.EmployeeNo, in.Name, in.Email, in.Username, string(hash), in.Role, in.DepartmentID).Scan(&id) != nil { adminDBError(w); return }; adminJSON(w, 201, map[string]any{"id": id, "username": in.Username, "role": in.Role})
}

func (h *AdminHandler) staffPassword(w http.ResponseWriter, r *http.Request, rawID string) {
	if r.Method != http.MethodPost { http.Error(w, "method not allowed", 405); return }
	id, err := strconv.ParseInt(rawID, 10, 64); if err != nil { http.Error(w, "invalid staff id", 400); return }
	var input struct { Password string `json:"password"`; Status string `json:"status"` }
	if decodeAdminBody(r, &input) != nil || len(input.Password) < 8 { http.Error(w, "password must be at least 8 characters", 400); return }
	hash, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost); if err != nil { http.Error(w, "could not hash password", 500); return }
	if _, err := h.db.Exec(r.Context(), `DELETE FROM auth_sessions WHERE staff_user_id=$1`, id); err != nil { adminDBError(w); return }
	result, err := h.db.Exec(r.Context(), `UPDATE staff_users SET password_hash=$1 WHERE id=$2`, string(hash), id); if err != nil { adminDBError(w); return }
	if result.RowsAffected() == 0 { http.Error(w, "staff user not found", 404); return }
	if strings.TrimSpace(input.Status) != "" {
		if _, err := h.db.Exec(r.Context(), `UPDATE staff_users SET status=$1 WHERE id=$2`, input.Status, id); err != nil { adminDBError(w); return }
	}
	adminJSON(w, 200, map[string]any{"id": id, "status": "password_reset"})
}
