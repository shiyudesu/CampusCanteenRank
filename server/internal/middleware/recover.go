package middleware

import (
	"net/http"
	"runtime/debug"

	errpkg "CampusCanteenRank/server/internal/pkg/errors"
	logpkg "CampusCanteenRank/server/internal/pkg/logger"
	"CampusCanteenRank/server/internal/pkg/response"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func Recover() gin.HandlerFunc {
	return gin.CustomRecovery(func(c *gin.Context, recovered any) {
		traceID := GetTraceID(c)
		requestID := traceID
		userID, _ := c.Get("userId")
		logpkg.L().Error("panic recovered",
			zap.String("trace_id", traceID),
			zap.String("request_id", requestID),
			zap.Any("user_id", userID),
			zap.Any("panic", recovered),
			zap.String("stack", string(debug.Stack())),
		)
		response.Fail(c, http.StatusInternalServerError, errpkg.CodeInternal, "internal error")
		c.Abort()
	})
}
