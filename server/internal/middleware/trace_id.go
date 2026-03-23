package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	TraceIDHeader     = "X-Trace-ID"
	traceIDContextKey = "traceId"
)

func TraceID() gin.HandlerFunc {
	return func(c *gin.Context) {
		traceID := c.GetHeader(TraceIDHeader)
		if !isValidTraceID(traceID) {
			traceID = newTraceID()
		}

		c.Set(traceIDContextKey, traceID)
		c.Writer.Header().Set(TraceIDHeader, traceID)
		c.Next()
	}
}

func GetTraceID(c *gin.Context) string {
	v, ok := c.Get(traceIDContextKey)
	if !ok {
		return ""
	}
	traceID, _ := v.(string)
	return traceID
}

func newTraceID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("trace-fallback-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

func isValidTraceID(traceID string) bool {
	if len(traceID) == 0 || len(traceID) > 128 {
		return false
	}
	for _, ch := range traceID {
		isLetter := (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z')
		isDigit := ch >= '0' && ch <= '9'
		isAllowedSymbol := ch == '-' || ch == '_' || ch == '.'
		if !isLetter && !isDigit && !isAllowedSymbol {
			return false
		}
	}
	return true
}
