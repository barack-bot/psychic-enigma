package handler

import (
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"school-platform/shared/identity"
)

type AuthHandler struct {
	db        *sql.DB
	jwtSecret string
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type authResponse struct {
	Token     string `json:"token"`
	Role      string `json:"role"`
	ExpiresAt int64  `json:"expires_at"`
}

func NewAuthHandler(db *sql.DB, jwtSecret string) *AuthHandler {
	return &AuthHandler{db: db, jwtSecret: jwtSecret}
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(req.Username) == "" || strings.TrimSpace(req.Password) == "" {
		http.Error(w, "username and password are required", http.StatusBadRequest)
		return
	}
	var (
		id       int64
		username string
		hash     string
		role     string
	)
	if err := h.db.QueryRowContext(r.Context(), `SELECT id, username, password_hash, role FROM staff_users WHERE username = $1 AND status = 'ACTIVE'`, req.Username).Scan(&id, &username, &hash, &role); err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "invalid credentials", http.StatusUnauthorized)
			return
		}
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(req.Password)); err != nil {
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}
	issuedAt := time.Now().Unix()
	expiresAt := time.Now().Add(8 * time.Hour).Unix()
	claims := identity.Claims{
		Subject: strconv.FormatInt(id, 10),
		Role:    role,
		Roles:   []string{role},
		Issued:  issuedAt,
		Expires: expiresAt,
	}
	token, err := identity.GenerateJWT(claims, h.jwtSecret)
	if err != nil {
		http.Error(w, "could not issue token", http.StatusInternalServerError)
		return
	}
	tokenHash := sha256.Sum256([]byte(token))
	if _, err := h.db.ExecContext(r.Context(), `
		INSERT INTO auth_sessions (token_hash, principal_type, staff_user_id, login_origin, user_agent, client_ip, expires_at)
		VALUES ($1, 'staff', $2, 'web', $3, $4, to_timestamp($5))`,
		fmt.Sprintf("%x", tokenHash), id, r.UserAgent(), r.RemoteAddr, expiresAt,
	); err != nil {
		http.Error(w, "session could not be created", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(authResponse{Token: token, Role: role, ExpiresAt: expiresAt})
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if token != "" { _, _ = h.db.ExecContext(r.Context(), `DELETE FROM auth_sessions WHERE token_hash=$1`, identity.TokenHash(token)) }
	w.WriteHeader(http.StatusNoContent)
}
