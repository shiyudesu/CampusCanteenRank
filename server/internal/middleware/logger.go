package middleware

import (
	"time"

	logpkg "CampusCanteenRank/server/internal/pkg/logger"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func RequestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		method := c.Request.Method

		c.Next()

		latency := time.Since(start)
		traceID := GetTraceID(c)
		userID, _ := c.Get("userId")

		requestID := traceID
		logpkg.L().Info("http request",
			zap.String("trace_id", traceID),
			zap.String("request_id", requestID),
			zap.Any("user_id", userID),
			zap.String("method", method),
			zap.String("path", path),
			zap.Int("status", c.Writer.Status()),
			zap.Duration("latency", latency),
			zap.String("client_ip", c.ClientIP()),
			zap.Any("query", logpkg.SanitizeQuery(c.Request.URL.Query())),
			zap.Any("headers", logpkg.SanitizeHeaders(c.Request.Header)),
		)
	}
}
