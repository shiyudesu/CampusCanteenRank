package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strconv"
	"time"

	commentdto "CampusCanteenRank/server/internal/dto/comment"
	medto "CampusCanteenRank/server/internal/dto/me"
	commentmodel "CampusCanteenRank/server/internal/model/comment"
	"CampusCanteenRank/server/internal/pkg/cursor"
	errpkg "CampusCanteenRank/server/internal/pkg/errors"
	authrepo "CampusCanteenRank/server/internal/repository/auth"
	commentrepo "CampusCanteenRank/server/internal/repository/comment"
	stallrepo "CampusCanteenRank/server/internal/repository/stall"
)

type MyRatingCursor struct {
	UpdatedAt time.Time `json:"updatedAt"`
	StallID   int64     `json:"stallId"`
}

type MeService struct {
	comments commentrepo.CommentRepository
	stalls   stallrepo.StallRepository
	users    authrepo.UserRepository
}

func NewMeService(comments commentrepo.CommentRepository, stalls stallrepo.StallRepository, users authrepo.UserRepository) *MeService {
	return &MeService{comments: comments, stalls: stalls, users: users}
}

func (s *MeService) ListMyComments(ctx context.Context, userID int64, limit int, cursorText string) (*medto.MyCommentListData, error) {
	if userID <= 0 {
		return nil, errpkg.New(errpkg.CodeUnauthorized, "unauthorized", nil)
	}
	if limit <= 0 {
		limit = 20
	}
	if limit > 50 {
		limit = 50
	}

	decodedCursor, decodeErr := cursor.Decode(cursorText)
	if decodeErr != nil {
		return nil, errpkg.New(errpkg.CodeBadRequest, "invalid params", decodeErr)
	}
	var token *commentrepo.CommentCursor
	if decodedCursor != nil {
		token = &commentrepo.CommentCursor{CreatedAt: decodedCursor.CreatedAt, ID: decodedCursor.ID}
	}

	items, hasMore, listErr := s.comments.ListByUser(ctx, userID, limit, token)
	if listErr != nil {
		return nil, errpkg.New(errpkg.CodeInternal, "internal error", listErr)
	}

	nicknames := make(map[int64]string, len(items))
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

	out := make([]commentdto.CommentItem, 0, len(items))
	likedMap, likedBatchErr := s.resolveLikedByMeBatch(ctx, userID, items)
	if likedBatchErr != nil {
		return nil, likedBatchErr
	}
	for _, item := range items {
		likedByMe := likedMap[item.ID]

		commentItem := commentdto.CommentItem{
			ID:            item.ID,
			StallID:       item.StallID,
			RootID:        item.RootID,
			ParentID:      item.ParentID,
			ReplyToUserID: item.ReplyToUserID,
			Content:       item.Content,
			LikeCount:     item.LikeCount,
			ReplyCount:    item.ReplyCount,
			CreatedAt:     item.CreatedAt.UTC().Format(time.RFC3339),
			Author:        commentdto.CommentAuthorVO{ID: item.UserID, Nickname: nicknames[item.UserID]},
			LikedByMe:     likedByMe,
		}
		if item.ReplyToUserID > 0 {
			replyToUser := commentdto.CommentAuthorVO{ID: item.ReplyToUserID, Nickname: nicknames[item.ReplyToUserID]}
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

	return &medto.MyCommentListData{Items: out, NextCursor: nextCursor, HasMore: hasMore}, nil
}

func (s *MeService) resolveLikedByMeBatch(ctx context.Context, viewerUserID int64, items []commentmodel.Comment) (map[int64]bool, error) {
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

func (s *MeService) resolveLikedByMe(ctx context.Context, viewerUserID int64, commentID int64) (bool, error) {
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

func (s *MeService) ListMyRatings(ctx context.Context, userID int64, limit int, cursorText string) (*medto.MyRatingListData, error) {
	if userID <= 0 {
		return nil, errpkg.New(errpkg.CodeUnauthorized, "unauthorized", nil)
	}
	if limit <= 0 {
		limit = 20
	}
	if limit > 50 {
		limit = 50
	}

	token, decodeErr := decodeMyRatingCursor(cursorText)
	if decodeErr != nil {
		return nil, errpkg.New(errpkg.CodeBadRequest, "invalid params", decodeErr)
	}

	ratings, hasMore, listErr := s.stalls.ListUserRatings(ctx, userID, limit, token)
	if listErr != nil {
		return nil, errpkg.New(errpkg.CodeInternal, "internal error", listErr)
	}

	out := make([]medto.MyRatingItem, 0, len(ratings))
	for _, rating := range ratings {
		stallItem, err := s.stalls.GetStallByID(ctx, rating.StallID)
		if err != nil {
			if errors.Is(err, stallrepo.ErrNotFound) {
				continue
			}
			return nil, errpkg.New(errpkg.CodeInternal, "internal error", err)
		}
		out = append(out, medto.MyRatingItem{
			StallID:   rating.StallID,
			StallName: stallItem.Name,
			Score:     rating.Score,
			UpdatedAt: rating.UpdatedAt.UTC().Format(time.RFC3339),
		})
	}

	var nextCursor *string
	if len(ratings) > 0 && hasMore {
		last := ratings[len(ratings)-1]
		encoded, encErr := encodeMyRatingCursor(MyRatingCursor{UpdatedAt: last.UpdatedAt, StallID: last.StallID})
		if encErr != nil {
			return nil, errpkg.New(errpkg.CodeInternal, "internal error", encErr)
		}
		nextCursor = &encoded
	}

	return &medto.MyRatingListData{Items: out, NextCursor: nextCursor, HasMore: hasMore}, nil
}

func decodeMyRatingCursor(raw string) (*stallrepo.UserRatingCursor, error) {
	if raw == "" {
		return nil, nil
	}
	decoded, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return nil, err
	}
	var payload MyRatingCursor
	if err := json.Unmarshal(decoded, &payload); err != nil {
		return nil, err
	}
	if payload.StallID <= 0 {
		return nil, strconv.ErrSyntax
	}
	if payload.UpdatedAt.IsZero() {
		return nil, strconv.ErrSyntax
	}
	return &stallrepo.UserRatingCursor{UpdatedAt: payload.UpdatedAt, StallID: payload.StallID}, nil
}

func encodeMyRatingCursor(payload MyRatingCursor) (string, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(data), nil
}
