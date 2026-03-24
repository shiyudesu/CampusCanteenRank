package middleware

import (
	"strings"

	authpkg "CampusCanteenRank/server/internal/pkg/auth"
	"github.com/gin-gonic/gin"
)

func OptionalAuth(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := strings.TrimSpace(c.GetHeader("Authorization"))
		if authHeader == "" {
			c.Next()
			return
		}
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.Next()
			return
		}
		claims, err := authpkg.ParseToken(secret, parts[1])
		if err != nil || claims.TokenType != authpkg.TokenTypeAccess {
			c.Next()
			return
		}
		c.Set("userId", claims.UserID)
		c.Next()
	}
}
