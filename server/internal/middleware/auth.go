package middleware

import (
	"net/http"
	"strings"

	authpkg "CampusCanteenRank/server/internal/pkg/auth"
	errpkg "CampusCanteenRank/server/internal/pkg/errors"
	"CampusCanteenRank/server/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

func Auth(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			response.Fail(c, http.StatusUnauthorized, errpkg.CodeUnauthorized, "unauthorized")
			c.Abort()
			return
		}

		claims, err := authpkg.ParseToken(secret, parts[1])
		if err != nil || claims.TokenType != authpkg.TokenTypeAccess {
			response.Fail(c, http.StatusUnauthorized, errpkg.CodeUnauthorized, "unauthorized")
			c.Abort()
			return
		}

		c.Set("userId", claims.UserID)
		c.Next()
	}
}
