package middleware

import (
	"log"
	"net/http"
	"runtime/debug"

	errpkg "CampusCanteenRank/server/internal/pkg/errors"
	"CampusCanteenRank/server/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

func Recover() gin.HandlerFunc {
	return gin.CustomRecovery(func(c *gin.Context, recovered any) {
		traceID := GetTraceID(c)
		log.Printf("trace_id=%s panic=%v stack=%s", traceID, recovered, string(debug.Stack()))
		response.Fail(c, http.StatusInternalServerError, errpkg.CodeInternal, "internal error")
		c.Abort()
	})
}
