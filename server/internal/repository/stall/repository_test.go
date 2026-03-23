package repository

import (
	"context"
	"testing"
)

func TestMemoryStallRepositoryListUserRatingsPagination(t *testing.T) {
	repo := NewMemoryStallRepository()
	ctx := context.Background()

	if _, err := repo.UpsertUserRating(ctx, 1001, 101, 5); err != nil {
		t.Fatalf("upsert first rating failed: %v", err)
	}
	if _, err := repo.UpsertUserRating(ctx, 1001, 102, 3); err != nil {
		t.Fatalf("upsert second rating failed: %v", err)
	}
	if _, err := repo.UpsertUserRating(ctx, 1001, 201, 4); err != nil {
		t.Fatalf("upsert third rating failed: %v", err)
	}

	first, hasMore, err := repo.ListUserRatings(ctx, 1001, 2, nil)
	if err != nil {
		t.Fatalf("list first page failed: %v", err)
	}
	if len(first) != 2 {
		t.Fatalf("first page len = %d, want 2", len(first))
	}
	if !hasMore {
		t.Fatalf("first page hasMore should be true")
	}
	seen := map[int64]struct{}{}
	for _, item := range first {
		seen[item.StallID] = struct{}{}
	}

	last := first[len(first)-1]
	second, secondHasMore, err := repo.ListUserRatings(ctx, 1001, 2, &UserRatingCursor{UpdatedAt: last.UpdatedAt, StallID: last.StallID})
	if err != nil {
		t.Fatalf("list second page failed: %v", err)
	}
	if secondHasMore {
		t.Fatalf("second page hasMore should be false")
	}
	if len(second) != 1 {
		t.Fatalf("second page len = %d, want 1", len(second))
	}
	for _, item := range second {
		if _, exists := seen[item.StallID]; exists {
			t.Fatalf("pagination duplicated stallId=%d", item.StallID)
		}
		if item.UserID != 1001 {
			t.Fatalf("unexpected user id = %d", item.UserID)
		}
	}

	empty, emptyHasMore, err := repo.ListUserRatings(ctx, 9999, 20, nil)
	if err != nil {
		t.Fatalf("list empty user ratings failed: %v", err)
	}
	if emptyHasMore {
		t.Fatalf("empty list hasMore should be false")
	}
	if len(empty) != 0 {
		t.Fatalf("empty list len = %d, want 0", len(empty))
	}
}
