package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	dto "CampusCanteenRank/server/internal/dto/ranking"
	errpkg "CampusCanteenRank/server/internal/pkg/errors"
	"CampusCanteenRank/server/internal/repository/ranking"
)

type rankingCursorPayload struct {
	SortValue    float64   `json:"sortValue"`
	LastActiveAt time.Time `json:"lastActiveAt"`
	StallID      int64     `json:"stallId"`
}

type RankingService struct {
	repo repository.RankingRepository
}

func NewRankingService(repo repository.RankingRepository) *RankingService {
	return &RankingService{repo: repo}
}

func (s *RankingService) ListRankings(
	ctx context.Context,
	scope string,
	scopeID int64,
	foodTypeID int64,
	days int,
	sort string,
	limit int,
	cursorText string,
) (*dto.RankingListData, error) {
	scope = strings.TrimSpace(scope)
	sort = strings.TrimSpace(sort)

	if scope == "" {
		scope = "global"
	}
	if scope != "global" && scope != "canteen" && scope != "foodType" {
		return nil, errpkg.New(errpkg.CodeBadRequest, "invalid params", nil)
	}
	if scope != "global" && scopeID <= 0 {
		return nil, errpkg.New(errpkg.CodeBadRequest, "invalid params", nil)
	}
	if scope == "global" && scopeID != 0 {
		return nil, errpkg.New(errpkg.CodeBadRequest, "invalid params", nil)
	}
	if days == 0 {
		days = 30
	}
	if days != 7 && days != 30 && days != 90 {
		return nil, errpkg.New(errpkg.CodeBadRequest, "invalid params", nil)
	}
	if sort == "" {
		sort = "score_desc"
	}
	if sort != "score_desc" && sort != "hot_desc" {
		return nil, errpkg.New(errpkg.CodeBadRequest, "invalid params", nil)
	}
	if limit <= 0 {
		limit = 20
	}
	if limit > 50 {
		limit = 50
	}

	cursorToken, decodeErr := decodeRankingCursor(cursorText)
	if decodeErr != nil {
		return nil, errpkg.New(errpkg.CodeBadRequest, "invalid params", decodeErr)
	}

	items, hasMore, listErr := s.repo.ListRankings(ctx, repository.RankingListOptions{
		Limit:  limit,
		Cursor: cursorToken,
		Filter: repository.RankingFilter{Scope: scope, ScopeID: scopeID, FoodTypeID: foodTypeID, Days: days, Sort: sort},
	})
	if listErr != nil {
		if errors.Is(listErr, repository.ErrInvalidScope) {
			return nil, errpkg.New(errpkg.CodeBadRequest, "invalid params", nil)
		}
		return nil, errpkg.New(errpkg.CodeInternal, "internal error", listErr)
	}

	out := make([]dto.RankingItem, 0, len(items))
	for idx, item := range items {
		out = append(out, dto.RankingItem{
			Rank:         idx + 1,
			StallID:      item.StallID,
			StallName:    item.StallName,
			CanteenID:    item.CanteenID,
			CanteenName:  item.CanteenName,
			FoodTypeID:   item.FoodTypeID,
			FoodTypeName: item.FoodTypeName,
			AvgRating:    item.AvgRating,
			RatingCount:  item.RatingCount,
			ReviewCount:  item.ReviewCount,
			HotScore:     item.HotScore,
		})
	}

	var nextCursor *string
	if len(items) > 0 && hasMore {
		last := items[len(items)-1]
		sortValue := last.AvgRating
		if sort == "hot_desc" {
			sortValue = last.HotScore
		}
		encoded, encodeErr := encodeRankingCursor(rankingCursorPayload{SortValue: sortValue, LastActiveAt: last.LastActiveAt, StallID: last.StallID})
		if encodeErr != nil {
			return nil, errpkg.New(errpkg.CodeInternal, "internal error", encodeErr)
		}
		nextCursor = &encoded
	}

	return &dto.RankingListData{Items: out, NextCursor: nextCursor, HasMore: hasMore}, nil
}

func decodeRankingCursor(raw string) (*repository.RankingCursor, error) {
	if raw == "" {
		return nil, nil
	}
	decoded, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return nil, err
	}
	var payload rankingCursorPayload
	if err := json.Unmarshal(decoded, &payload); err != nil {
		return nil, err
	}
	if payload.StallID <= 0 || payload.LastActiveAt.IsZero() {
		return nil, strconv.ErrSyntax
	}
	return &repository.RankingCursor{SortValue: payload.SortValue, LastActiveAt: payload.LastActiveAt.UTC(), StallID: payload.StallID}, nil
}

func encodeRankingCursor(payload rankingCursorPayload) (string, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}
