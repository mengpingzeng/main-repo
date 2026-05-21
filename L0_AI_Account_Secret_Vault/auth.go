package vault

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type claims struct {
	UID      string `json:"uid"`
	Username string `json:"username"`
	Role     string `json:"role"`
	Exp      int64  `json:"exp"`
	Iat      int64  `json:"iat"`
}

func GenerateToken(uid, username, role, secret string) (string, error) {
	now := time.Now().UTC()
	c := claims{
		UID:      uid,
		Username: username,
		Role:     role,
		Iat:      now.Unix(),
		Exp:      now.Add(7 * 24 * time.Hour).Unix(),
	}

	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	payloadBytes, err := json.Marshal(c)
	if err != nil {
		return "", fmt.Errorf("marshal claims: %w", err)
	}
	payload := base64.RawURLEncoding.EncodeToString(payloadBytes)

	signingInput := header + "." + payload
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(signingInput))
	signature := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))

	return signingInput + "." + signature, nil
}

func VerifyToken(tokenStr, secret string) (*claims, error) {
	parts := strings.SplitN(tokenStr, ".", 3)
	if len(parts) != 3 {
		return nil, ErrInvalidToken
	}

	header := parts[0]
	payload := parts[1]
	sig := parts[2]

	signingInput := header + "." + payload
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(signingInput))
	expectedSig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(sig), []byte(expectedSig)) {
		return nil, ErrInvalidToken
	}

	payloadBytes, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil {
		return nil, ErrInvalidToken
	}

	var c claims
	if err := json.Unmarshal(payloadBytes, &c); err != nil {
		return nil, ErrInvalidToken
	}

	if time.Now().UTC().Unix() > c.Exp {
		return nil, ErrInvalidToken
	}

	return &c, nil
}

type contextKey string

const authClaimsKey contextKey = "auth_claims"

type CustomClaims struct {
	UID      string `json:"uid"`
	Username string `json:"username"`
	Role     string `json:"role"`
	Iat      int64  `json:"iat"`
}

func SetAuthContext(ctx context.Context, c interface{}) context.Context {
	return context.WithValue(ctx, authClaimsKey, c)
}

func SetAuthUID(ctx context.Context, uid string) context.Context {
	return context.WithValue(ctx, authClaimsKey, &CustomClaims{UID: uid})
}

func GetAuthClaims(ctx context.Context) *CustomClaims {
	if c, ok := ctx.Value(authClaimsKey).(*CustomClaims); ok {
		return c
	}
	if c, ok := ctx.Value(authClaimsKey).(*claims); ok {
		return &CustomClaims{UID: c.UID, Username: c.Username, Role: c.Role, Iat: c.Iat}
	}
	return nil
}

func GetAuthUID(ctx context.Context) string {
	c := GetAuthClaims(ctx)
	if c == nil {
		return ""
	}
	return c.UID
}

func AuthMiddleware(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				writeAuthError(w, "MISSING_TOKEN", "authorization header is required")
				return
			}

			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
				writeAuthError(w, "INVALID_TOKEN", "authorization header must be Bearer <token>")
				return
			}

			c, err := VerifyToken(parts[1], secret)
			if err != nil {
				writeAuthError(w, "INVALID_TOKEN", "invalid or expired token")
				return
			}

			ctx := SetAuthContext(r.Context(), c)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func writeAuthError(w http.ResponseWriter, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	json.NewEncoder(w).Encode(map[string]string{
		"code":    code,
		"message": message,
	})
}
