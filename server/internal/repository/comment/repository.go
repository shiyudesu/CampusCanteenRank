package repository

import (
	"context"
	"errors"
	"time"

	model "CampusCanteenRank/server/internal/model/comment"
)

var ErrNotFound = errors.New("not found")

type CommentCursor struct {
	CreatedAt time.Time
	ID        int64
}

type CommentListOptions struct {
	StallID int64
	Limit   int
	Cursor  *CommentCursor
	Sort    string
}

type CommentRepository interface {
	Create(ctx context.Context, comment *model.Comment) error
	CreateReplyAndIncrementRoot(ctx context.Context, reply *model.Comment, rootID int64) error
	GetByID(ctx context.Context, commentID int64) (*model.Comment, error)
	IncrementRootReplyCount(ctx context.Context, rootID int64) error
	Like(ctx context.Context, userID int64, commentID int64) (int64, error)
	Unlike(ctx context.Context, userID int64, commentID int64) (int64, error)
	HasLiked(ctx context.Context, userID int64, commentID int64) (bool, error)
	HasLikedBatch(ctx context.Context, userID int64, commentIDs []int64) (map[int64]bool, error)
	ListTopLevelByStall(ctx context.Context, options CommentListOptions) ([]model.Comment, bool, error)
	ListRepliesByRoot(ctx context.Context, rootCommentID int64, limit int, cursor *CommentCursor) ([]model.Comment, bool, error)
	ListByUser(ctx context.Context, userID int64, limit int, cursor *CommentCursor) ([]model.Comment, bool, error)
}
