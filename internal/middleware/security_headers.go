package middleware

import (
	"github.com/gin-gonic/gin"
)

// SecurityHeaders menambahkan header keamanan standar pada semua response API
// (temuan DAST: sebelumnya tidak ada).
func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.Writer.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		h.Set("X-XSS-Protection", "0") // legacy auditor dinonaktifkan (rekomendasi modern)
		if c.Request.TLS != nil {
			h.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		c.Next()
	}
}
