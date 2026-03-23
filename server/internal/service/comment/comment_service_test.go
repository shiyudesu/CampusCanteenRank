package service

import (
	"context"
	"errors"
	"testing"

	errpkg "CampusCanteenRank/server/internal/pkg/errors"
	authrepo "CampusCanteenRank/server/internal/repository/auth"
	commentrepo "CampusCanteenRank/server/internal/repository/comment"
	stallrepo "CampusCanteenRank/server/internal/repository/stall"
)

func TestCommentServiceLikeUnlike(t *testing.T) {
	service := NewCommentService(
		commentrepo.NewMemoryCommentRepository(),
		stallrepo.NewMemoryStallRepository(),
		authrepo.NewMemoryUserRepository(),
	)

	ctx := context.Background()

	liked, err := service.LikeComment(ctx, 1001, 9001)
	if err != nil {
		t.Fatalf("like comment failed: %v", err)
	}
	if !liked.Liked {
		t.Fatalf("liked flag should be true")
	}
	if liked.LikeCount != 13 {
		t.Fatalf("like count = %d, want 13", liked.LikeCount)
	}

	liked, err = service.LikeComment(ctx, 1001, 9001)
	if err != nil {
		t.Fatalf("repeat like should be idempotent: %v", err)
	}
	if liked.LikeCount != 13 {
		t.Fatalf("repeat like count = %d, want 13", liked.LikeCount)
	}

	unliked, err := service.UnlikeComment(ctx, 1001, 9001)
	if err != nil {
		t.Fatalf("unlike comment failed: %v", err)
	}
	if unliked.Liked {
		t.Fatalf("liked flag should be false")
	}
	if unliked.LikeCount != 12 {
		t.Fatalf("unlike count = %d, want 12", unliked.LikeCount)
	}

	unliked, err = service.UnlikeComment(ctx, 1001, 9001)
	if err != nil {
		t.Fatalf("repeat unlike should be idempotent: %v", err)
	}
	if unliked.LikeCount != 12 {
		t.Fatalf("repeat unlike count = %d, want 12", unliked.LikeCount)
	}
}

func TestCommentServiceLikeCommentNotFound(t *testing.T) {
	service := NewCommentService(
		commentrepo.NewMemoryCommentRepository(),
		stallrepo.NewMemoryStallRepository(),
		authrepo.NewMemoryUserRepository(),
	)

	_, err := service.LikeComment(context.Background(), 1001, 999999)
	if err == nil {
		t.Fatalf("expected error for unknown comment")
	}
	var appErr *errpkg.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("expected AppError, got %T", err)
	}
	if appErr.Code != errpkg.CodeNotFound {
		t.Fatalf("error code = %d, want %d", appErr.Code, errpkg.CodeNotFound)
	}
}

func TestCommentServiceUnlikeCommentNotFound(t *testing.T) {
	service := NewCommentService(
		commentrepo.NewMemoryCommentRepository(),
		stallrepo.NewMemoryStallRepository(),
		authrepo.NewMemoryUserRepository(),
	)

	_, err := service.UnlikeComment(context.Background(), 1001, 999999)
	if err == nil {
		t.Fatalf("expected error for unknown comment")
	}
	var appErr *errpkg.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("expected AppError, got %T", err)
	}
	if appErr.Code != errpkg.CodeNotFound {
		t.Fatalf("error code = %d, want %d", appErr.Code, errpkg.CodeNotFound)
	}
}

func TestCommentServiceLikeUnlikeInvalidParams(t *testing.T) {
	service := NewCommentService(
		commentrepo.NewMemoryCommentRepository(),
		stallrepo.NewMemoryStallRepository(),
		authrepo.NewMemoryUserRepository(),
	)

	cases := []struct {
		name string
		run  func() error
	}{
		{name: "like invalid user", run: func() error {
			_, err := service.LikeComment(context.Background(), 0, 9001)
			return err
		}},
		{name: "like invalid comment", run: func() error {
			_, err := service.LikeComment(context.Background(), 1001, 0)
			return err
		}},
		{name: "unlike invalid user", run: func() error {
			_, err := service.UnlikeComment(context.Background(), 0, 9001)
			return err
		}},
		{name: "unlike invalid comment", run: func() error {
			_, err := service.UnlikeComment(context.Background(), 1001, -1)
			return err
		}},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.run()
			if err == nil {
				t.Fatalf("expected bad request error")
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
