package middleware

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"time"

	"school-platform/shared/identity"
)

type sqlSessionChecker struct{ db *sql.DB }

func (c sqlSessionChecker) SessionExists(ctx context.Context, tokenHash string) bool {
	if c.db == nil {
		return false
	}
	var exists bool
	if err := c.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM auth_sessions WHERE token_hash=$1 AND expires_at > CURRENT_TIMESTAMP)`, tokenHash).Scan(&exists); err != nil {
		return false
	}
	return exists
}

func writeAuthError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"error": msg, "login_url": "/students.html"})
}

func RequireAdmin(db *sql.DB, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/healthz" {
			next.ServeHTTP(w, r)
			return
		}
		secret := strings.TrimSpace(os.Getenv("JWT_SECRET")); token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "); claims, err := identity.ParseJWT(token, secret, time.Now()); if err == nil { err = identity.ParseSession(r, sqlSessionChecker{db: db}, token) }
		if err != nil || !claims.HasAdminRole() {
			writeAuthError(w, http.StatusUnauthorized, "administrator authorization required")
			return
		}
		next.ServeHTTP(w, r)
	})
}
