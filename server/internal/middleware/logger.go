package middleware

import (
	"log"
	"time"

	"github.com/gin-gonic/gin"
)

func RequestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		method := c.Request.Method

		c.Next()

		latency := time.Since(start)
		traceID := GetTraceID(c)
		log.Printf("trace_id=%s method=%s path=%s status=%d latency=%s client_ip=%s", traceID, method, path, c.Writer.Status(), latency, c.ClientIP())
	}
}
