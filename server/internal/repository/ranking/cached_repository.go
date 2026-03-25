package repository

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	model "CampusCanteenRank/server/internal/model/ranking"

	"github.com/redis/go-redis/v9"
)

type rankingCacheStore interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key string, value string, ttl time.Duration) error
	Increment(ctx context.Context, key string) (int64, error)
	SetNX(ctx context.Context, key string, value string, ttl time.Duration) (bool, error)
	ReleaseLock(ctx context.Context, key string, expectedValue string) error
}

type redisRankingCacheStore struct {
	client redis.Cmdable
}

func (s *redisRankingCacheStore) Get(ctx context.Context, key string) (string, error) {
	return s.client.Get(ctx, key).Result()
}

func (s *redisRankingCacheStore) Set(ctx context.Context, key string, value string, ttl time.Duration) error {
	return s.client.Set(ctx, key, value, ttl).Err()
}

func (s *redisRankingCacheStore) Increment(ctx context.Context, key string) (int64, error) {
	return s.client.Incr(ctx, key).Result()
}

func (s *redisRankingCacheStore) SetNX(ctx context.Context, key string, value string, ttl time.Duration) (bool, error) {
	return s.client.SetNX(ctx, key, value, ttl).Result()
}

func (s *redisRankingCacheStore) ReleaseLock(ctx context.Context, key string, expectedValue string) error {
	const releaseScript = `
if redis.call("get", KEYS[1]) == ARGV[1] then
  return redis.call("del", KEYS[1])
end
return 0
`
	return s.client.Eval(ctx, releaseScript, []string{key}, expectedValue).Err()
}

type cachedRankingRepository struct {
	next   RankingRepository
	cache  rankingCacheStore
	prefix string
	ttl    time.Duration

	lockTTL       time.Duration
	lockWait      time.Duration
	lockRetry     int
	versionKey    string
	lockKeyPrefix string
}

type cachedRankingPayload struct {
	Items   []model.RankingItem `json:"items"`
	HasMore bool                `json:"hasMore"`
}

const (
	defaultRankingVersion   = int64(0)
	defaultLockTTL          = 3 * time.Second
	defaultLockWaitInterval = 40 * time.Millisecond
	defaultLockRetryCount   = 3
)

func NewCachedRankingRepository(next RankingRepository, client redis.Cmdable, prefix string, ttl time.Duration) RankingRepository {
	if next == nil || client == nil {
		return next
	}
	if prefix == "" {
		prefix = "ranking:list"
	}
	if ttl <= 0 {
		ttl = 30 * time.Second
	}
	return &cachedRankingRepository{
		next:   next,
		cache:  &redisRankingCacheStore{client: client},
		prefix: prefix,
		ttl:    ttl,

		lockTTL:       defaultLockTTL,
		lockWait:      defaultLockWaitInterval,
		lockRetry:     defaultLockRetryCount,
		versionKey:    prefix + ":version",
		lockKeyPrefix: prefix + ":lock:",
	}
}

func newCachedRankingRepositoryWithStore(next RankingRepository, store rankingCacheStore, prefix string, ttl time.Duration) RankingRepository {
	if next == nil || store == nil {
		return next
	}
	if prefix == "" {
		prefix = "ranking:list"
	}
	if ttl <= 0 {
		ttl = 30 * time.Second
	}
	return &cachedRankingRepository{
		next:          next,
		cache:         store,
		prefix:        prefix,
		ttl:           ttl,
		lockTTL:       defaultLockTTL,
		lockWait:      defaultLockWaitInterval,
		lockRetry:     defaultLockRetryCount,
		versionKey:    prefix + ":version",
		lockKeyPrefix: prefix + ":lock:",
	}
}

