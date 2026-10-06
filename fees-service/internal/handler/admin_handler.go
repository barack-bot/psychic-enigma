package handler

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
	"school-platform/fees-service/internal/domain"
	"school-platform/fees-service/internal/repository/postgres"
	"school-platform/fees-service/internal/service"
	"school-platform/shared/identity"
)

// AdminHandler exposes finance-admin endpoints: rates, waivers, term billing.
type AdminHandler struct {
	db      *pgxpool.Pool
	tuition *service.TuitionService
	chargeStore *postgres.ChargeRepository
}

func NewAdminHandler(db *pgxpool.Pool) *AdminHandler {
	return &AdminHandler{db: db, tuition: service.NewTuitionService(postgres.NewTuitionRepository(db)), chargeStore: postgres.NewChargeRepository(db)}
}

func (h *AdminHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("/api/admin/rates", h.rates)
	mux.HandleFunc("/api/admin/waivers", h.waivers)
	mux.HandleFunc("/api/admin/charges", h.charges)
	mux.HandleFunc("/api/admin/students/", h.issuePIN)
	mux.HandleFunc("/api/admin/collections", h.collections)
	mux.HandleFunc("/api/admin/unpaid", h.unpaid)
}

func (h *AdminHandler) rates(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		rates, err := h.tuition.ListRates(r.Context())
		if err != nil { http.Error(w, "database error", http.StatusInternalServerError); return }
		writeAdminJSON(w, http.StatusOK, rates)
	case http.MethodPut, http.MethodPost:
		var input struct { AcademicTermID int64 `json:"academic_term_id"`; Amount float64 `json:"amount"` }
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil { http.Error(w, "invalid JSON", http.StatusBadRequest); return }
		if err := h.tuition.SetRate(r.Context(), input.AcademicTermID, input.Amount); err != nil { http.Error(w, err.Error(), http.StatusBadRequest); return }
		writeAdminJSON(w, http.StatusOK, map[string]any{"academic_term_id": input.AcademicTermID, "amount": input.Amount})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *AdminHandler) waivers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost { http.Error(w, "method not allowed", http.StatusMethodNotAllowed); return }
	var input struct { StudentID int64 `json:"student_id"`; AcademicTermID *int64 `json:"academic_term_id"`; Sponsor string `json:"sponsor"`; Amount float64 `json:"amount"`; Description string `json:"description"` }
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil { http.Error(w, "invalid JSON", http.StatusBadRequest); return }
	if input.StudentID <= 0 || input.Amount <= 0 || strings.TrimSpace(input.Sponsor) == "" { http.Error(w, "student_id, sponsor and a positive amount are required", http.StatusBadRequest); return }
	var id int64
	err := h.db.QueryRow(r.Context(), `INSERT INTO student_fee_waivers (student_id, academic_term_id, sponsor, amount, description) VALUES ($1,$2,$3,$4,NULLIF($5,'')) RETURNING id`, input.StudentID, input.AcademicTermID, input.Sponsor, input.Amount, input.Description).Scan(&id)
	if err != nil { http.Error(w, "waiver could not be created", http.StatusConflict); return }
	writeAdminJSON(w, http.StatusCreated, map[string]any{"id": id})
}

func (h *AdminHandler) charges(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost { http.Error(w, "method not allowed", http.StatusMethodNotAllowed); return }
	var input domain.Charge
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil { http.Error(w, "invalid JSON", http.StatusBadRequest); return }
	if input.StudentID <= 0 || input.AcademicTermID <= 0 || input.Amount <= 0 { http.Error(w, "student_id, academic_term_id and a positive amount are required", http.StatusBadRequest); return }
	id, err := h.chargeStore.Create(r.Context(), input)
	if err != nil { http.Error(w, "charge could not be created", http.StatusConflict); return }
	writeAdminJSON(w, http.StatusCreated, map[string]any{"id": id})
}

