package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"school-platform/shared/identity"
)

type AcademicHandler struct{ db *pgxpool.Pool }

func NewAcademicHandler(db *pgxpool.Pool) *AcademicHandler { return &AcademicHandler{db: db} }

func (h *AcademicHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("/api/subjects", h.subjects)
	mux.HandleFunc("/api/classes", h.classes)
	mux.HandleFunc("/api/enrollments", h.enrollments)
	mux.HandleFunc("/api/assessments", h.assessments)
	mux.HandleFunc("/api/teacher/class-subjects", h.teacherClassSubjects)
	mux.HandleFunc("/api/marks", h.marks)
	mux.HandleFunc("/api/marks/submit", h.submitMarks)
	mux.HandleFunc("/api/marks/approve", h.approveMarks)
	mux.HandleFunc("/api/marks/reject", h.rejectMarks)
	mux.HandleFunc("/api/teacher/mark-sheet", h.markSheet)
}

func (h *AcademicHandler) markSheet(w http.ResponseWriter, r *http.Request) {
	classID, err := strconv.ParseInt(r.URL.Query().Get("class_offering_id"), 10, 64)
	if err != nil || classID <= 0 {
		http.Error(w, "class_offering_id is required", 400)
		return
	}
	assessmentID, _ := strconv.ParseInt(r.URL.Query().Get("assessment_id"), 10, 64)
	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if err := identity.ParseSession(r, pgxSessionChecker{db: h.db}, token); err != nil {
		http.Error(w, "authentication required", http.StatusUnauthorized)
		return
	}
	claims, err := identity.ParseJWT(token, jwtSecret(), time.Now())
	if err != nil {
		http.Error(w, "authentication required", http.StatusUnauthorized)
		return
	}
	var rows pgx.Rows
	if claims.Role == identity.RoleTeacher {
		teacherID, convErr := strconv.ParseInt(claims.Subject, 10, 64)
		if convErr != nil {
			http.Error(w, "invalid staff session", http.StatusUnauthorized)
			return
		}
		if assessmentID > 0 {
			rows, err = h.db.Query(r.Context(), `SELECT e.student_id, s.name, m.id AS mark_id, COALESCE(m.score, 0), COALESCE(m.status, '') FROM student_term_enrollment e JOIN students s ON s.student_id = e.student_id LEFT JOIN marks m ON m.student_id = e.student_id AND m.assessment_id = $3 WHERE e.class_offering_id = $1 AND e.status IN ('ENROLLED','ACTIVE') AND EXISTS (SELECT 1 FROM class_subjects cs WHERE cs.class_offering_id = $1 AND (cs.teacher_id = $2 OR EXISTS (SELECT 1 FROM class_offerings co WHERE co.id = $1 AND co.class_teacher_id = $2))) ORDER BY s.name`, classID, teacherID, assessmentID)
		} else {
			rows, err = h.db.Query(r.Context(), `SELECT e.student_id, s.name, NULL::bigint AS mark_id, 0::numeric(6,2) AS score, '' AS status FROM student_term_enrollment e JOIN students s ON s.student_id = e.student_id WHERE e.class_offering_id = $1 AND e.status IN ('ENROLLED','ACTIVE') AND EXISTS (SELECT 1 FROM class_subjects cs WHERE cs.class_offering_id = $1 AND (cs.teacher_id = $2 OR EXISTS (SELECT 1 FROM class_offerings co WHERE co.id = $1 AND co.class_teacher_id = $2))) ORDER BY s.name`, classID, teacherID)
		}
	} else {
		if assessmentID > 0 {
			rows, err = h.db.Query(r.Context(), `SELECT e.student_id, s.name, m.id AS mark_id, COALESCE(m.score, 0), COALESCE(m.status, '') FROM student_term_enrollment e JOIN students s ON s.student_id = e.student_id LEFT JOIN marks m ON m.student_id = e.student_id AND m.assessment_id = $2 WHERE e.class_offering_id = $1 AND e.status IN ('ENROLLED','ACTIVE') ORDER BY s.name`, classID, assessmentID)
		} else {
			rows, err = h.db.Query(r.Context(), `SELECT e.student_id, s.name, NULL::bigint AS mark_id, 0::numeric(6,2) AS score, '' AS status FROM student_term_enrollment e JOIN students s ON s.student_id = e.student_id WHERE e.class_offering_id = $1 AND e.status IN ('ENROLLED','ACTIVE') ORDER BY s.name`, classID)
		}
	}
	if err != nil {
		http.Error(w, "database error", 500)
		return
	}
	defer rows.Close()
	var result []map[string]any
	for rows.Next() {
		var id int64
		var name string
		var markID *int64
		var score float64
		var status string
		if err := rows.Scan(&id, &name, &markID, &score, &status); err != nil {
			http.Error(w, "database error", 500)
			return
		}
		result = append(result, map[string]any{"student_id": id, "name": name, "mark_id": markID, "score": score, "status": status})
	}
	writeJSON(w, 200, result)
}

