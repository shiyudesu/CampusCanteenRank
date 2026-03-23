package repository

import (
	"context"
	"errors"
	"testing"

	model "CampusCanteenRank/server/internal/model/comment"
)

func TestMemoryCommentRepositoryListByUserPagination(t *testing.T) {
	repo := NewMemoryCommentRepository()
	ctx := context.Background()

	created := make([]*model.Comment, 0, 3)
	for i := 0; i < 3; i++ {
		item := &model.Comment{
			StallID:       101,
			UserID:        1001,
			RootID:        0,
			ParentID:      0,
			ReplyToUserID: 0,
			Content:       "mine",
			LikeCount:     0,
			ReplyCount:    0,
			Status:        1,
		}
		if err := repo.Create(ctx, item); err != nil {
			t.Fatalf("create my comment failed: %v", err)
		}
		created = append(created, item)
	}

	otherUser := &model.Comment{
		StallID:       101,
		UserID:        1002,
		RootID:        0,
		ParentID:      0,
		ReplyToUserID: 0,
		Content:       "other",
		LikeCount:     0,
		ReplyCount:    0,
		Status:        1,
	}
	if err := repo.Create(ctx, otherUser); err != nil {
		t.Fatalf("create other user comment failed: %v", err)
	}

	first, hasMore, err := repo.ListByUser(ctx, 1001, 2, nil)
	if err != nil {
		t.Fatalf("list by user first page failed: %v", err)
	}
	if len(first) != 2 {
		t.Fatalf("first page len = %d, want 2", len(first))
	}
	if !hasMore {
		t.Fatalf("first page hasMore should be true")
	}
	for _, item := range first {
		if item.UserID != 1001 {
			t.Fatalf("unexpected user id=%d in first page", item.UserID)
		}
	}

	cursor := &CommentCursor{CreatedAt: first[len(first)-1].CreatedAt, ID: first[len(first)-1].ID}
	second, secondHasMore, err := repo.ListByUser(ctx, 1001, 2, cursor)
	if err != nil {
		t.Fatalf("list by user second page failed: %v", err)
	}
	if secondHasMore {
		t.Fatalf("second page hasMore should be false")
	}
	if len(second) == 0 {
		t.Fatalf("second page should contain remaining comments")
	}
	for _, item := range second {
		if item.UserID != 1001 {
			t.Fatalf("unexpected user id=%d in second page", item.UserID)
		}
	}
}

func TestMemoryCommentRepositoryLikeUnlikeIdempotent(t *testing.T) {
	repo := NewMemoryCommentRepository()
	ctx := context.Background()

	count, err := repo.Like(ctx, 1001, 9001)
	if err != nil {
		t.Fatalf("first like failed: %v", err)
	}
	if count != 13 {
		t.Fatalf("first like count = %d, want 13", count)
	}

	count, err = repo.Like(ctx, 1001, 9001)
	if err != nil {
		t.Fatalf("second like should be idempotent: %v", err)
	}
	if count != 13 {
		t.Fatalf("second like count = %d, want 13", count)
	}

	count, err = repo.Like(ctx, 1002, 9001)
	if err != nil {
		t.Fatalf("another user like failed: %v", err)
	}
	if count != 14 {
		t.Fatalf("another user like count = %d, want 14", count)
	}

	count, err = repo.Unlike(ctx, 1001, 9001)
	if err != nil {
		t.Fatalf("first unlike failed: %v", err)
	}
	if count != 13 {
		t.Fatalf("first unlike count = %d, want 13", count)
	}

	count, err = repo.Unlike(ctx, 1001, 9001)
	if err != nil {
		t.Fatalf("second unlike should be idempotent: %v", err)
	}
	if count != 13 {
		t.Fatalf("second unlike count = %d, want 13", count)
	}
}

func TestMemoryCommentRepositoryLikeUnlikeNotFound(t *testing.T) {
	repo := NewMemoryCommentRepository()
	ctx := context.Background()

	_, err := repo.Like(ctx, 1001, 999999)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("like unknown comment error = %v, want ErrNotFound", err)
	}

	_, err = repo.Unlike(ctx, 1001, 999999)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("unlike unknown comment error = %v, want ErrNotFound", err)
	}
}

