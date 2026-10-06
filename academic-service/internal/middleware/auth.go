package middleware

import (
	"context"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
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

func RequireStaff(db *pgxpool.Pool, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/healthz" {
			next.ServeHTTP(w, r)
			return
		}
		token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "); secret := jwtSecret(); claims, err := identity.ParseJWT(token, secret, time.Now()); if err == nil { err = identity.ParseSession(r, pgxSessionChecker{db: db}, token) }
		if err != nil || claims.Subject == "" || !identity.IsStaffRole(claims.Role) {
			http.Error(w, "staff authorization required", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func jwtSecret() string {
	return strings.TrimSpace(os.Getenv("JWT_SECRET"))
}