func decodeBody(r *http.Request, target any) error { return json.NewDecoder(r.Body).Decode(target) }
func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func pageQuery(r *http.Request) (int, int) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	size, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 50
	}
	if size > 200 {
		size = 200
	}
	return page, size
}
func addPage(w http.ResponseWriter, query string, args []any, r *http.Request) (string, []any) {
	page, size := pageQuery(r)
	w.Header().Set("X-Page", strconv.Itoa(page))
	w.Header().Set("X-Page-Size", strconv.Itoa(size))
	query += fmt.Sprintf(" LIMIT $%d OFFSET $%d", len(args)+1, len(args)+2)
	return query, append(args, size, (page-1)*size)
}

func (h *AcademicHandler) subjects(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		rows, err := h.db.Query(r.Context(), `SELECT id, name, COALESCE(code,''), department_id FROM subjects ORDER BY name`)
		if err != nil {
			http.Error(w, "database error", 500)
			return
		}
		defer rows.Close()
		result := []map[string]any{}
		for rows.Next() {
			var id int64
			var name, code string
			var departmentID *int64
			if err := rows.Scan(&id, &name, &code, &departmentID); err != nil {
				http.Error(w, "database error", 500)
				return
			}
			result = append(result, map[string]any{"id": id, "name": name, "code": code, "department_id": departmentID})
		}
		writeJSON(w, 200, result)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	var input struct {
		Name         string `json:"name"`
		Code         string `json:"code"`
		DepartmentID *int64 `json:"department_id"`
	}
	if decodeBody(r, &input) != nil || strings.TrimSpace(input.Name) == "" {
		http.Error(w, "name is required", 400)
		return
	}
	var id int64
	err := h.db.QueryRow(r.Context(), `INSERT INTO subjects (name, code, department_id) VALUES ($1, NULLIF($2,''), $3) RETURNING id`, input.Name, input.Code, input.DepartmentID).Scan(&id)
	if err != nil {
		http.Error(w, "subject could not be created", 409)
		return
	}
	writeJSON(w, 201, map[string]any{"id": id, "name": input.Name, "code": input.Code, "department_id": input.DepartmentID})
}

