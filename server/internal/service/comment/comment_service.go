package service

import (
	"context"
	"errors"
	"strings"
	"time"

	dto "CampusCanteenRank/server/internal/dto/comment"
	model "CampusCanteenRank/server/internal/model/comment"
	"CampusCanteenRank/server/internal/pkg/cursor"
	errpkg "CampusCanteenRank/server/internal/pkg/errors"
	authrepo "CampusCanteenRank/server/internal/repository/auth"
	commentrepo "CampusCanteenRank/server/internal/repository/comment"
	stallrepo "CampusCanteenRank/server/internal/repository/stall"
)

type CommentService struct {
	comments commentrepo.CommentRepository
	stalls   stallrepo.StallRepository
	users    authrepo.UserRepository
}

func NewCommentService(comments commentrepo.CommentRepository, stalls stallrepo.StallRepository, users authrepo.UserRepository) *CommentService {
	return &CommentService{comments: comments, stalls: stalls, users: users}
}

func (s *CommentService) CreateComment(ctx context.Context, userID int64, stallID int64, req dto.CreateCommentRequest) (*dto.CreateCommentData, error) {
	if userID <= 0 || stallID <= 0 {
		return nil, errpkg.New(errpkg.CodeBadRequest, "invalid params", nil)
	}
	content := strings.TrimSpace(req.Content)
	if content == "" {
		return nil, errpkg.New(errpkg.CodeBadRequest, "invalid params", nil)
	}

	if _, err := s.stalls.GetStallByID(ctx, stallID); err != nil {
		if errors.Is(err, stallrepo.ErrNotFound) {
			return nil, errpkg.New(errpkg.CodeNotFound, "stall not found", nil)
		}
		return nil, errpkg.New(errpkg.CodeInternal, "internal error", err)
	}

	author, err := s.users.GetByID(ctx, userID)
	if err != nil {
		if errors.Is(err, authrepo.ErrNotFound) {
			return nil, errpkg.New(errpkg.CodeUnauthorized, "unauthorized", nil)
		}
		return nil, errpkg.New(errpkg.CodeInternal, "internal error", err)
	}

	isTopLevel := req.RootID == 0 && req.ParentID == 0 && req.ReplyToUserID == 0
	isReply := req.RootID > 0 && req.ParentID > 0 && req.ReplyToUserID > 0
	if !isTopLevel && !isReply {
		return nil, errpkg.New(errpkg.CodeBadRequest, "invalid params", nil)
	}

	newComment := &model.Comment{
		StallID:       stallID,
		UserID:        userID,
		RootID:        req.RootID,
		ParentID:      req.ParentID,
		ReplyToUserID: req.ReplyToUserID,
		Content:       content,
		LikeCount:     0,
		ReplyCount:    0,
		Status:        1,
	}

	if isReply {
		rootComment, rootErr := s.comments.GetByID(ctx, req.RootID)
		if rootErr != nil {
			if errors.Is(rootErr, commentrepo.ErrNotFound) {
				return nil, errpkg.New(errpkg.CodeNotFound, "comment not found", nil)
			}
			return nil, errpkg.New(errpkg.CodeInternal, "internal error", rootErr)
		}
		if rootComment.StallID != stallID || rootComment.RootID != 0 || rootComment.ParentID != 0 {
			return nil, errpkg.New(errpkg.CodeBadRequest, "invalid params", nil)
		}

		parentComment, parentErr := s.comments.GetByID(ctx, req.ParentID)
		if parentErr != nil {
			if errors.Is(parentErr, commentrepo.ErrNotFound) {
				return nil, errpkg.New(errpkg.CodeNotFound, "comment not found", nil)
			}
			return nil, errpkg.New(errpkg.CodeInternal, "internal error", parentErr)
		}
		if parentComment.StallID != stallID {
			return nil, errpkg.New(errpkg.CodeBadRequest, "invalid params", nil)
		}
		if parentComment.RootID == 0 {
			if parentComment.ID != req.RootID {
				return nil, errpkg.New(errpkg.CodeBadRequest, "invalid params", nil)
			}
		} else if parentComment.RootID != req.RootID {
			return nil, errpkg.New(errpkg.CodeBadRequest, "invalid params", nil)
		}
		if parentComment.UserID != req.ReplyToUserID {
			return nil, errpkg.New(errpkg.CodeBadRequest, "invalid params", nil)
		}
	}

	if createErr := s.comments.Create(ctx, newComment); createErr != nil {
		return nil, errpkg.New(errpkg.CodeInternal, "internal error", createErr)
	}

	if isReply {
		incErr := s.comments.IncrementRootReplyCount(ctx, req.RootID)
		if incErr != nil {
			if errors.Is(incErr, commentrepo.ErrNotFound) {
				return nil, errpkg.New(errpkg.CodeNotFound, "comment not found", nil)
			}
			return nil, errpkg.New(errpkg.CodeInternal, "internal error", incErr)
		}
	}

	createdComment, loadErr := s.comments.GetByID(ctx, newComment.ID)
	if loadErr != nil {
		if errors.Is(loadErr, commentrepo.ErrNotFound) {
			return nil, errpkg.New(errpkg.CodeNotFound, "comment not found", nil)
		}
		return nil, errpkg.New(errpkg.CodeInternal, "internal error", loadErr)
	}

	item := toCommentItem(*createdComment, author.ID, author.Nickname)
	return &dto.CreateCommentData{Comment: item}, nil
}

