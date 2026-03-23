package service

import (
	"context"
	"errors"
	"testing"

	errpkg "CampusCanteenRank/server/internal/pkg/errors"
	repository "CampusCanteenRank/server/internal/repository/ranking"
)

func TestRankingServiceListRankings(t *testing.T) {
	svc := NewRankingService(repository.NewMemoryRankingRepository())

	first, err := svc.ListRankings(context.Background(), "global", 0, 0, 30, "score_desc", 2, "")
	if err != nil {
		t.Fatalf("list rankings first page failed: %v", err)
	}
	if len(first.Items) != 2 {
		t.Fatalf("first page len = %d, want 2", len(first.Items))
	}
	if !first.HasMore {
		t.Fatalf("first page hasMore should be true")
	}
	if first.NextCursor == nil || *first.NextCursor == "" {
		t.Fatalf("first page nextCursor should be non-empty")
	}
	if first.Items[0].Rank != 1 || first.Items[1].Rank != 2 {
		t.Fatalf("first page rank should start from 1 and increment")
	}

	second, err := svc.ListRankings(context.Background(), "global", 0, 0, 30, "score_desc", 2, *first.NextCursor)
	if err != nil {
		t.Fatalf("list rankings second page failed: %v", err)
	}
	if len(second.Items) == 0 {
		t.Fatalf("second page should not be empty")
	}
	if second.Items[0].StallID == first.Items[0].StallID || second.Items[0].StallID == first.Items[1].StallID {
		t.Fatalf("second page should not duplicate first page")
	}

	hot, err := svc.ListRankings(context.Background(), "global", 0, 0, 30, "hot_desc", 20, "")
	if err != nil {
		t.Fatalf("list hot rankings failed: %v", err)
	}
	if len(hot.Items) < 2 {
		t.Fatalf("hot rankings should have at least 2 items")
	}
	if hot.Items[0].HotScore < hot.Items[1].HotScore {
		t.Fatalf("hot rankings should be sorted by hotScore desc")
	}
}

func TestRankingServiceInvalidParams(t *testing.T) {
	svc := NewRankingService(repository.NewMemoryRankingRepository())

	cases := []struct {
		name string
		run  func() error
	}{
		{name: "invalid scope", run: func() error {
			_, err := svc.ListRankings(context.Background(), "unknown", 0, 0, 30, "score_desc", 20, "")
			return err
		}},
		{name: "canteen scope without scopeId", run: func() error {
			_, err := svc.ListRankings(context.Background(), "canteen", 0, 0, 30, "score_desc", 20, "")
			return err
		}},
		{name: "invalid days", run: func() error {
			_, err := svc.ListRankings(context.Background(), "global", 0, 0, 15, "score_desc", 20, "")
			return err
		}},
		{name: "invalid sort", run: func() error {
			_, err := svc.ListRankings(context.Background(), "global", 0, 0, 30, "latest", 20, "")
			return err
		}},
		{name: "invalid cursor", run: func() error {
			_, err := svc.ListRankings(context.Background(), "global", 0, 0, 30, "score_desc", 20, "bad-cursor")
			return err
		}},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.run()
			if err == nil {
				t.Fatalf("expected error")
			}
			var appErr *errpkg.AppError
			if !errors.As(err, &appErr) {
				t.Fatalf("expected AppError, got %T", err)
			}
			if appErr.Code != errpkg.CodeBadRequest {
				t.Fatalf("error code = %d, want %d", appErr.Code, errpkg.CodeBadRequest)
			}
		})
	}
}
