package repository

import (
	model "CampusCanteenRank/server/internal/model/stall"
	"context"
	"errors"
	"time"
)

var ErrNotFound = errors.New("not found")

type StallFilter struct {
	CanteenID  int64
	FoodTypeID int64
	Sort       string
}

type StallListOptions struct {
	Limit  int
	Cursor *StallCursor
	Filter StallFilter
}

type StallCursor struct {
	AvgRatingX100 int64
	ID            int64
}

type UserRatingCursor struct {
	UpdatedAt time.Time
	StallID   int64
}

type StallRepository interface {
	ListCanteens(ctx context.Context) ([]model.Canteen, error)
	ListStalls(ctx context.Context, options StallListOptions) ([]model.Stall, bool, error)
	GetStallByID(ctx context.Context, stallID int64) (*model.Stall, error)
	GetStallsByIDs(ctx context.Context, stallIDs []int64) (map[int64]*model.Stall, error)
	GetUserRating(ctx context.Context, userID int64, stallID int64) (*int, error)
	UpsertUserRating(ctx context.Context, userID int64, stallID int64, score int) (*model.Stall, error)
	ListUserRatings(ctx context.Context, userID int64, limit int, cursor *UserRatingCursor) ([]model.UserRating, bool, error)
}
