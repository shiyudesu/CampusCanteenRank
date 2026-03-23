package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"math"
	"strconv"
	"strings"

	dto "CampusCanteenRank/server/internal/dto/stall"
	errpkg "CampusCanteenRank/server/internal/pkg/errors"
	"CampusCanteenRank/server/internal/repository/stall"
)

type stallListCursor struct {
	AvgRating float64 `json:"avgRating"`
	ID        int64   `json:"id"`
}

type StallService struct {
	repo repository.StallRepository
}

func NewStallService(repo repository.StallRepository) *StallService {
	return &StallService{repo: repo}
}

func (s *StallService) ListCanteens(ctx context.Context) (*dto.CanteenListData, error) {
	items, err := s.repo.ListCanteens(ctx)
	if err != nil {
		return nil, errpkg.New(errpkg.CodeInternal, "internal error", err)
	}
	out := make([]dto.CanteenItem, 0, len(items))
	for _, item := range items {
		out = append(out, dto.CanteenItem{ID: item.ID, Name: item.Name, Campus: item.Campus})
	}
	return &dto.CanteenListData{Items: out}, nil
}

func (s *StallService) ListStalls(ctx context.Context, limit int, cursorText string, canteenID int64, foodTypeID int64, sort string) (*dto.StallListData, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 50 {
		limit = 50
	}
	if sort == "" {
		sort = "score_desc"
	}
	if strings.TrimSpace(sort) != "score_desc" {
		return nil, errpkg.New(errpkg.CodeBadRequest, "invalid params", nil)
	}
	cursorToken, err := decodeStallCursor(cursorText)
	if err != nil {
		return nil, errpkg.New(errpkg.CodeBadRequest, "invalid params", err)
	}
	items, hasMore, err := s.repo.ListStalls(ctx, repository.StallListOptions{
		Limit:  limit,
		Cursor: cursorToken,
		Filter: repository.StallFilter{CanteenID: canteenID, FoodTypeID: foodTypeID, Sort: sort},
	})
	if err != nil {
		return nil, errpkg.New(errpkg.CodeInternal, "internal error", err)
	}
	out := make([]dto.StallItem, 0, len(items))
	for _, item := range items {
		out = append(out, dto.StallItem{
			ID:          item.ID,
			Name:        item.Name,
			CanteenID:   item.CanteenID,
			FoodTypeID:  item.FoodTypeID,
			AvgRating:   item.AvgRating,
			RatingCount: item.RatingCount,
		})
	}
	var nextCursor *string
	if len(items) > 0 && hasMore {
		last := items[len(items)-1]
		token, encodeErr := encodeStallCursor(stallListCursor{AvgRating: last.AvgRating, ID: last.ID})
		if encodeErr != nil {
			return nil, errpkg.New(errpkg.CodeInternal, "internal error", encodeErr)
		}
		nextCursor = &token
	}
	return &dto.StallListData{Items: out, NextCursor: nextCursor, HasMore: hasMore}, nil
}

func (s *StallService) GetStallDetail(ctx context.Context, stallID int64, userID *int64) (*dto.StallDetailData, error) {
	if stallID <= 0 {
		return nil, errpkg.New(errpkg.CodeBadRequest, "invalid params", nil)
	}
	item, err := s.repo.GetStallByID(ctx, stallID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, errpkg.New(errpkg.CodeNotFound, "stall not found", nil)
		}
		return nil, errpkg.New(errpkg.CodeInternal, "internal error", err)
	}
	data := &dto.StallDetailData{
		ID:          item.ID,
		Name:        item.Name,
		CanteenID:   item.CanteenID,
		FoodTypeID:  item.FoodTypeID,
		AvgRating:   item.AvgRating,
		RatingCount: item.RatingCount,
	}
	if userID != nil && *userID > 0 {
		rating, ratingErr := s.repo.GetUserRating(ctx, *userID, stallID)
		if ratingErr != nil {
			return nil, errpkg.New(errpkg.CodeInternal, "internal error", ratingErr)
		}
		data.MyRating = rating
	}
	return data, nil
}

func decodeStallCursor(raw string) (*repository.StallCursor, error) {
	if raw == "" {
		return nil, nil
	}
	decoded, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return nil, err
	}
	var payload stallListCursor
	if err := json.Unmarshal(decoded, &payload); err != nil {
		return nil, err
	}
	if payload.ID <= 0 || math.IsNaN(payload.AvgRating) || math.IsInf(payload.AvgRating, 0) {
		return nil, strconv.ErrSyntax
	}
	return &repository.StallCursor{AvgRating: payload.AvgRating, ID: payload.ID}, nil
}

func encodeStallCursor(payload stallListCursor) (string, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(data), nil
}
