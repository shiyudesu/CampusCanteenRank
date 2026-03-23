package repository

import (
	"context"
	"errors"
	"sort"
	"sync"
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
	GetByID(ctx context.Context, commentID int64) (*model.Comment, error)
	IncrementRootReplyCount(ctx context.Context, rootID int64) error
	ListTopLevelByStall(ctx context.Context, options CommentListOptions) ([]model.Comment, bool, error)
}

type MemoryCommentRepository struct {
	mu     sync.RWMutex
	nextID int64
	byID   map[int64]model.Comment
}

func NewMemoryCommentRepository() *MemoryCommentRepository {
	now := time.Now().UTC()
	seed := map[int64]model.Comment{
		9001: {
			ID:            9001,
			StallID:       101,
			UserID:        1001,
			RootID:        0,
			ParentID:      0,
			ReplyToUserID: 0,
			Content:       "味道稳定，推荐",
			LikeCount:     12,
			ReplyCount:    3,
			Status:        1,
			CreatedAt:     now.Add(-3 * time.Minute),
		},
		9002: {
			ID:            9002,
			StallID:       101,
			UserID:        1002,
			RootID:        0,
			ParentID:      0,
			ReplyToUserID: 0,
			Content:       "出餐很快，午高峰排队也能接受",
			LikeCount:     5,
			ReplyCount:    1,
			Status:        1,
			CreatedAt:     now.Add(-2 * time.Minute),
		},
		9003: {
			ID:            9003,
			StallID:       101,
			UserID:        1003,
			RootID:        0,
			ParentID:      0,
			ReplyToUserID: 0,
			Content:       "今天红烧肉偏咸，下次希望稳定一点",
			LikeCount:     2,
			ReplyCount:    0,
			Status:        1,
			CreatedAt:     now.Add(-1 * time.Minute),
		},
	}
	return &MemoryCommentRepository{
		nextID: 9003,
		byID:   seed,
	}
}

func (r *MemoryCommentRepository) Create(_ context.Context, comment *model.Comment) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.nextID++
	clone := *comment
	clone.ID = r.nextID
	clone.CreatedAt = time.Now().UTC()
	clone.Status = 1
	r.byID[clone.ID] = clone
	comment.ID = clone.ID
	comment.CreatedAt = clone.CreatedAt
	comment.Status = clone.Status
	return nil
}

func (r *MemoryCommentRepository) GetByID(_ context.Context, commentID int64) (*model.Comment, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	item, ok := r.byID[commentID]
	if !ok || item.Status != 1 {
		return nil, ErrNotFound
	}
	clone := item
	return &clone, nil
}

func (r *MemoryCommentRepository) IncrementRootReplyCount(_ context.Context, rootID int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	item, ok := r.byID[rootID]
	if !ok || item.Status != 1 {
		return ErrNotFound
	}
	if item.RootID != 0 || item.ParentID != 0 {
		return ErrNotFound
	}
	item.ReplyCount++
	r.byID[rootID] = item
	return nil
}

func (r *MemoryCommentRepository) ListTopLevelByStall(_ context.Context, options CommentListOptions) ([]model.Comment, bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	list := make([]model.Comment, 0, len(r.byID))
	for _, item := range r.byID {
		if item.Status != 1 {
			continue
		}
		if item.StallID != options.StallID {
			continue
		}
		if item.RootID != 0 || item.ParentID != 0 {
			continue
		}
		list = append(list, item)
	}

	sort.Slice(list, func(i, j int) bool {
		if list[i].CreatedAt.Equal(list[j].CreatedAt) {
			return list[i].ID > list[j].ID
		}
		return list[i].CreatedAt.After(list[j].CreatedAt)
	})

	filtered := make([]model.Comment, 0, len(list))
	for _, item := range list {
		if options.Cursor != nil {
			if item.CreatedAt.After(options.Cursor.CreatedAt) {
				continue
			}
			if item.CreatedAt.Equal(options.Cursor.CreatedAt) && item.ID >= options.Cursor.ID {
				continue
			}
		}
		filtered = append(filtered, item)
	}

	limit := options.Limit
	if limit <= 0 {
		limit = 20
	}
	hasMore := len(filtered) > limit
	if hasMore {
		filtered = filtered[:limit]
	}
	out := make([]model.Comment, len(filtered))
	copy(out, filtered)
	return out, hasMore, nil
}