func TestMemoryCommentRepositoryHasLiked(t *testing.T) {
	repo := NewMemoryCommentRepository()
	ctx := context.Background()

	liked, err := repo.HasLiked(ctx, 1001, 9001)
	if err != nil {
		t.Fatalf("initial hasLiked failed: %v", err)
	}
	if liked {
		t.Fatalf("initial liked should be false")
	}

	if _, err := repo.Like(ctx, 1001, 9001); err != nil {
		t.Fatalf("like failed: %v", err)
	}
	liked, err = repo.HasLiked(ctx, 1001, 9001)
	if err != nil {
		t.Fatalf("hasLiked after like failed: %v", err)
	}
	if !liked {
		t.Fatalf("liked should be true after like")
	}

	if _, err := repo.Unlike(ctx, 1001, 9001); err != nil {
		t.Fatalf("unlike failed: %v", err)
	}
	liked, err = repo.HasLiked(ctx, 1001, 9001)
	if err != nil {
		t.Fatalf("hasLiked after unlike failed: %v", err)
	}
	if liked {
		t.Fatalf("liked should be false after unlike")
	}

	if _, err := repo.HasLiked(ctx, 1001, 999999); !errors.Is(err, ErrNotFound) {
		t.Fatalf("hasLiked unknown comment error = %v, want ErrNotFound", err)
	}
}

func TestMemoryCommentRepositoryListRepliesByRootPagination(t *testing.T) {
	repo := NewMemoryCommentRepository()
	ctx := context.Background()

	base := &model.Comment{
		StallID:       101,
		UserID:        1001,
		RootID:        9001,
		ParentID:      9001,
		ReplyToUserID: 1001,
		Content:       "reply base",
		LikeCount:     0,
		ReplyCount:    0,
		Status:        1,
	}
	if err := repo.Create(ctx, base); err != nil {
		t.Fatalf("create base reply failed: %v", err)
	}

	second := &model.Comment{
		StallID:       101,
		UserID:        1002,
		RootID:        9001,
		ParentID:      9001,
		ReplyToUserID: 1001,
		Content:       "reply second",
		LikeCount:     0,
		ReplyCount:    0,
		Status:        1,
	}
	if err := repo.Create(ctx, second); err != nil {
		t.Fatalf("create second reply failed: %v", err)
	}

	otherRoot := &model.Comment{
		StallID:       101,
		UserID:        1003,
		RootID:        9002,
		ParentID:      9002,
		ReplyToUserID: 1002,
		Content:       "reply other root",
		LikeCount:     0,
		ReplyCount:    0,
		Status:        1,
	}
	if err := repo.Create(ctx, otherRoot); err != nil {
		t.Fatalf("create other-root reply failed: %v", err)
	}

	firstPage, hasMore, err := repo.ListRepliesByRoot(ctx, 9001, 1, nil)
	if err != nil {
		t.Fatalf("list first page failed: %v", err)
	}
	if len(firstPage) != 1 {
		t.Fatalf("first page len = %d, want 1", len(firstPage))
	}
	if !hasMore {
		t.Fatalf("first page should have more")
	}
	if firstPage[0].RootID != 9001 {
		t.Fatalf("first page root id = %d, want 9001", firstPage[0].RootID)
	}

	c := &CommentCursor{CreatedAt: firstPage[0].CreatedAt, ID: firstPage[0].ID}
	secondPage, secondHasMore, err := repo.ListRepliesByRoot(ctx, 9001, 10, c)
	if err != nil {
		t.Fatalf("list second page failed: %v", err)
	}
	if secondHasMore {
		t.Fatalf("second page should not have more")
	}
	if len(secondPage) == 0 {
		t.Fatalf("second page should contain remaining replies")
	}
	for _, item := range secondPage {
		if item.RootID != 9001 {
			t.Fatalf("unexpected root id %d in second page", item.RootID)
		}
		if item.ID == firstPage[0].ID {
			t.Fatalf("pagination duplicated reply id=%d", item.ID)
		}
	}

}
