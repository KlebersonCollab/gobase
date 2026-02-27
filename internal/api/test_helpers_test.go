package api_test

import (
	"gobase/internal/auth"

	"github.com/golang-jwt/jwt/v5"
)

func createTestToken() string {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":       "00000000-0000-0000-0000-000000000000",
		"email":     "test@gobase.com",
		"tenant_id": "00000000-0000-0000-0000-000000000000",
	})
	tokenString, _ := token.SignedString(auth.JwtSecret)
	return tokenString
}
