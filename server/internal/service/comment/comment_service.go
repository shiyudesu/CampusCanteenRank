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

	item := toCommentItem(*createdComment, author.ID, author.Nickname, false)
	return &dto.CreateCommentData{Comment: item}, nil
}

func (s *CommentService) ListTopLevelComments(
	ctx context.Context,
	viewerUserID int64,
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
	likedMap, likedBatchErr := s.resolveLikedByMeBatch(ctx, viewerUserID, items)
	if likedBatchErr != nil {
		return nil, likedBatchErr
	}
	for _, item := range items {
		likedByMe := likedMap[item.ID]
		out = append(out, toCommentItem(item, item.UserID, nicknames[item.UserID], likedByMe))
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

func (s *CommentService) ListReplies(
	ctx context.Context,
	viewerUserID int64,
	rootCommentID int64,
	limit int,
	cursorText string,
) (*dto.CommentListData, error) {
	if rootCommentID <= 0 {
		return nil, errpkg.New(errpkg.CodeBadRequest, "invalid params", nil)
	}
	if limit <= 0 {
		limit = 20
	}
	if limit > 50 {
		limit = 50
	}

	rootComment, rootErr := s.comments.GetByID(ctx, rootCommentID)
	if rootErr != nil {
		if errors.Is(rootErr, commentrepo.ErrNotFound) {
			return nil, errpkg.New(errpkg.CodeNotFound, "comment not found", nil)
		}
		return nil, errpkg.New(errpkg.CodeInternal, "internal error", rootErr)
	}
	if rootComment.RootID != 0 || rootComment.ParentID != 0 {
		return nil, errpkg.New(errpkg.CodeBadRequest, "invalid params", nil)
	}

	decodedCursor, decodeErr := cursor.Decode(cursorText)
	if decodeErr != nil {
		return nil, errpkg.New(errpkg.CodeBadRequest, "invalid params", decodeErr)
	}
	var token *commentrepo.CommentCursor
	if decodedCursor != nil {
		token = &commentrepo.CommentCursor{CreatedAt: decodedCursor.CreatedAt, ID: decodedCursor.ID}
	}

	items, hasMore, listErr := s.comments.ListRepliesByRoot(ctx, rootCommentID, limit, token)
	if listErr != nil {
		return nil, errpkg.New(errpkg.CodeInternal, "internal error", listErr)
	}

	nicknames := make(map[int64]string, len(items)+1)
	for _, item := range items {
		if _, exists := nicknames[item.UserID]; !exists {
			user, userErr := s.users.GetByID(ctx, item.UserID)
			if userErr != nil {
				if errors.Is(userErr, authrepo.ErrNotFound) {
					nicknames[item.UserID] = "Unknown User"
				} else {
					return nil, errpkg.New(errpkg.CodeInternal, "internal error", userErr)
				}
			} else {
				nicknames[item.UserID] = user.Nickname
			}
		}
		if item.ReplyToUserID > 0 {
			if _, exists := nicknames[item.ReplyToUserID]; !exists {
				user, userErr := s.users.GetByID(ctx, item.ReplyToUserID)
				if userErr != nil {
					if errors.Is(userErr, authrepo.ErrNotFound) {
						nicknames[item.ReplyToUserID] = "Unknown User"
					} else {
						return nil, errpkg.New(errpkg.CodeInternal, "internal error", userErr)
					}
				} else {
					nicknames[item.ReplyToUserID] = user.Nickname
				}
			}
		}
	}

	out := make([]dto.CommentItem, 0, len(items))
	likedMap, likedBatchErr := s.resolveLikedByMeBatch(ctx, viewerUserID, items)
	if likedBatchErr != nil {
		return nil, likedBatchErr
	}
	for _, item := range items {
		likedByMe := likedMap[item.ID]
		commentItem := toCommentItem(item, item.UserID, nicknames[item.UserID], likedByMe)
		if item.ReplyToUserID > 0 {
			replyToUser := dto.CommentAuthorVO{ID: item.ReplyToUserID, Nickname: nicknames[item.ReplyToUserID]}
			commentItem.ReplyToUser = &replyToUser
		}
		out = append(out, commentItem)
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

func (s *CommentService) resolveLikedByMe(ctx context.Context, viewerUserID int64, commentID int64) (bool, error) {
	if viewerUserID <= 0 {
		return false, nil
	}
	likedByMe, err := s.comments.HasLiked(ctx, viewerUserID, commentID)
	if err != nil {
		if errors.Is(err, commentrepo.ErrNotFound) {
			return false, errpkg.New(errpkg.CodeNotFound, "comment not found", nil)
		}
		return false, errpkg.New(errpkg.CodeInternal, "internal error", err)
	}
	return likedByMe, nil
}

func (s *CommentService) resolveLikedByMeBatch(ctx context.Context, viewerUserID int64, items []model.Comment) (map[int64]bool, error) {
	result := make(map[int64]bool, len(items))
	if len(items) == 0 || viewerUserID <= 0 {
		for _, item := range items {
			result[item.ID] = false
		}
		return result, nil
	}

	ids := make([]int64, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.ID)
	}
	likedMap, err := s.comments.HasLikedBatch(ctx, viewerUserID, ids)
	if err != nil {
		return nil, errpkg.New(errpkg.CodeInternal, "internal error", err)
	}
	for _, item := range items {
		result[item.ID] = likedMap[item.ID]
	}
	return result, nil
}

func (s *CommentService) LikeComment(ctx context.Context, userID int64, commentID int64) (*dto.ToggleLikeData, error) {
	if userID <= 0 || commentID <= 0 {
		return nil, errpkg.New(errpkg.CodeBadRequest, "invalid params", nil)
	}
	likeCount, err := s.comments.Like(ctx, userID, commentID)
	if err != nil {
		if errors.Is(err, commentrepo.ErrNotFound) {
			return nil, errpkg.New(errpkg.CodeNotFound, "comment not found", nil)
		}
		return nil, errpkg.New(errpkg.CodeInternal, "internal error", err)
	}
	return &dto.ToggleLikeData{Liked: true, LikeCount: likeCount}, nil
}

func (s *CommentService) UnlikeComment(ctx context.Context, userID int64, commentID int64) (*dto.ToggleLikeData, error) {
	if userID <= 0 || commentID <= 0 {
		return nil, errpkg.New(errpkg.CodeBadRequest, "invalid params", nil)
	}
	likeCount, err := s.comments.Unlike(ctx, userID, commentID)
	if err != nil {
		if errors.Is(err, commentrepo.ErrNotFound) {
			return nil, errpkg.New(errpkg.CodeNotFound, "comment not found", nil)
		}
		return nil, errpkg.New(errpkg.CodeInternal, "internal error", err)
	}
	return &dto.ToggleLikeData{Liked: false, LikeCount: likeCount}, nil
}

func toCommentItem(item model.Comment, authorID int64, nickname string, likedByMe bool) dto.CommentItem {
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
		LikedByMe:     likedByMe,
	}
}