func (h *AcademicHandler) classes(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		term := r.URL.Query().Get("academic_term_id")
		teacher := r.URL.Query().Get("teacher_id")
		if token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "); token != "" {
			if err := identity.ParseSession(r, pgxSessionChecker{db: h.db}, token); err != nil {
				http.Error(w, "authentication required", http.StatusUnauthorized)
				return
			}
			claims, err := identity.ParseJWT(token, jwtSecret(), time.Now())
			if err != nil {
				http.Error(w, "authentication required", http.StatusUnauthorized)
				return
			}
			if claims.Role == identity.RoleTeacher {
				teacher = claims.Subject
			}
		}
		query := `SELECT DISTINCT c.id, c.course_id, c.academic_term_id, c.name, c.class_teacher_id, c.capacity FROM class_offerings c LEFT JOIN class_subjects cs ON cs.class_offering_id = c.id`
		conditions := []string{}
		args := []any{}
		if term != "" {
			conditions = append(conditions, fmt.Sprintf("c.academic_term_id = $%d", len(args)+1))
			args = append(args, term)
		}
		if teacher != "" {
			conditions = append(conditions, fmt.Sprintf("(c.class_teacher_id = $%d OR cs.teacher_id = $%d)", len(args)+1, len(args)+1))
			args = append(args, teacher)
		}
		if len(conditions) > 0 {
			query += " WHERE " + strings.Join(conditions, " AND ")
		}
		query += " ORDER BY c.id DESC"
		rows, err := h.db.Query(r.Context(), query, args...)
		if err != nil {
			http.Error(w, "database error", 500)
			return
		}
		defer rows.Close()
		result := []map[string]any{}
		for rows.Next() {
			var id, courseID, termID int64
			var name string
			var teacherID *int64
			var capacity *int
			if err := rows.Scan(&id, &courseID, &termID, &name, &teacherID, &capacity); err != nil {
				http.Error(w, "database error", 500)
				return
			}
			result = append(result, map[string]any{"id": id, "course_id": courseID, "academic_term_id": termID, "name": name, "class_teacher_id": teacherID, "capacity": capacity})
		}
		writeJSON(w, 200, result)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	var input struct {
		CourseID       int64  `json:"course_id"`
		AcademicTermID int64  `json:"academic_term_id"`
		Name           string `json:"name"`
		ClassTeacherID *int64 `json:"class_teacher_id"`
		Capacity       *int   `json:"capacity"`
	}
	if decodeBody(r, &input) != nil || input.CourseID <= 0 || input.AcademicTermID <= 0 || strings.TrimSpace(input.Name) == "" {
		http.Error(w, "course_id, academic_term_id and name are required", 400)
		return
	}
	var id int64
	err := h.db.QueryRow(r.Context(), `INSERT INTO class_offerings (course_id, academic_term_id, name, class_teacher_id, capacity) VALUES ($1,$2,$3,$4,$5) RETURNING id`, input.CourseID, input.AcademicTermID, input.Name, input.ClassTeacherID, input.Capacity).Scan(&id)
	if err != nil {
		http.Error(w, "class could not be created", 409)
		return
	}
	writeJSON(w, 201, map[string]any{"id": id})
}

