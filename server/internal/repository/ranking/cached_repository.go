package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	model "CampusCanteenRank/server/internal/model/ranking"
	"github.com/redis/go-redis/v9"
)

type rankingCacheStore interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key string, value string, ttl time.Duration) error
	DeleteByPrefix(ctx context.Context, prefix string) error
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

func (s *redisRankingCacheStore) DeleteByPrefix(ctx context.Context, prefix string) error {
	var cursor uint64
	for {
		keys, nextCursor, err := s.client.Scan(ctx, cursor, prefix+"*", 100).Result()
		if err != nil {
			return err
		}
		if len(keys) > 0 {
			if err := s.client.Del(ctx, keys...).Err(); err != nil {
				return err
			}
		}
		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}
	return nil
}

type cachedRankingRepository struct {
	next   RankingRepository
	cache  rankingCacheStore
	prefix string
	ttl    time.Duration
}

type cachedRankingPayload struct {
	Items   []model.RankingItem `json:"items"`
	HasMore bool                `json:"hasMore"`
}

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
	return &cachedRankingRepository{next: next, cache: store, prefix: prefix, ttl: ttl}
}

func (r *cachedRankingRepository) ListRankings(ctx context.Context, options RankingListOptions) ([]model.RankingItem, bool, error) {
	key := r.cacheKey(options)
	if raw, err := r.cache.Get(ctx, key); err == nil {
		var payload cachedRankingPayload
		if jsonErr := json.Unmarshal([]byte(raw), &payload); jsonErr == nil {
			items := make([]model.RankingItem, len(payload.Items))
			copy(items, payload.Items)
			return items, payload.HasMore, nil
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

func (r *cachedRankingRepository) cacheKey(options RankingListOptions) string {
	var sortValue float64
	var lastActive int64
	var cursorStallID int64
	if options.Cursor != nil {
		sortValue = options.Cursor.SortValue
		lastActive = options.Cursor.LastActiveAt.UTC().UnixNano()
		cursorStallID = options.Cursor.StallID
	}
	return fmt.Sprintf(
		"%s:scope=%s:scopeId=%d:foodTypeId=%d:days=%d:sort=%s:limit=%d:cursorSort=%.10f:cursorLast=%d:cursorStall=%d",
		r.prefix,
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
	return r.cache.DeleteByPrefix(ctx, r.prefix+":")
}
