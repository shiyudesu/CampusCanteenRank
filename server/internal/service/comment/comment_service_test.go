package service

import (
	"context"
	"errors"
	"testing"

	dto "CampusCanteenRank/server/internal/dto/comment"
	authmodel "CampusCanteenRank/server/internal/model/auth"
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

func TestCommentServiceListReplies(t *testing.T) {
	users := authrepo.NewMemoryUserRepository()
	if err := users.Create(context.Background(), &authmodel.User{
		Nickname:     "Tom",
		Email:        "tom@example.com",
		PasswordHash: "hashed",
		Status:       1,
	}); err != nil {
		t.Fatalf("create seed user failed: %v", err)
	}

	service := NewCommentService(
		commentrepo.NewMemoryCommentRepository(),
		stallrepo.NewMemoryStallRepository(),
		users,
	)

	ctx := context.Background()
	rootID := int64(9001)
	createdReplyIDs := make([]int64, 0, 3)
	for i := 0; i < 3; i++ {
		created, err := service.CreateComment(ctx, 1001, 101, dto.CreateCommentRequest{
			Content:       "reply",
			RootID:        rootID,
			ParentID:      rootID,
			ReplyToUserID: 1001,
		})
		if err != nil {
			t.Fatalf("create reply failed: %v", err)
		}
		createdReplyIDs = append(createdReplyIDs, created.Comment.ID)
	}

	for _, replyID := range createdReplyIDs {
		if _, err := service.LikeComment(ctx, 1001, replyID); err != nil {
			t.Fatalf("like reply failed: %v", err)
		}
	}

	first, err := service.ListReplies(ctx, 1001, rootID, 2, "")
	if err != nil {
		t.Fatalf("list replies first page failed: %v", err)
	}
	if len(first.Items) != 2 {
		t.Fatalf("first page len = %d, want 2", len(first.Items))
	}
	if !first.HasMore {
		t.Fatalf("first page hasMore should be true")
	}
	if first.NextCursor == nil || *first.NextCursor == "" {
		t.Fatalf("first page nextCursor should not be empty")
	}
	if first.Items[0].ReplyToUser == nil {
		t.Fatalf("replyToUser should be present for reply items")
	}
	if !first.Items[0].LikedByMe {
		t.Fatalf("first page item likedByMe should be true for viewer")
	}

	second, err := service.ListReplies(ctx, 1001, rootID, 2, *first.NextCursor)
	if err != nil {
		t.Fatalf("list replies second page failed: %v", err)
	}
	if len(second.Items) == 0 {
		t.Fatalf("second page should contain remaining replies")
	}
	if second.HasMore {
		t.Fatalf("second page hasMore should be false")
	}
	for _, item := range second.Items {
		if item.ID == first.Items[0].ID || item.ID == first.Items[1].ID {
			t.Fatalf("pagination duplicated item id=%d", item.ID)
		}
		if !item.LikedByMe {
			t.Fatalf("second page item likedByMe should be true for viewer")
		}
	}

	guestView, err := service.ListReplies(ctx, 0, rootID, 2, "")
	if err != nil {
		t.Fatalf("guest list replies failed: %v", err)
	}
	if len(guestView.Items) == 0 {
		t.Fatalf("guest list should have items")
	}
	if guestView.Items[0].LikedByMe {
		t.Fatalf("guest likedByMe should be false")
	}
}

func TestCommentServiceListRepliesErrors(t *testing.T) {
	service := NewCommentService(
		commentrepo.NewMemoryCommentRepository(),
		stallrepo.NewMemoryStallRepository(),
		authrepo.NewMemoryUserRepository(),
	)

	ctx := context.Background()

	_, err := service.ListReplies(ctx, 0, 0, 20, "")
	if err == nil {
		t.Fatalf("expected invalid params for zero root id")
	}
	var appErr *errpkg.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("expected AppError, got %T", err)
	}
	if appErr.Code != errpkg.CodeBadRequest {
		t.Fatalf("error code = %d, want %d", appErr.Code, errpkg.CodeBadRequest)
	}

	_, err = service.ListReplies(ctx, 0, 999999, 20, "")
	if err == nil {
		t.Fatalf("expected not found for unknown root")
	}
	if !errors.As(err, &appErr) {
		t.Fatalf("expected AppError, got %T", err)
	}
	if appErr.Code != errpkg.CodeNotFound {
		t.Fatalf("error code = %d, want %d", appErr.Code, errpkg.CodeNotFound)
	}

	_, err = service.ListReplies(ctx, 0, 9001, 20, "bad-cursor")
	if err == nil {
		t.Fatalf("expected bad request for invalid cursor")
	}
	if !errors.As(err, &appErr) {
		t.Fatalf("expected AppError, got %T", err)
	}
	if appErr.Code != errpkg.CodeBadRequest {
		t.Fatalf("error code = %d, want %d", appErr.Code, errpkg.CodeBadRequest)
	}
}
