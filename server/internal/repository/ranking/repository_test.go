package repository

import (
	"context"
	"testing"
)

func TestMemoryRankingRepositoryPaginationAndFilter(t *testing.T) {
	repo := NewMemoryRankingRepository()
	ctx := context.Background()

	first, hasMore, err := repo.ListRankings(ctx, RankingListOptions{
		Limit: 2,
		Filter: RankingFilter{
			Scope: "global",
			Days:  30,
			Sort:  "score_desc",
		},
	})
	if err != nil {
		t.Fatalf("list first page failed: %v", err)
	}
	if len(first) != 2 {
		t.Fatalf("first page len = %d, want 2", len(first))
	}
	if !hasMore {
		t.Fatalf("first page hasMore should be true")
	}

	last := first[len(first)-1]
	second, secondHasMore, err := repo.ListRankings(ctx, RankingListOptions{
		Limit: 2,
		Cursor: &RankingCursor{
			SortValue:    last.AvgRating,
			LastActiveAt: last.LastActiveAt,
			StallID:      last.StallID,
		},
		Filter: RankingFilter{Scope: "global", Days: 30, Sort: "score_desc"},
	})
	if err != nil {
		t.Fatalf("list second page failed: %v", err)
	}
	if len(second) == 0 {
		t.Fatalf("second page should have items")
	}
	if !secondHasMore {
		t.Fatalf("second page hasMore should be true with current seeds")
	}
	for _, item := range second {
		if item.StallID == first[0].StallID || item.StallID == first[1].StallID {
			t.Fatalf("duplicated item across pages: stallId=%d", item.StallID)
		}
	}

	filtered, _, err := repo.ListRankings(ctx, RankingListOptions{Limit: 20, Filter: RankingFilter{Scope: "canteen", ScopeID: 1, Days: 30, Sort: "score_desc"}})
	if err != nil {
		t.Fatalf("list canteen filter failed: %v", err)
	}
	if len(filtered) == 0 {
		t.Fatalf("filtered result should not be empty")
	}
	for _, item := range filtered {
		if item.CanteenID != 1 {
			t.Fatalf("unexpected canteenId=%d for canteen scope", item.CanteenID)
		}
	}
}
