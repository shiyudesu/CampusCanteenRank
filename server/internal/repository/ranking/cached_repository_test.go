package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	model "CampusCanteenRank/server/internal/model/ranking"
)

type fakeRankingStore struct {
	data map[string]string
	err  error
	sets int
	gets int
	dels int
}

func (s *fakeRankingStore) Get(_ context.Context, key string) (string, error) {
	s.gets++
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

func (s *fakeRankingStore) DeleteByPrefix(_ context.Context, prefix string) error {
	s.dels++
	if s.data == nil {
		return nil
	}
	for key := range s.data {
		if len(key) >= len(prefix) && key[:len(prefix)] == prefix {
			delete(s.data, key)
		}
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
	store := &fakeRankingStore{}
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
	store := &fakeRankingStore{data: map[string]string{
		"ranking:test:scope=global": "x",
		"other:key":                 "y",
	}}
	base := &fakeRankingRepo{items: []model.RankingItem{{StallID: 101, StallName: "A"}}}
	repo := newCachedRankingRepositoryWithStore(base, store, "ranking:test", time.Minute)

	cachedRepo, ok := repo.(*cachedRankingRepository)
	if !ok {
		t.Fatalf("repo should be *cachedRankingRepository")
	}

	if err := cachedRepo.InvalidateRankingCache(context.Background()); err != nil {
		t.Fatalf("invalidate ranking cache failed: %v", err)
	}
	if store.dels != 1 {
		t.Fatalf("delete by prefix calls = %d, want 1", store.dels)
	}
	if _, exists := store.data["ranking:test:scope=global"]; exists {
		t.Fatalf("ranking cache key should be deleted")
	}
	if _, exists := store.data["other:key"]; !exists {
		t.Fatalf("non-matching key should remain")
	}
}
