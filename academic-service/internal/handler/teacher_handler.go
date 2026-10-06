package handler

import (
	"encoding/json"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"school-platform/academic-service/internal/domain"
	"school-platform/academic-service/internal/service"
	"school-platform/shared/identity"
)

type TeacherHandler struct {
	db         *pgxpool.Pool
	submission *service.MarkSubmissionService
	roster     *service.RosterService
}

func NewTeacherHandler(db *pgxpool.Pool, submission *service.MarkSubmissionService, roster *service.RosterService) *TeacherHandler {
	return &TeacherHandler{db: db, submission: submission, roster: roster}
}

func (h *TeacherHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("/api/teacher/draft", h.saveDraft)
	mux.HandleFunc("/api/teacher/submit", h.submit)
	mux.HandleFunc("/api/teacher/roster", h.rosterForStudent)
}

type markDraft struct {
	AssessmentID int64   `json:"assessment_id"`
	StudentID    int64   `json:"student_id"`
	Score        float64 `json:"score"`
	SubmittedBy  *int64  `json:"submitted_by"`
}

type markSubmission struct {
	MarkID int64 `json:"mark_id"`
}

func domainMark(in markDraft) domain.Mark {
	return domain.Mark{
		AssessmentID: in.AssessmentID,
		StudentID:    in.StudentID,
		Score:        in.Score,
		Status:       "DRAFT",
		SubmittedBy:  in.SubmittedBy,
	}
}

func (h *TeacherHandler) saveDraft(w http.ResponseWriter, r *http.Request) {
	var in markDraft
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if err := identity.ParseSession(r, pgxSessionChecker{db: h.db}, token); err != nil {
		http.Error(w, "teacher authorization required", http.StatusForbidden)
		return
	}
	claims, err := identity.ParseJWT(token, strings.TrimSpace(os.Getenv("JWT_SECRET")), time.Now())
	if err != nil || claims.Role != identity.RoleTeacher {
		http.Error(w, "teacher authorization required", http.StatusForbidden)
		return
	}
	actorID, err := strconv.ParseInt(claims.Subject, 10, 64)
	if err != nil { http.Error(w, "invalid staff session", http.StatusUnauthorized); return }
	var maxScore float64
	var assignedTeacher *int64
	if err := h.db.QueryRow(r.Context(), `SELECT a.max_score, cs.teacher_id FROM assessments a JOIN class_subjects cs ON cs.id = a.class_subject_id WHERE a.id = $1`, in.AssessmentID).Scan(&maxScore, &assignedTeacher); err != nil {
		http.Error(w, "assessment not found", http.StatusBadRequest)
		return
	}
	if assignedTeacher == nil || *assignedTeacher != actorID {
		http.Error(w, "teacher is not assigned to this subject", http.StatusForbidden)
		return
	}
	if in.Score > maxScore {
		http.Error(w, "score exceeds assessment maximum", http.StatusBadRequest)
		return
	}
	in.SubmittedBy = &actorID
	id, err := h.submission.SaveDraft(r.Context(), domainMark(in))
	if err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]any{"id": id, "status": "DRAFT"})
}

func (h *TeacherHandler) submit(w http.ResponseWriter, r *http.Request) {
	var in markSubmission
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if err := identity.ParseSession(r, pgxSessionChecker{db: h.db}, token); err != nil {
		http.Error(w, "teacher authorization required", http.StatusForbidden)
		return
	}
	claims, err := identity.ParseJWT(token, strings.TrimSpace(os.Getenv("JWT_SECRET")), time.Now())
	if err != nil || claims.Role != identity.RoleTeacher {
		http.Error(w, "teacher authorization required", http.StatusForbidden)
		return
	}
	teacherID, err := strconv.ParseInt(claims.Subject, 10, 64)
	if err != nil {
		http.Error(w, "invalid staff session", http.StatusUnauthorized)
		return
	}
	if err := h.submission.Submit(r.Context(), in.MarkID, teacherID); err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (h *TeacherHandler) rosterForStudent(w http.ResponseWriter, r *http.Request) {
	studentID, err := strconv.ParseInt(r.URL.Query().Get("student_id"), 10, 64)
	if err != nil || studentID <= 0 {
		http.Error(w, "invalid student_id", http.StatusBadRequest)
		return
	}
	items, err := h.roster.StudentRoster(r.Context(), studentID)
	if err != nil {
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(items)
}