func (s *CommentService) ListTopLevelComments(
	ctx context.Context,
	stallID int64,
	limit int,
	cursorText string,
	sort string,
) (*dto.CommentListData, error) {
	if stallID <= 0 {
		return nil, errpkg.New(errpkg.CodeBadRequest, "invalid params", nil)
	}
	if limit <= 0 {
		limit = 20
	}
	if limit > 50 {
		limit = 50
	}
	if sort == "" {
		sort = "latest"
	}
	if strings.TrimSpace(sort) != "latest" {
		return nil, errpkg.New(errpkg.CodeBadRequest, "invalid params", nil)
	}

	if _, err := s.stalls.GetStallByID(ctx, stallID); err != nil {
		if errors.Is(err, stallrepo.ErrNotFound) {
			return nil, errpkg.New(errpkg.CodeNotFound, "stall not found", nil)
		}
		return nil, errpkg.New(errpkg.CodeInternal, "internal error", err)
	}

	decodedCursor, decodeErr := cursor.Decode(cursorText)
	if decodeErr != nil {
		return nil, errpkg.New(errpkg.CodeBadRequest, "invalid params", decodeErr)
	}
	var token *commentrepo.CommentCursor
	if decodedCursor != nil {
		token = &commentrepo.CommentCursor{CreatedAt: decodedCursor.CreatedAt, ID: decodedCursor.ID}
	}

	items, hasMore, listErr := s.comments.ListTopLevelByStall(ctx, commentrepo.CommentListOptions{
		StallID: stallID,
		Limit:   limit,
		Cursor:  token,
		Sort:    sort,
	})
	if listErr != nil {
		return nil, errpkg.New(errpkg.CodeInternal, "internal error", listErr)
	}

	nicknames := make(map[int64]string, len(items))
	for _, item := range items {
		if _, exists := nicknames[item.UserID]; exists {
			continue
		}
		user, userErr := s.users.GetByID(ctx, item.UserID)
		if userErr != nil {
			if errors.Is(userErr, authrepo.ErrNotFound) {
				nicknames[item.UserID] = "Unknown User"
				continue
			}
			return nil, errpkg.New(errpkg.CodeInternal, "internal error", userErr)
		}
		nicknames[item.UserID] = user.Nickname
	}

	out := make([]dto.CommentItem, 0, len(items))
	for _, item := range items {
		out = append(out, toCommentItem(item, item.UserID, nicknames[item.UserID]))
	}

	var nextCursor *string
	if len(items) > 0 && hasMore {
		last := items[len(items)-1]
		encoded, encErr := cursor.Encode(cursor.Token{CreatedAt: last.CreatedAt, ID: last.ID})
		if encErr != nil {
			return nil, errpkg.New(errpkg.CodeInternal, "internal error", encErr)
		}
		nextCursor = &encoded
	}

	return &dto.CommentListData{Items: out, NextCursor: nextCursor, HasMore: hasMore}, nil
}

func toCommentItem(item model.Comment, authorID int64, nickname string) dto.CommentItem {
	return dto.CommentItem{
		ID:            item.ID,
		StallID:       item.StallID,
		RootID:        item.RootID,
		ParentID:      item.ParentID,
		ReplyToUserID: item.ReplyToUserID,
		Content:       item.Content,
		LikeCount:     item.LikeCount,
		ReplyCount:    item.ReplyCount,
		CreatedAt:     item.CreatedAt.UTC().Format(time.RFC3339),
		Author:        dto.CommentAuthorVO{ID: authorID, Nickname: nickname},
		LikedByMe:     false,
	}
}
