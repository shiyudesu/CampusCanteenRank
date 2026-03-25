package repository

import (
	model "CampusCanteenRank/server/internal/model/ranking"
	"context"
	"errors"
	"time"
)

var ErrInvalidScope = errors.New("invalid scope")

type RankingFilter struct {
	Scope      string
	ScopeID    int64
	FoodTypeID int64
	Days       int
	Sort       string
}

type RankingCursor struct {
	SortValue    float64
	LastActiveAt time.Time
	StallID      int64
}

type RankingListOptions struct {
	Limit  int
	Cursor *RankingCursor
	Filter RankingFilter
}

type RankingRepository interface {
	ListRankings(ctx context.Context, options RankingListOptions) ([]model.RankingItem, bool, error)
}