func (h *AdminHandler) issuePIN(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	pathParts := strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/admin/students/"), "/"), "/")
	studentID, err := strconv.ParseInt(pathParts[0], 10, 64)
	if err != nil || studentID <= 0 || len(pathParts) != 2 || pathParts[1] != "pin" {
		http.Error(w, "invalid student id", http.StatusBadRequest)
		return
	}
	value, err := rand.Int(rand.Reader, big.NewInt(900000))
	if err != nil {
		http.Error(w, "could not generate PIN", http.StatusInternalServerError)
		return
	}
	pin := fmt.Sprintf("%06d", value.Int64()+100000)
	hash, err := bcrypt.GenerateFromPassword([]byte(pin), bcrypt.DefaultCost)
	if err != nil {
		http.Error(w, "could not secure PIN", http.StatusInternalServerError)
		return
	}
	result, err := h.db.Exec(r.Context(), `UPDATE students SET portal_pin_hash = $1 WHERE student_id = $2 AND deleted_at IS NULL`, string(hash), studentID)
	if err != nil || result.RowsAffected() == 0 {
		http.Error(w, "student not found", http.StatusNotFound)
		return
	}
	actorID := int64(0)
	if token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "); token != "" {
		if claims, parseErr := identity.ParseJWT(token, strings.TrimSpace(os.Getenv("JWT_SECRET")), time.Now()); parseErr == nil {
			if parsed, convErr := strconv.ParseInt(claims.Subject, 10, 64); convErr == nil {
				actorID = parsed
			}
		}
	}
	if actorID > 0 {
		_, _ = h.db.Exec(r.Context(), `INSERT INTO audit_logs (actor_type, actor_id, action, target_type, target_id, details) VALUES ('staff', $1, 'portal_pin_issued', 'student', $2, jsonb_build_object('student_id', $2, 'pin_last4', right($3, 4)))`, actorID, studentID, pin)
	}
	writeAdminJSON(w, http.StatusOK, map[string]any{"student_id": studentID, "pin": pin})
}

func (h *AdminHandler) collections(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet { http.Error(w, "method not allowed", http.StatusMethodNotAllowed); return }
	rows, err := h.db.Query(r.Context(), `SELECT id, student_id, student_name, admission_number, amount, method, source, transaction_reference, received_by_name, paid_at FROM payments_today ORDER BY paid_at DESC`)
	if err != nil { http.Error(w, "database error", http.StatusInternalServerError); return }
	defer rows.Close()
	var today []map[string]any
	for rows.Next() {
		var id, studentID int64
		var studentName, admissionNumber, method, source, transactionRef, receivedByName string
		var amount float64
		var paidAt time.Time
		if err := rows.Scan(&id, &studentID, &studentName, &admissionNumber, &amount, &method, &source, &transactionRef, &receivedByName, &paidAt); err != nil { http.Error(w, "database error", http.StatusInternalServerError); return }
		today = append(today, map[string]any{"id": id, "student_id": studentID, "student_name": studentName, "admission_number": admissionNumber, "amount": amount, "method": method, "source": source, "transaction_reference": transactionRef, "received_by_name": receivedByName, "paid_at": paidAt.Format(time.RFC3339)})
	}
	summaryRows, err := h.db.Query(r.Context(), `SELECT collection_date, method, payment_count, total_amount FROM daily_collections_summary WHERE collection_date = CURRENT_DATE ORDER BY method`)
	if err != nil { http.Error(w, "database error", http.StatusInternalServerError); return }
	defer summaryRows.Close()
	var summary []map[string]any
	for summaryRows.Next() {
		var date, method string
		var paymentCount int
		var totalAmount float64
		if err := summaryRows.Scan(&date, &method, &paymentCount, &totalAmount); err != nil { http.Error(w, "database error", http.StatusInternalServerError); return }
		summary = append(summary, map[string]any{"collection_date": date, "method": method, "payment_count": paymentCount, "total_amount": totalAmount})
	}
	writeAdminJSON(w, http.StatusOK, map[string]any{"today": today, "summary": summary})
}

func (h *AdminHandler) unpaid(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet { http.Error(w, "method not allowed", http.StatusMethodNotAllowed); return }
	rows, err := h.db.Query(r.Context(), `SELECT student_id, name, COALESCE(admission_number,''), total_charges, total_waived, total_paid, balance FROM unpaid_students ORDER BY balance DESC LIMIT 200`)
	if err != nil { http.Error(w, "database error", http.StatusInternalServerError); return }
	defer rows.Close()
	var result []map[string]any
	for rows.Next() { var id int64; var name, admission string; var charges, waived, paid, balance float64; if err := rows.Scan(&id, &name, &admission, &charges, &waived, &paid, &balance); err != nil { http.Error(w, "database error", 500); return }; result = append(result, map[string]any{"student_id": id, "name": name, "admission_number": admission, "total_charges": charges, "total_waived": waived, "total_paid": paid, "balance": balance}) }
	writeAdminJSON(w, http.StatusOK, result)
}

func writeAdminJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}