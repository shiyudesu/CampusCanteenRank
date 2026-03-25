package middleware

import (
	"fmt"
	"sync"
	"time"

	errpkg "CampusCanteenRank/server/internal/pkg/errors"
	"CampusCanteenRank/server/internal/pkg/response"

	"github.com/gin-gonic/gin"
)

type rateBucket struct {
	windowStart time.Time
	count       int
	lastSeen    time.Time
}

var (
	rateLimiterMu sync.Mutex
	rateBuckets   = map[string]rateBucket{}
	lastCleanupAt time.Time
)

func RateLimitByClient(routeKey string, maxRequests int, window time.Duration) gin.HandlerFunc {
	if maxRequests <= 0 {
		maxRequests = 30
	}
	if window <= 0 {
		window = time.Minute
	}

	return func(c *gin.Context) {
		clientKey := c.ClientIP()
		if clientKey == "" {
			clientKey = "unknown"
		}
		bucketKey := fmt.Sprintf("%s|%s", routeKey, clientKey)
		now := time.Now().UTC()

		rateLimiterMu.Lock()
		bucket := rateBuckets[bucketKey]
		if bucket.windowStart.IsZero() || now.Sub(bucket.windowStart) >= window {
			bucket = rateBucket{windowStart: now, count: 0, lastSeen: now}
		}
		bucket.count++
		bucket.lastSeen = now
		rateBuckets[bucketKey] = bucket
		if lastCleanupAt.IsZero() || now.Sub(lastCleanupAt) >= window {
			for key, existing := range rateBuckets {
				if now.Sub(existing.lastSeen) >= window {
					delete(rateBuckets, key)
				}
			}
			lastCleanupAt = now
		}
		allowed := bucket.count <= maxRequests
		rateLimiterMu.Unlock()

		if !allowed {
			response.Fail(c, 429, errpkg.CodeTooMany, "too many requests")
			c.Abort()
			return
		}

		c.Next()
	}
}
