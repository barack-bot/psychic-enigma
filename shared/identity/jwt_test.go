package identity

import (
	"context"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

type fakeSessionChecker struct {
	present bool
}

func (f fakeSessionChecker) SessionExists(ctx context.Context, tokenHash string) bool {
	return f.present
}

func TestParseSessionUsesSessionChecker(t *testing.T) {
	secret := "test-session-secret"
	if err := os.Setenv("JWT_SECRET", secret); err != nil {
		t.Fatalf("setenv: %v", err)
	}
	defer os.Unsetenv("JWT_SECRET")

	claims := Claims{Subject: "42", Role: RoleAcademicAdmin, Roles: []string{RoleAcademicAdmin}, Issued: time.Now().Unix(), Expires: time.Now().Add(time.Hour).Unix()}
	token, err := GenerateJWT(claims, secret)
	if err != nil {
		t.Fatalf("GenerateJWT: %v", err)
	}

	req := httptest.NewRequest("GET", "/", nil)
	if err := ParseSession(req, fakeSessionChecker{present: true}, token); err != nil {
		t.Fatalf("ParseSession returned error: %v", err)
	}
}
