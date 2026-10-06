package identity

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

func TokenHash(token string) string { sum := sha256.Sum256([]byte(token)); return fmt.Sprintf("%x", sum) }

var ErrInvalidToken = errors.New("invalid token")

func GenerateJWT(claims Claims, secret string) (string, error) {
	if strings.TrimSpace(secret) == "" {
		return "", fmt.Errorf("jwt secret is not configured")
	}
	if claims.Issued == 0 {
		claims.Issued = time.Now().Unix()
	}
	if claims.Expires == 0 {
		claims.Expires = time.Now().Add(8 * time.Hour).Unix()
	}
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", fmt.Errorf("marshal jwt claims: %w", err)
	}
	signingInput := header + "." + base64.RawURLEncoding.EncodeToString(payload)
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(signingInput))
	signature := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return signingInput + "." + signature, nil
}

func ParseJWT(token, secret string, now time.Time) (Claims, error) {
	if strings.TrimSpace(secret) == "" {
		return Claims{}, fmt.Errorf("jwt secret is not configured")
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return Claims{}, ErrInvalidToken
	}
	var header struct {
		Algorithm string `json:"alg"`
	}
	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return Claims{}, ErrInvalidToken
	}
	if json.Unmarshal(headerBytes, &header) != nil || header.Algorithm != "HS256" {
		return Claims{}, ErrInvalidToken
	}
	provided, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return Claims{}, ErrInvalidToken
	}
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(parts[0] + "." + parts[1]))
	if !hmac.Equal(provided, mac.Sum(nil)) {
		return Claims{}, ErrInvalidToken
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return Claims{}, ErrInvalidToken
	}
	var claims Claims
	if json.Unmarshal(payload, &claims) != nil || claims.Expired(now) {
		return Claims{}, ErrInvalidToken
	}
	return claims, nil
}

type SessionChecker interface {
	SessionExists(ctx context.Context, tokenHash string) bool
}

func ParseSession(r *http.Request, checker SessionChecker, token string) error {
	if checker == nil || strings.TrimSpace(token) == "" {
		return ErrInvalidToken
	}
	secret := strings.TrimSpace(os.Getenv("JWT_SECRET"))
	if _, err := ParseJWT(token, secret, time.Now()); err != nil {
		return err
	}
	if !checker.SessionExists(r.Context(), TokenHash(token)) {
		return ErrInvalidToken
	}
	return nil
}
