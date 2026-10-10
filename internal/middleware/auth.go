package middleware

import (
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"caupocket-api/pkg/response"
)

// ContextKey kunci context Gin untuk user_id hasil decode JWT.
const ContextKeyUserID = "auth.user_id"

// AuthRequired memvalidasi header `Authorization: Bearer <token>`,
// memverifikasi signature JWT (HS256), lalu menyematkan user_id ke context.
func AuthRequired(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		const prefix = "Bearer "
		if !strings.HasPrefix(header, prefix) {
			response.Error(c, 401, "Header Authorization wajib berisi Bearer token", response.ErrCodeUnauthorized)
			c.Abort()
			return
		}

		tokenString := strings.TrimSpace(header[len(prefix):])
		token, err := jwt.Parse(tokenString, func(t *jwt.Token) (any, error) {
			return []byte(secret), nil
		}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
		if err != nil || !token.Valid {
			response.Error(c, 401, "Token tidak valid atau kadaluarsa", response.ErrCodeUnauthorized)
			c.Abort()
			return
		}

		subject, err := token.Claims.GetSubject()
		if err != nil {
			response.Error(c, 401, "Token tidak memuat user_id", response.ErrCodeUnauthorized)
			c.Abort()
			return
		}
		userID, err := uuid.Parse(subject)
		if err != nil {
			response.Error(c, 401, fmt.Sprintf("user_id pada token tidak valid: %v", err), response.ErrCodeUnauthorized)
			c.Abort()
			return
		}

		c.Set(ContextKeyUserID, userID)
		c.Next()
	}
}

// UserIDFrom mengambil user_id yang disematkan AuthRequired.
func UserIDFrom(c *gin.Context) (uuid.UUID, bool) {
	v, exists := c.Get(ContextKeyUserID)
	if !exists {
		return uuid.Nil, false
	}
	id, ok := v.(uuid.UUID)
	return id, ok
}
