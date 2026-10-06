package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	sharedexport "school-platform/shared/export"
	"school-platform/shared/identity"
)

type PaymentHandler struct{ db *pgxpool.Pool }

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

type paymentRequest struct {
	StudentID            int64   `json:"student_id"`
	AcademicTermID       *int64  `json:"academic_term_id,omitempty"`
	Source               string  `json:"source"`
	Method               string  `json:"method"`
	Amount               float64 `json:"amount"`
	TransactionReference string  `json:"transaction_reference"`
}

func NewPaymentHandler(db *pgxpool.Pool) *PaymentHandler { return &PaymentHandler{db: db} }

func (h *PaymentHandler) Create(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req paymentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	if req.StudentID <= 0 || req.Amount <= 0 || strings.TrimSpace(req.Method) == "" || strings.TrimSpace(req.Source) == "" {
		http.Error(w, "student_id, amount, source and method are required", http.StatusBadRequest)
		return
	}
	if !validPaymentSource(req.Source) || !validPaymentMethod(req.Method) { http.Error(w, "invalid payment source or method", http.StatusBadRequest); return }
	if strings.TrimSpace(req.TransactionReference) == "" {
		req.TransactionReference = "manual-" + strconv.FormatInt(req.StudentID, 10) + "-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	}
	claims, err := identity.ParseJWT(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "), jwtSecret(), time.Now())
	if err != nil {
		http.Error(w, "authentication required", http.StatusUnauthorized)
		return
	}
	receivedBy, err := strconv.ParseInt(claims.Subject, 10, 64)
	if err != nil {
		http.Error(w, "invalid staff session", http.StatusUnauthorized)
		return
	}
	var id int64
	if err := h.db.QueryRow(r.Context(), `
		INSERT INTO tuition_payments (student_id, academic_term_id, source, method, amount, transaction_reference, status, received_by)
		VALUES ($1, $2, $3, $4, $5, $6, 'COMPLETED', $7)
		RETURNING id`,
		req.StudentID, req.AcademicTermID, req.Source, req.Method, req.Amount, req.TransactionReference, receivedBy,
	).Scan(&id); err != nil {
		http.Error(w, "payment could not be created", http.StatusInternalServerError)
		return
	}
	detailsJSON, err := json.Marshal(map[string]any{"amount": req.Amount, "method": req.Method, "source": req.Source})
	if err == nil {
		_, _ = h.db.Exec(r.Context(), `INSERT INTO audit_logs (actor_type, actor_id, action, target_type, target_id, details) VALUES ('staff', $1, 'payment_recorded', 'payment', $2, $3)`, receivedBy, id, detailsJSON)
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]any{"id": id, "student_id": req.StudentID, "source": req.Source, "method": req.Method, "amount": req.Amount, "transaction_reference": req.TransactionReference, "status": "COMPLETED"})
}

func validPaymentSource(value string) bool { switch value { case "STUDENT", "HELB", "SCHOLARSHIP", "OTHER": return true }; return false }
func validPaymentMethod(value string) bool { switch value { case "MPESA", "BANK", "CASH", "BURSARY", "GOVERNMENT", "LEGACY": return true }; return false }

func (h *PaymentHandler) ListForStudent(w http.ResponseWriter, r *http.Request) {
	studentID, err := parseTrailingID(r.URL.Path, "/api/payments/student/")
	if err != nil {
		http.Error(w, "invalid student id", http.StatusBadRequest)
		return
	}
	if err := requireStudentOrStaff(r, h.db, studentID); err != nil {
		http.Error(w, err.Error(), err.status)
		return
	}
	rows, err := h.db.Query(r.Context(), `SELECT id, student_id, academic_term_id, source, method, amount, transaction_reference, status, paid_at FROM tuition_payments WHERE student_id = $1 ORDER BY paid_at DESC`, studentID)
	if err != nil {
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	var payments []map[string]any
	for rows.Next() {
		var id, studentID int64
		var academicTermID *int64
		var source, method, transactionReference, status string
		var amount float64
		var paidAt any
		if err := rows.Scan(&id, &studentID, &academicTermID, &source, &method, &amount, &transactionReference, &status, &paidAt); err != nil {
			http.Error(w, "database error", http.StatusInternalServerError)
			return
		}
		payments = append(payments, map[string]any{"id": id, "student_id": studentID, "academic_term_id": academicTermID, "source": source, "method": method, "amount": amount, "transaction_reference": transactionReference, "status": status, "paid_at": paidAt})
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(payments)
}

func (h *PaymentHandler) Receipt(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(strings.TrimPrefix(r.URL.Path, "/api/payments/"), 10, 64)
	if err != nil || id <= 0 { http.Error(w, "invalid payment id", 400); return }
	var studentID int64; var amount float64; var source, method, reference, status string; var paidAt any
	if err := h.db.QueryRow(r.Context(), `SELECT student_id, amount, source, method, transaction_reference, status, paid_at FROM tuition_payments WHERE id=$1`, id).Scan(&studentID, &amount, &source, &method, &reference, &status, &paidAt); err != nil { http.Error(w, "payment not found", 404); return }
	if authErr := requireStudentOrStaff(r, h.db, studentID); authErr != nil { http.Error(w, authErr.Error(), authErr.status); return }
	w.Header().Set("Content-Type", "application/pdf"); w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=receipt-%d.pdf", id)); _ = sharedexport.WriteSimplePDF(w, "Tuition payment receipt", []string{fmt.Sprintf("Receipt: %d", id), fmt.Sprintf("Student: %d", studentID), fmt.Sprintf("Amount: %.2f", amount), "Source: "+source, "Method: "+method, "Reference: "+reference, "Status: "+status, fmt.Sprintf("Paid at: %v", paidAt)})
}

type authorizationError struct {
	message string
	status  int
}

func (e authorizationError) Error() string { return e.message }

func requireStudentOrStaff(r *http.Request, db *pgxpool.Pool, studentID int64) *authorizationError {
	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if err := identity.ParseSession(r, pgxSessionChecker{db: db}, token); err != nil {
		return &authorizationError{"authentication required", http.StatusUnauthorized}
	}
	claims, err := identity.ParseJWT(token, jwtSecret(), time.Now())
	if err != nil {
		return &authorizationError{"authentication required", http.StatusUnauthorized}
	}
	if claims.Role == "student" || containsRole(claims.Roles, "student") {
		self, err := strconv.ParseInt(claims.Subject, 10, 64)
		if err != nil || self != studentID {
			return &authorizationError{"forbidden", http.StatusForbidden}
		}
		return nil
	}
	if !identity.IsStaffRole(claims.Role) && !hasStaffRole(claims.Roles) {
		return &authorizationError{"forbidden", http.StatusForbidden}
	}
	return nil
}

func containsRole(roles []string, target string) bool {
	for _, role := range roles {
		if role == target {
			return true
		}
	}
	return false
}

func hasStaffRole(roles []string) bool {
	for _, role := range roles {
		if identity.IsStaffRole(role) {
			return true
		}
	}
	return false
}

func jwtSecret() string {
	return strings.TrimSpace(os.Getenv("JWT_SECRET"))
}

func parseTrailingID(path, prefix string) (int64, error) {
	trimmed := strings.TrimPrefix(path, prefix)
	trimmed = strings.Trim(trimmed, "/")
	if trimmed == "" {
		return 0, strconv.ErrSyntax
	}
	return strconv.ParseInt(trimmed, 10, 64)
}