func (h *AcademicHandler) enrollments(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		studentID := r.URL.Query().Get("student_id")
		query := `SELECT id, student_id, academic_term_id, class_offering_id, status FROM student_term_enrollment ORDER BY id DESC`
		args := []any{}
		if studentID != "" {
			query = `SELECT id, student_id, academic_term_id, class_offering_id, status FROM student_term_enrollment WHERE student_id = $1 ORDER BY id DESC`
			args = append(args, studentID)
		}
		rows, err := h.db.Query(r.Context(), query, args...)
		if err != nil {
			http.Error(w, "database error", 500)
			return
		}
		defer rows.Close()
		result := []map[string]any{}
		for rows.Next() {
			var id, sid, tid int64
			var classID *int64
			var status string
			if err := rows.Scan(&id, &sid, &tid, &classID, &status); err != nil {
				http.Error(w, "database error", 500)
				return
			}
			result = append(result, map[string]any{"id": id, "student_id": sid, "academic_term_id": tid, "class_offering_id": classID, "status": status})
		}
		writeJSON(w, 200, result)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	var input struct {
		StudentID       int64  `json:"student_id"`
		AcademicTermID  int64  `json:"academic_term_id"`
		ClassOfferingID *int64 `json:"class_offering_id"`
		Status          string `json:"status"`
	}
	if decodeBody(r, &input) != nil || input.StudentID <= 0 || input.AcademicTermID <= 0 {
		http.Error(w, "student_id and academic_term_id are required", 400)
		return
	}
	if input.Status == "" {
		input.Status = "ENROLLED"
	}
	if input.Status != "ENROLLED" && input.Status != "ACTIVE" {
		http.Error(w, "status must be ENROLLED or ACTIVE", 400)
		return
	}
	var id int64
	err := h.db.QueryRow(r.Context(), `INSERT INTO student_term_enrollment (student_id, academic_term_id, class_offering_id, status) SELECT $1,$2,$3,$4 WHERE ($3 IS NULL OR EXISTS (SELECT 1 FROM class_offerings co WHERE co.id=$3 AND co.academic_term_id=$2)) AND ($3 IS NULL OR NOT EXISTS (SELECT 1 FROM class_offerings co WHERE co.id=$3 AND co.capacity IS NOT NULL AND (SELECT COUNT(*) FROM student_term_enrollment e WHERE e.class_offering_id=co.id AND e.status IN ('ENROLLED','ACTIVE')) >= co.capacity)) ON CONFLICT (student_id, academic_term_id) DO UPDATE SET class_offering_id = EXCLUDED.class_offering_id, status = EXCLUDED.status, version = student_term_enrollment.version + 1 WHERE student_term_enrollment.status IN ('DEFERRED','ENROLLED','ACTIVE') RETURNING id`, input.StudentID, input.AcademicTermID, input.ClassOfferingID, input.Status).Scan(&id)
	if err == pgx.ErrNoRows {
		http.Error(w, "class is from another term, full, or enrollment is not reopenable", 409)
		return
	}
	if err != nil {
		http.Error(w, "enrollment could not be saved", 409)
		return
	}
	writeJSON(w, 201, map[string]any{"id": id, "status": input.Status})
}

