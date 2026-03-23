package service

import (
	"context"
	"errors"
	"testing"

	authmodel "CampusCanteenRank/server/internal/model/auth"
	commentmodel "CampusCanteenRank/server/internal/model/comment"
	errpkg "CampusCanteenRank/server/internal/pkg/errors"
	authrepo "CampusCanteenRank/server/internal/repository/auth"
	commentrepo "CampusCanteenRank/server/internal/repository/comment"
	stallrepo "CampusCanteenRank/server/internal/repository/stall"
)

func TestMeServiceListMyCommentsReturnsLikedByMeState(t *testing.T) {
	ctx := context.Background()
	users := authrepo.NewMemoryUserRepository()
	userID := createUser(t, users, "me@example.com", "MeUser")
	if userID != 1001 {
		t.Fatalf("seed user id = %d, want 1001", userID)
	}

	comments := commentrepo.NewMemoryCommentRepository()
	if _, err := comments.Like(ctx, userID, 9001); err != nil {
		t.Fatalf("seed like failed: %v", err)
	}

	service := NewMeService(comments, stallrepo.NewMemoryStallRepository(), users)
	data, err := service.ListMyComments(ctx, userID, 20, "")
	if err != nil {
		t.Fatalf("list my comments failed: %v", err)
	}

	if data.HasMore {
		t.Fatalf("hasMore = true, want false")
	}
	if data.NextCursor != nil {
		t.Fatalf("nextCursor = %v, want nil", *data.NextCursor)
	}

	likedFound := false
	for _, item := range data.Items {
		if item.ID != 9001 {
			continue
		}
		likedFound = true
		if !item.LikedByMe {
			t.Fatalf("likedByMe = false, want true")
		}
		if item.Author.Nickname != "MeUser" {
			t.Fatalf("author nickname = %q, want %q", item.Author.Nickname, "MeUser")
		}
	}
	if !likedFound {
		t.Fatalf("expected liked seed comment in my comments list")
	}
}

func TestMeServiceListMyCommentsInvalidCursor(t *testing.T) {
	users := authrepo.NewMemoryUserRepository()
	userID := createUser(t, users, "me@example.com", "MeUser")
	service := NewMeService(commentrepo.NewMemoryCommentRepository(), stallrepo.NewMemoryStallRepository(), users)

	_, err := service.ListMyComments(context.Background(), userID, 20, "bad-cursor")
	requireAppErrorCode(t, err, errpkg.CodeBadRequest)
}

func TestMeServiceListMyCommentsUnauthorized(t *testing.T) {
	service := NewMeService(
		commentrepo.NewMemoryCommentRepository(),
		stallrepo.NewMemoryStallRepository(),
		authrepo.NewMemoryUserRepository(),
	)

	_, err := service.ListMyComments(context.Background(), 0, 20, "")
	requireAppErrorCode(t, err, errpkg.CodeUnauthorized)
}

func TestMeServiceListMyRatingsPagination(t *testing.T) {
	ctx := context.Background()
	users := authrepo.NewMemoryUserRepository()
	userID := createUser(t, users, "me@example.com", "MeUser")
	stalls := stallrepo.NewMemoryStallRepository()

	for _, payload := range []struct {
		stallID int64
		score   int
	}{{stallID: 101, score: 5}, {stallID: 102, score: 4}, {stallID: 201, score: 3}} {
		if _, err := stalls.UpsertUserRating(ctx, userID, payload.stallID, payload.score); err != nil {
			t.Fatalf("seed rating failed for stall %d: %v", payload.stallID, err)
		}
	}

	service := NewMeService(commentrepo.NewMemoryCommentRepository(), stalls, users)
	first, err := service.ListMyRatings(ctx, userID, 2, "")
	if err != nil {
		t.Fatalf("list my ratings first page failed: %v", err)
	}
	if len(first.Items) != 2 {
		t.Fatalf("first page len = %d, want 2", len(first.Items))
	}
	if !first.HasMore {
		t.Fatalf("first page hasMore = false, want true")
	}
	if first.NextCursor == nil || *first.NextCursor == "" {
		t.Fatalf("first page nextCursor should not be empty")
	}

	second, err := service.ListMyRatings(ctx, userID, 2, *first.NextCursor)
	if err != nil {
		t.Fatalf("list my ratings second page failed: %v", err)
	}
	if len(second.Items) != 1 {
		t.Fatalf("second page len = %d, want 1", len(second.Items))
	}
	if second.HasMore {
		t.Fatalf("second page hasMore = true, want false")
	}

	seen := make(map[int64]struct{}, 3)
	for _, item := range append(first.Items, second.Items...) {
		if item.StallName == "" {
			t.Fatalf("stallName should not be empty")
		}
		if item.UpdatedAt == "" {
			t.Fatalf("updatedAt should not be empty")
		}
		seen[item.StallID] = struct{}{}
	}
	if len(seen) != 3 {
		t.Fatalf("unique rated stalls = %d, want 3", len(seen))
	}
}

