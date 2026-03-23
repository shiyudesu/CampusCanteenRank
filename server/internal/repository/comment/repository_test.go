package repository

import (
	"context"
	"errors"
	"testing"
)

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