func (h *AcademicHandler) assessments(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		page, size := pageQuery(r)
		w.Header().Set("X-Page", strconv.Itoa(page))
		w.Header().Set("X-Page-Size", strconv.Itoa(size))
		query := `SELECT a.id, a.class_subject_id, a.name, a.assessment_type, a.max_score, COALESCE(a.weight_percent, 0), cs.class_offering_id, cs.subject_id FROM assessments a JOIN class_subjects cs ON cs.id = a.class_subject_id`
		args := []any{}
		token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if token != "" {
			if err := identity.ParseSession(r, pgxSessionChecker{db: h.db}, token); err != nil {
				http.Error(w, "authentication required", http.StatusUnauthorized)
				return
			}
			claims, err := identity.ParseJWT(token, jwtSecret(), time.Now())
			if err != nil {
				http.Error(w, "authentication required", http.StatusUnauthorized)
				return
			}
			if claims.Role == identity.RoleTeacher {
				query += ` WHERE cs.teacher_id = $1`
				args = append(args, claims.Subject)
			}
		}
		query += ` ORDER BY a.id DESC`
		query, args = addPage(w, query, args, r)
		rows, err := h.db.Query(r.Context(), query, args...)
		if err != nil {
			http.Error(w, "database error", 500)
			return
		}
		defer rows.Close()
		result := []map[string]any{}
		for rows.Next() {
			var id, classSubjectID, classID, subjectID int64
			var name, assessmentType string
			var maxScore, weight float64
			if err := rows.Scan(&id, &classSubjectID, &name, &assessmentType, &maxScore, &weight, &classID, &subjectID); err != nil {
				http.Error(w, "database error", 500)
				return
			}
			result = append(result, map[string]any{"id": id, "class_subject_id": classSubjectID, "name": name, "assessment_type": assessmentType, "max_score": maxScore, "weight_percent": weight, "class_offering_id": classID, "subject_id": subjectID})
		}
		writeJSON(w, 200, result)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	var input struct {
		ClassSubjectID int64    `json:"class_subject_id"`
		Name           string   `json:"name"`
		AssessmentType string   `json:"assessment_type"`
		MaxScore       float64  `json:"max_score"`
		WeightPercent  *float64 `json:"weight_percent"`
		CreatedBy      *int64   `json:"created_by"`
	}
	if decodeBody(r, &input) != nil || input.ClassSubjectID <= 0 || strings.TrimSpace(input.Name) == "" || input.MaxScore <= 0 {
		http.Error(w, "class_subject_id, name and max_score are required", 400)
		return
	}
	if input.AssessmentType != "CAT" && input.AssessmentType != "EXAM" && input.AssessmentType != "ASSIGNMENT" && input.AssessmentType != "PRACTICAL" {
		http.Error(w, "invalid assessment_type", 400)
		return
	}
	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if err := identity.ParseSession(r, pgxSessionChecker{db: h.db}, token); err != nil {
		http.Error(w, "authentication required", http.StatusUnauthorized)
		return
	}
	claims, err := identity.ParseJWT(token, jwtSecret(), time.Now())
	if err != nil {
		http.Error(w, "authentication required", http.StatusUnauthorized)
		return
	}
	if claims.Role == identity.RoleTeacher {
		var owner int64
		if err := h.db.QueryRow(r.Context(), `SELECT teacher_id FROM class_subjects WHERE id=$1`, input.ClassSubjectID).Scan(&owner); err != nil || owner != actorIDFromClaims(claims) {
			http.Error(w, "teacher is not assigned to this subject", http.StatusForbidden)
			return
		}
		input.CreatedBy = ptrInt64(actorIDFromClaims(claims))
	} else if claims.Role != identity.RoleAcademicAdmin && claims.Role != identity.RoleSuperAdmin {
		http.Error(w, "not authorized", http.StatusForbidden)
		return
	}
	var id int64
	err = h.db.QueryRow(r.Context(), `INSERT INTO assessments (class_subject_id, name, assessment_type, max_score, weight_percent, created_by) VALUES ($1,$2,$3,$4,$5,$6) RETURNING id`, input.ClassSubjectID, input.Name, input.AssessmentType, input.MaxScore, input.WeightPercent, input.CreatedBy).Scan(&id)
	if err != nil {
		http.Error(w, "assessment could not be created", 409)
		return
	}
	writeJSON(w, 201, map[string]any{"id": id})
}

func actorIDFromClaims(claims identity.Claims) int64 {
	actorID, _ := strconv.ParseInt(claims.Subject, 10, 64)
	return actorID
}

func ptrInt64(v int64) *int64 { return &v }

func (h *AcademicHandler) teacherClassSubjects(w http.ResponseWriter, r *http.Request) {
	classID, err := strconv.ParseInt(r.URL.Query().Get("class_offering_id"), 10, 64)
	if err != nil || classID <= 0 {
		http.Error(w, "class_offering_id is required", 400)
		return
	}
	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if err := identity.ParseSession(r, pgxSessionChecker{db: h.db}, token); err != nil {
		http.Error(w, "authentication required", 401)
		return
	}
	claims, err := identity.ParseJWT(token, jwtSecret(), time.Now())
	if err != nil || claims.Role != identity.RoleTeacher {
		http.Error(w, "teacher authorization required", 403)
		return
	}
	teacherID, _ := strconv.ParseInt(claims.Subject, 10, 64)
	rows, err := h.db.Query(r.Context(), `SELECT cs.id, s.name FROM class_subjects cs JOIN subjects s ON s.id = cs.subject_id WHERE cs.class_offering_id = $1 AND cs.teacher_id = $2 ORDER BY s.name`, classID, teacherID)
	if err != nil {
		http.Error(w, "database error", 500)
		return
	}
	defer rows.Close()
	var result []map[string]any
	for rows.Next() {
		var id int64
		var name string
		if rows.Scan(&id, &name) != nil {
			http.Error(w, "database error", 500)
			return
		}
		result = append(result, map[string]any{"class_subject_id": id, "subject_name": name})
	}
	writeJSON(w, 200, result)
}