func (r *cachedRankingRepository) ListRankings(ctx context.Context, options RankingListOptions) ([]model.RankingItem, bool, error) {
	version := r.currentVersion(ctx)
	key := r.cacheKey(version, options)
	if raw, err := r.cache.Get(ctx, key); err == nil {
		var payload cachedRankingPayload
		if jsonErr := json.Unmarshal([]byte(raw), &payload); jsonErr == nil {
			items := make([]model.RankingItem, len(payload.Items))
			copy(items, payload.Items)
			return items, payload.HasMore, nil
		}
	}

	lockToken := newLockToken()
	lockKey := r.lockKey(version, options)
	hasLock, lockErr := r.cache.SetNX(ctx, lockKey, lockToken, r.lockTTL)
	if lockErr == nil && hasLock {
		defer func() {
			_ = r.cache.ReleaseLock(ctx, lockKey, lockToken)
		}()

		if raw, err := r.cache.Get(ctx, key); err == nil {
			var payload cachedRankingPayload
			if jsonErr := json.Unmarshal([]byte(raw), &payload); jsonErr == nil {
				items := make([]model.RankingItem, len(payload.Items))
				copy(items, payload.Items)
				return items, payload.HasMore, nil
			}
		}
	} else {
		for i := 0; i < r.lockRetry; i++ {
			timer := time.NewTimer(r.lockWait)
			select {
			case <-ctx.Done():
				timer.Stop()
				return nil, false, ctx.Err()
			case <-timer.C:
			}

			if raw, err := r.cache.Get(ctx, key); err == nil {
				var payload cachedRankingPayload
				if jsonErr := json.Unmarshal([]byte(raw), &payload); jsonErr == nil {
					items := make([]model.RankingItem, len(payload.Items))
					copy(items, payload.Items)
					return items, payload.HasMore, nil
				}
			}
		}
	}

	items, hasMore, err := r.next.ListRankings(ctx, options)
	if err != nil {
		return nil, false, err
	}
	payload := cachedRankingPayload{Items: items, HasMore: hasMore}
	if raw, marshalErr := json.Marshal(payload); marshalErr == nil {
		_ = r.cache.Set(ctx, key, string(raw), r.ttl)
	}

	out := make([]model.RankingItem, len(items))
	copy(out, items)
	return out, hasMore, nil
}

func (r *cachedRankingRepository) cacheKey(version int64, options RankingListOptions) string {
	var sortValue float64
	var lastActive int64
	var cursorStallID int64
	if options.Cursor != nil {
		sortValue = options.Cursor.SortValue
		lastActive = options.Cursor.LastActiveAt.UTC().UnixNano()
		cursorStallID = options.Cursor.StallID
	}
	return fmt.Sprintf(
		"%s:data:v=%d:scope=%s:scopeId=%d:foodTypeId=%d:days=%d:sort=%s:limit=%d:cursorSort=%.10f:cursorLast=%d:cursorStall=%d",
		r.prefix,
		version,
		options.Filter.Scope,
		options.Filter.ScopeID,
		options.Filter.FoodTypeID,
		options.Filter.Days,
		options.Filter.Sort,
		options.Limit,
		sortValue,
		lastActive,
		cursorStallID,
	)
}

func (r *cachedRankingRepository) InvalidateRankingCache(ctx context.Context) error {
	if _, err := r.cache.Increment(ctx, r.versionKey); err != nil {
		return err
	}
	return nil
}

func (r *cachedRankingRepository) currentVersion(ctx context.Context) int64 {
	raw, err := r.cache.Get(ctx, r.versionKey)
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return defaultRankingVersion
		}
		return defaultRankingVersion
	}
	parsed, parseErr := strconv.ParseInt(raw, 10, 64)
	if parseErr != nil || parsed <= 0 {
		return defaultRankingVersion
	}
	return parsed
}

func (r *cachedRankingRepository) lockKey(version int64, options RankingListOptions) string {
	return r.lockKeyPrefix + r.cacheKey(version, options)
}

func newLockToken() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("fallback-%d", time.Now().UnixNano())
	}
	return fmt.Sprintf("%x", b)
}
