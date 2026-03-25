package repository

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	model "CampusCanteenRank/server/internal/model/ranking"
)

type fakeRankingStore struct {
	data map[string]string
	err  error

	sets int
	gets int
	incs int

	setNXResult bool
	setNXErr    error
	lockValues  map[string]string

	afterGet func(store *fakeRankingStore, key string)
}

func (s *fakeRankingStore) Get(_ context.Context, key string) (string, error) {
	s.gets++
	if s.afterGet != nil {
		s.afterGet(s, key)
	}
	if s.err != nil {
		return "", s.err
	}
	value, ok := s.data[key]
	if !ok {
		return "", errors.New("missing")
	}
	return value, nil
}

func (s *fakeRankingStore) Set(_ context.Context, key string, value string, _ time.Duration) error {
	s.sets++
	if s.data == nil {
		s.data = map[string]string{}
	}
	s.data[key] = value
	return nil
}

func (s *fakeRankingStore) Increment(_ context.Context, key string) (int64, error) {
	s.incs++
	if s.data == nil {
		s.data = map[string]string{}
	}
	current := int64(0)
	if raw, ok := s.data[key]; ok {
		var parsed int64
		if _, err := fmt.Sscanf(raw, "%d", &parsed); err == nil {
			current = parsed
		}
	}
	next := current + 1
	s.data[key] = fmt.Sprintf("%d", next)
	return next, nil
}

func (s *fakeRankingStore) SetNX(_ context.Context, key string, value string, _ time.Duration) (bool, error) {
	if s.setNXErr != nil {
		return false, s.setNXErr
	}
	if s.lockValues == nil {
		s.lockValues = map[string]string{}
	}
	if s.setNXResult {
		s.lockValues[key] = value
		return true, nil
	}
	if _, exists := s.lockValues[key]; exists {
		return false, nil
	}
	return false, nil
}

func (s *fakeRankingStore) ReleaseLock(_ context.Context, key string, expectedValue string) error {
	if s.lockValues == nil {
		return nil
	}
	if value, exists := s.lockValues[key]; exists && value == expectedValue {
		delete(s.lockValues, key)
	}
	return nil
}

type fakeRankingRepo struct {
	items []model.RankingItem
	calls int
}

func (r *fakeRankingRepo) ListRankings(_ context.Context, _ RankingListOptions) ([]model.RankingItem, bool, error) {
	r.calls++
	out := make([]model.RankingItem, len(r.items))
	copy(out, r.items)
	return out, false, nil
}

func TestNewCachedRankingRepositoryFallback(t *testing.T) {
	next := &fakeRankingRepo{items: []model.RankingItem{{StallID: 101, StallName: "A"}}}
	if got := NewCachedRankingRepository(next, nil, "", 0); got != next {
		t.Fatalf("expected fallback to next repository when client is nil")
	}
}

func TestCachedRankingRepositoryCachesResult(t *testing.T) {
	store := &fakeRankingStore{setNXResult: true}
	base := &fakeRankingRepo{items: []model.RankingItem{{StallID: 101, StallName: "A"}}}
	repo := newCachedRankingRepositoryWithStore(base, store, "ranking:test", time.Minute)

	opts := RankingListOptions{Limit: 20, Filter: RankingFilter{Scope: "global", Days: 30, Sort: "score_desc"}}

	first, firstHasMore, err := repo.ListRankings(context.Background(), opts)
	if err != nil {
		t.Fatalf("first list rankings failed: %v", err)
	}
	if firstHasMore {
		t.Fatalf("first hasMore should be false")
	}
	if len(first) != 1 || first[0].StallID != 101 {
		t.Fatalf("first result mismatch: %+v", first)
	}
	if base.calls != 1 {
		t.Fatalf("base repo calls = %d, want 1", base.calls)
	}

	second, secondHasMore, err := repo.ListRankings(context.Background(), opts)
	if err != nil {
		t.Fatalf("second list rankings failed: %v", err)
	}
	if secondHasMore {
		t.Fatalf("second hasMore should be false")
	}
	if len(second) != 1 || second[0].StallID != 101 {
		t.Fatalf("second result mismatch: %+v", second)
	}
	if base.calls != 1 {
		t.Fatalf("base repo should be called once due to cache hit, got %d", base.calls)
	}
	if store.sets == 0 {
		t.Fatalf("cache set should be called at least once")
	}
	if store.gets < 2 {
		t.Fatalf("cache get should be called for both reads")
	}
}

func TestCachedRankingRepositoryInvalidatePrefix(t *testing.T) {
	store := &fakeRankingStore{data: map[string]string{"other:key": "y"}, setNXResult: true}
	base := &fakeRankingRepo{items: []model.RankingItem{{StallID: 101, StallName: "A"}}}
	repo := newCachedRankingRepositoryWithStore(base, store, "ranking:test", time.Minute)

	cachedRepo, ok := repo.(*cachedRankingRepository)
	if !ok {
		t.Fatalf("repo should be *cachedRankingRepository")
	}

	if err := cachedRepo.InvalidateRankingCache(context.Background()); err != nil {
		t.Fatalf("invalidate ranking cache failed: %v", err)
	}
	if store.incs != 1 {
		t.Fatalf("version increment calls = %d, want 1", store.incs)
	}
	if got := store.data["ranking:test:version"]; got != "1" {
		t.Fatalf("version value = %s, want 1", got)
	}
	if _, exists := store.data["other:key"]; !exists {
		t.Fatalf("unrelated key should remain")
	}
}

func TestCachedRankingRepositoryWaitsForLockOwnerPopulate(t *testing.T) {
	store := &fakeRankingStore{setNXResult: false, data: map[string]string{}}
	base := &fakeRankingRepo{items: []model.RankingItem{{StallID: 501, StallName: "fallback"}}}
	repo := newCachedRankingRepositoryWithStore(base, store, "ranking:test", time.Minute)

	cachedRepo, ok := repo.(*cachedRankingRepository)
	if !ok {
		t.Fatalf("repo should be *cachedRankingRepository")
	}
	cachedRepo.lockRetry = 2
	cachedRepo.lockWait = 2 * time.Millisecond

	opts := RankingListOptions{Limit: 20, Filter: RankingFilter{Scope: "global", Days: 30, Sort: "score_desc"}}
	cacheKey := cachedRepo.cacheKey(defaultRankingVersion, opts)
	misses := 0
	store.afterGet = func(s *fakeRankingStore, key string) {
		if key != cacheKey {
			return
		}
		if _, exists := s.data[key]; exists {
			return
		}
		misses++
		if misses == 2 {
			s.data[key] = `{"items":[{"stallId":777,"stallName":"from-cache"}],"hasMore":false}`
		}
	}

	items, hasMore, err := repo.ListRankings(context.Background(), opts)
	if err != nil {
		t.Fatalf("list rankings failed: %v", err)
	}
	if hasMore {
		t.Fatalf("hasMore should be false")
	}
	if len(items) != 1 || items[0].StallID != 777 {
		t.Fatalf("items should come from populated cache, got %+v", items)
	}
	if base.calls != 0 {
		t.Fatalf("base repo should not be called when cache gets populated by lock owner, got %d", base.calls)
	}
}