func TestMeServiceListMyRatingsUnauthorized(t *testing.T) {
	service := NewMeService(
		commentrepo.NewMemoryCommentRepository(),
		stallrepo.NewMemoryStallRepository(),
		authrepo.NewMemoryUserRepository(),
	)

	_, err := service.ListMyRatings(context.Background(), 0, 20, "")
	requireAppErrorCode(t, err, errpkg.CodeUnauthorized)
}

func TestMeServiceListMyRatingsInvalidCursor(t *testing.T) {
	users := authrepo.NewMemoryUserRepository()
	userID := createUser(t, users, "me@example.com", "MeUser")
	service := NewMeService(commentrepo.NewMemoryCommentRepository(), stallrepo.NewMemoryStallRepository(), users)

	_, err := service.ListMyRatings(context.Background(), userID, 20, "bad-cursor")
	requireAppErrorCode(t, err, errpkg.CodeBadRequest)
}

func createUser(t *testing.T, users *authrepo.MemoryUserRepository, email string, nickname string) int64 {
	t.Helper()
	user := &authmodel.User{
		Nickname:     nickname,
		Email:        email,
		PasswordHash: "hashed",
		Status:       1,
	}
	if err := users.Create(context.Background(), user); err != nil {
		t.Fatalf("create user failed: %v", err)
	}
	return user.ID
}

func requireAppErrorCode(t *testing.T, err error, wantCode int) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected app error code %d, got nil", wantCode)
	}
	var appErr *errpkg.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("expected AppError, got %T", err)
	}
	if appErr.Code != wantCode {
		t.Fatalf("app error code = %d, want %d", appErr.Code, wantCode)
	}
}

func TestMeServiceListMyCommentsReturnsUnknownUserNicknameFallback(t *testing.T) {
	ctx := context.Background()
	service := NewMeService(
		commentrepo.NewMemoryCommentRepository(),
		stallrepo.NewMemoryStallRepository(),
		authrepo.NewMemoryUserRepository(),
	)

	data, err := service.ListMyComments(ctx, 1001, 20, "")
	if err != nil {
		t.Fatalf("list my comments failed: %v", err)
	}

	found := false
	for _, item := range data.Items {
		if item.ID != 9001 {
			continue
		}
		found = true
		if item.Author.Nickname != "Unknown User" {
			t.Fatalf("author nickname = %q, want %q", item.Author.Nickname, "Unknown User")
		}
	}
	if !found {
		t.Fatalf("expected seed comment in my comments list")
	}
}

func TestMeServiceListMyCommentsPagination(t *testing.T) {
	ctx := context.Background()
	users := authrepo.NewMemoryUserRepository()
	userID := createUser(t, users, "me@example.com", "MeUser")
	comments := commentrepo.NewMemoryCommentRepository()

	for i := 0; i < 3; i++ {
		if err := comments.Create(ctx, &commentmodel.Comment{
			StallID:       101,
			UserID:        userID,
			RootID:        0,
			ParentID:      0,
			ReplyToUserID: 0,
			Content:       "my comment",
			LikeCount:     0,
			ReplyCount:    0,
			Status:        1,
		}); err != nil {
			t.Fatalf("create comment failed: %v", err)
		}
	}

	service := NewMeService(comments, stallrepo.NewMemoryStallRepository(), users)
	first, err := service.ListMyComments(ctx, userID, 2, "")
	if err != nil {
		t.Fatalf("list my comments first page failed: %v", err)
	}
	if len(first.Items) != 2 {
		t.Fatalf("first page len = %d, want 2", len(first.Items))
	}
	if !first.HasMore {
		t.Fatalf("first page hasMore = false, want true")
	}
	if first.NextCursor == nil || *first.NextCursor == "" {
		t.Fatalf("first page nextCursor should not be empty")
	}

	second, err := service.ListMyComments(ctx, userID, 2, *first.NextCursor)
	if err != nil {
		t.Fatalf("list my comments second page failed: %v", err)
	}
	if len(second.Items) == 0 {
		t.Fatalf("second page should contain remaining items")
	}
	if second.HasMore {
		t.Fatalf("second page hasMore = true, want false")
	}
}