func (h *AcademicHandler) marks(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if err := identity.ParseSession(r, pgxSessionChecker{db: h.db}, token); err != nil {
			http.Error(w, "authentication required", http.StatusUnauthorized)
			return
		}
		claims, err := identity.ParseJWT(token, jwtSecret(), time.Now())
		if err != nil {
			http.Error(w, "authentication required", http.StatusUnauthorized)
			return
		}
		status := r.URL.Query().Get("status")
		query := `SELECT m.id, m.assessment_id, m.student_id, st.name, m.score, m.status, COALESCE(m.rejection_reason,''), a.name, COALESCE(co.name,''), COALESCE(s.name,'') FROM marks m JOIN assessments a ON a.id = m.assessment_id JOIN students st ON st.student_id = m.student_id JOIN class_subjects cs ON cs.id = a.class_subject_id JOIN subjects s ON s.id = cs.subject_id JOIN class_offerings co ON co.id = cs.class_offering_id ORDER BY m.id DESC`
		args := []any{}
		if status != "" {
			query = `SELECT m.id, m.assessment_id, m.student_id, st.name, m.score, m.status, COALESCE(m.rejection_reason,''), a.name, COALESCE(co.name,''), COALESCE(s.name,'') FROM marks m JOIN assessments a ON a.id = m.assessment_id JOIN students st ON st.student_id = m.student_id JOIN class_subjects cs ON cs.id = a.class_subject_id JOIN subjects s ON s.id = cs.subject_id JOIN class_offerings co ON co.id = cs.class_offering_id WHERE m.status = $1`
			args = append(args, status)
		}
		if claims.Role == identity.RoleTeacher {
			if status == "" {
				query += ` WHERE m.submitted_by = $1`
				args = append(args, claims.Subject)
			} else {
				query += ` AND m.submitted_by = $2`
				args = append(args, claims.Subject)
			}
		}
		if claims.Role == identity.RoleHOD {
			if status == "" {
				query += ` WHERE EXISTS (SELECT 1 FROM assessments a2 JOIN class_subjects cs2 ON cs2.id = a2.class_subject_id JOIN subjects s2 ON s2.id = cs2.subject_id JOIN staff_users u2 ON u2.id = $1 WHERE a2.id = m.assessment_id AND u2.role = 'hod' AND (u2.department_id = s2.department_id OR s2.department_id IS NULL))`
			} else {
				query += ` AND EXISTS (SELECT 1 FROM assessments a2 JOIN class_subjects cs2 ON cs2.id = a2.class_subject_id JOIN subjects s2 ON s2.id = cs2.subject_id JOIN staff_users u2 ON u2.id = $2 WHERE a2.id = m.assessment_id AND u2.role = 'hod' AND (u2.department_id = s2.department_id OR s2.department_id IS NULL))`
			}
			args = append(args, claims.Subject)
		}
		query += ` ORDER BY m.id DESC`
		query, args = addPage(w, query, args, r)
		rows, err := h.db.Query(r.Context(), query, args...)
		if err != nil {
			http.Error(w, "database error", 500)
			return
		}
		defer rows.Close()
		result := []map[string]any{}
		for rows.Next() {
			var id, assessmentID, studentID int64
			var studentName string
			var score float64
			var markStatus, reason, assessmentName, className, subjectName string
			if err := rows.Scan(&id, &assessmentID, &studentID, &studentName, &score, &markStatus, &reason, &assessmentName, &className, &subjectName); err != nil {
				http.Error(w, "database error", 500)
				return
			}
			result = append(result, map[string]any{"id": id, "assessment_id": assessmentID, "student_id": studentID, "student_name": studentName, "score": score, "status": markStatus, "rejection_reason": reason, "assessment_name": assessmentName, "class_name": className, "subject_name": subjectName})
		}
		writeJSON(w, 200, result)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	var input struct {
		AssessmentID int64   `json:"assessment_id"`
		StudentID    int64   `json:"student_id"`
		Score        float64 `json:"score"`
		SubmittedBy  *int64  `json:"submitted_by"`
	}
	if decodeBody(r, &input) != nil || input.AssessmentID <= 0 || input.StudentID <= 0 || input.Score < 0 {
		http.Error(w, "assessment_id, student_id and score are required", 400)
		return
	}
	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if err := identity.ParseSession(r, pgxSessionChecker{db: h.db}, token); err != nil {
		http.Error(w, "teacher authorization required", http.StatusForbidden)
		return
	}
	claims, err := identity.ParseJWT(token, jwtSecret(), time.Now())
	if err != nil || claims.Role != identity.RoleTeacher {
		http.Error(w, "teacher authorization required", http.StatusForbidden)
		return
	}
	actorID, err := strconv.ParseInt(claims.Subject, 10, 64)
	if err != nil {
		http.Error(w, "invalid staff session", http.StatusUnauthorized)
		return
	}
	var maxScore float64
	var assignedTeacher *int64
	if err := h.db.QueryRow(r.Context(), `SELECT a.max_score, cs.teacher_id FROM assessments a JOIN class_subjects cs ON cs.id = a.class_subject_id WHERE a.id = $1`, input.AssessmentID).Scan(&maxScore, &assignedTeacher); err != nil {
		http.Error(w, "assessment not found", http.StatusBadRequest)
		return
	}
	if assignedTeacher == nil || *assignedTeacher != actorID {
		http.Error(w, "teacher is not assigned to this subject", http.StatusForbidden)
		return
	}
	if input.Score > maxScore {
		http.Error(w, "score exceeds assessment maximum", http.StatusBadRequest)
		return
	}
	input.SubmittedBy = &actorID
	var id int64
	err = h.db.QueryRow(r.Context(), `INSERT INTO marks (assessment_id, student_id, score, submitted_by) VALUES ($1,$2,$3,$4) ON CONFLICT (assessment_id, student_id) DO UPDATE SET score = EXCLUDED.score, submitted_by = EXCLUDED.submitted_by, status = 'DRAFT', version = marks.version + 1, approved_by = NULL, approved_at = NULL, rejection_reason = NULL WHERE marks.status IN ('DRAFT', 'REJECTED') RETURNING id`, input.AssessmentID, input.StudentID, input.Score, input.SubmittedBy).Scan(&id)
	if err == pgx.ErrNoRows {
		http.Error(w, "mark is already submitted or approved", 409)
		return
	}
	if err != nil {
		http.Error(w, "mark could not be saved", 409)
		return
	}
	writeJSON(w, 201, map[string]any{"id": id, "status": "DRAFT"})
}

