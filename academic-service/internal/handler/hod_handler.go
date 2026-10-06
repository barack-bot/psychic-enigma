package handler

import (
	"encoding/json"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"school-platform/academic-service/internal/service"
	"school-platform/shared/identity"
)

type HODHandler struct {
	db       *pgxpool.Pool
	approval *service.MarkApprovalService
}

func NewHODHandler(db *pgxpool.Pool, approval *service.MarkApprovalService) *HODHandler {
	return &HODHandler{db: db, approval: approval}
}

func (h *HODHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("/api/hod/approve", h.approve)
	mux.HandleFunc("/api/hod/reject", h.reject)
}

type hodDecision struct {
	MarkID          int64  `json:"mark_id"`
	RejectionReason string `json:"rejection_reason"`
}

func (h *HODHandler) actorID(r *http.Request) (int64, bool) {
	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if err := identity.ParseSession(r, pgxSessionChecker{db: h.db}, token); err != nil {
		return 0, false
	}
	claims, err := identity.ParseJWT(token, strings.TrimSpace(os.Getenv("JWT_SECRET")), time.Now())
	if err != nil || claims.Role != identity.RoleHOD { return 0, false }
	id, err := strconv.ParseInt(claims.Subject, 10, 64)
	return id, err == nil
}

func (h *HODHandler) approve(w http.ResponseWriter, r *http.Request) {
	var in hodDecision
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	actorID, ok := h.actorID(r)
	if !ok { http.Error(w, "HOD authorization required", http.StatusForbidden); return }
	if err := h.approval.Approve(r.Context(), in.MarkID, actorID); err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (h *HODHandler) reject(w http.ResponseWriter, r *http.Request) {
	var in hodDecision
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	actorID, ok := h.actorID(r)
	if !ok { http.Error(w, "HOD authorization required", http.StatusForbidden); return }
	if err := h.approval.Reject(r.Context(), in.MarkID, actorID, in.RejectionReason); err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}
	w.WriteHeader(http.StatusOK)
}