func (h *AcademicHandler) transitionMark(w http.ResponseWriter, r *http.Request, status string) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	var input struct {
		MarkID          int64  `json:"mark_id"`
		ActorID         int64  `json:"actor_id"`
		RejectionReason string `json:"rejection_reason"`
	}
	if decodeBody(r, &input) != nil || input.MarkID <= 0 {
		http.Error(w, "mark_id is required", 400)
		return
	}
	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if err := identity.ParseSession(r, pgxSessionChecker{db: h.db}, token); err != nil {
		http.Error(w, "authentication required", http.StatusUnauthorized)
		return
	}
	claims, err := identity.ParseJWT(token, jwtSecret(), time.Now())
	if err != nil {
		http.Error(w, "authentication required", http.StatusUnauthorized)
		return
	}
	switch status {
	case "SUBMITTED":
		if claims.Role != identity.RoleTeacher {
			http.Error(w, "teacher authorization required", http.StatusForbidden)
			return
		}
	case "APPROVED", "REJECTED":
		if claims.Role != identity.RoleHOD {
			http.Error(w, "HOD authorization required", http.StatusForbidden)
			return
		}
	default:
		http.Error(w, "unsupported mark transition", http.StatusBadRequest)
		return
	}
	actorID, err := strconv.ParseInt(claims.Subject, 10, 64)
	if err != nil {
		http.Error(w, "invalid staff session", http.StatusUnauthorized)
		return
	}
	input.ActorID = actorID
	var query string
	var args []any
	switch status {
	case "SUBMITTED":
		query = `UPDATE marks SET status='SUBMITTED', submitted_by=$2, submitted_at=CURRENT_TIMESTAMP, version=version+1 WHERE id=$1 AND status IN ('DRAFT','REJECTED') AND EXISTS (SELECT 1 FROM assessments a JOIN class_subjects cs ON cs.id = a.class_subject_id WHERE a.id = marks.assessment_id AND cs.teacher_id = $2)`
		args = []any{input.MarkID, actorID}
	case "APPROVED":
		query = `UPDATE marks SET status='APPROVED', approved_by=$2, approved_at=CURRENT_TIMESTAMP, version=version+1 WHERE id=$1 AND status='SUBMITTED' AND EXISTS (SELECT 1 FROM assessments a JOIN class_subjects cs ON cs.id = a.class_subject_id JOIN subjects s ON s.id = cs.subject_id JOIN staff_users u ON u.id = $2 WHERE a.id = marks.assessment_id AND u.role = 'hod' AND (u.department_id = s.department_id OR s.department_id IS NULL))`
		args = []any{input.MarkID, actorID}
	default:
		query = `UPDATE marks SET status='REJECTED', rejection_reason=$2, version=version+1 WHERE id=$1 AND status='SUBMITTED' AND EXISTS (SELECT 1 FROM assessments a JOIN class_subjects cs ON cs.id = a.class_subject_id JOIN subjects s ON s.id = cs.subject_id JOIN staff_users u ON u.id = $3 WHERE a.id = marks.assessment_id AND u.role = 'hod' AND (u.department_id = s.department_id OR s.department_id IS NULL))`
		args = []any{input.MarkID, input.RejectionReason, actorID}
	}
	tx, err := h.db.Begin(r.Context())
	if err != nil {
		http.Error(w, "database error", 500)
		return
	}
	defer tx.Rollback(r.Context())
	var fromStatus string
	if err := tx.QueryRow(r.Context(), `SELECT status FROM marks WHERE id = $1 FOR UPDATE`, input.MarkID).Scan(&fromStatus); err != nil {
		http.Error(w, "mark transition rejected", 409)
		return
	}
	result, err := tx.Exec(r.Context(), query, args...)
	if err != nil || result.RowsAffected() == 0 {
		http.Error(w, "mark transition rejected", 409)
		return
	}
	if _, err := tx.Exec(r.Context(), `INSERT INTO audit_logs (actor_type, actor_id, action, target_type, target_id, details) VALUES ('staff', $1, 'mark_transition', 'mark', $2, jsonb_build_object('from', $3, 'to', $4, 'reason', $5))`, actorID, input.MarkID, fromStatus, status, input.RejectionReason); err != nil {
		http.Error(w, "audit log failure", 500)
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		http.Error(w, "database error", 500)
		return
	}
	writeJSON(w, 200, map[string]any{"mark_id": input.MarkID, "status": status})
}

func (h *AcademicHandler) submitMarks(w http.ResponseWriter, r *http.Request) {
	h.transitionMark(w, r, "SUBMITTED")
}
func (h *AcademicHandler) approveMarks(w http.ResponseWriter, r *http.Request) {
	h.transitionMark(w, r, "APPROVED")
}
func (h *AcademicHandler) rejectMarks(w http.ResponseWriter, r *http.Request) {
	h.transitionMark(w, r, "REJECTED")
}

func jwtSecret() string {
	return strings.TrimSpace(os.Getenv("JWT_SECRET"))
}
