package repository

import (
	"CampusCanteenRank/server/internal/model/stall"
	"context"
	"errors"
	"math"
	"sort"
	"sync"
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
	AvgRating float64
	ID        int64
}

type StallRepository interface {
	ListCanteens(ctx context.Context) ([]model.Canteen, error)
	ListStalls(ctx context.Context, options StallListOptions) ([]model.Stall, bool, error)
	GetStallByID(ctx context.Context, stallID int64) (*model.Stall, error)
	GetUserRating(ctx context.Context, userID int64, stallID int64) (*int, error)
	UpsertUserRating(ctx context.Context, userID int64, stallID int64, score int) (*model.Stall, error)
}

type MemoryStallRepository struct {
	mu       sync.RWMutex
	canteens []model.Canteen
	stalls   []model.Stall
	ratings  map[int64]map[int64]int
}

func NewMemoryStallRepository() *MemoryStallRepository {
	now := time.Now().UTC()
	stalls := []model.Stall{
		{ID: 101, CanteenID: 1, FoodTypeID: 2, Name: "川味小炒", AvgRating: 4.6, RatingCount: 532, Status: 1, CreatedAt: now.Add(-1 * time.Minute)},
		{ID: 102, CanteenID: 1, FoodTypeID: 3, Name: "北方面食", AvgRating: 4.2, RatingCount: 321, Status: 1, CreatedAt: now.Add(-2 * time.Minute)},
		{ID: 201, CanteenID: 2, FoodTypeID: 1, Name: "轻食沙拉", AvgRating: 4.8, RatingCount: 188, Status: 1, CreatedAt: now.Add(-3 * time.Minute)},
	}
	sort.Slice(stalls, func(i, j int) bool {
		if stalls[i].CreatedAt.Equal(stalls[j].CreatedAt) {
			return stalls[i].ID > stalls[j].ID
		}
		return stalls[i].CreatedAt.After(stalls[j].CreatedAt)
	})
	return &MemoryStallRepository{
		canteens: []model.Canteen{
			{ID: 1, Name: "一食堂", Campus: "主校区", Status: 1},
			{ID: 2, Name: "二食堂", Campus: "东校区", Status: 1},
		},
		stalls:  stalls,
		ratings: map[int64]map[int64]int{},
	}
}

func (r *MemoryStallRepository) ListCanteens(_ context.Context) ([]model.Canteen, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]model.Canteen, 0, len(r.canteens))
	for _, c := range r.canteens {
		if c.Status == 1 {
			out = append(out, c)
		}
	}
	return out, nil
}

func (r *MemoryStallRepository) ListStalls(_ context.Context, options StallListOptions) ([]model.Stall, bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	base := make([]model.Stall, 0, len(r.stalls))
	for _, s := range r.stalls {
		if s.Status != 1 {
			continue
		}
		if options.Filter.CanteenID > 0 && s.CanteenID != options.Filter.CanteenID {
			continue
		}
		if options.Filter.FoodTypeID > 0 && s.FoodTypeID != options.Filter.FoodTypeID {
			continue
		}
		base = append(base, s)
	}
	sort.Slice(base, func(i, j int) bool {
		if math.Abs(base[i].AvgRating-base[j].AvgRating) < 1e-9 {
			return base[i].ID > base[j].ID
		}
		return base[i].AvgRating > base[j].AvgRating
	})
	filtered := make([]model.Stall, 0, len(base))
	for _, s := range base {
		if options.Cursor != nil {
			if s.AvgRating > options.Cursor.AvgRating {
				continue
			}
			if math.Abs(s.AvgRating-options.Cursor.AvgRating) < 1e-9 && s.ID >= options.Cursor.ID {
				continue
			}
		}
		filtered = append(filtered, s)
	}
	limit := options.Limit
	if limit <= 0 {
		limit = 20
	}
	hasMore := len(filtered) > limit
	if hasMore {
		filtered = filtered[:limit]
	}
	out := make([]model.Stall, len(filtered))
	copy(out, filtered)
	return out, hasMore, nil
}

func (r *MemoryStallRepository) GetStallByID(_ context.Context, stallID int64) (*model.Stall, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, s := range r.stalls {
		if s.ID == stallID && s.Status == 1 {
			clone := s
			return &clone, nil
		}
	}
	return nil, ErrNotFound
}

func (r *MemoryStallRepository) GetUserRating(_ context.Context, userID int64, stallID int64) (*int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	byStall, ok := r.ratings[userID]
	if !ok {
		return nil, nil
	}
	rating, ok := byStall[stallID]
	if !ok {
		return nil, nil
	}
	result := rating
	return &result, nil
}

func (r *MemoryStallRepository) UpsertUserRating(_ context.Context, userID int64, stallID int64, score int) (*model.Stall, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	stallIndex := -1
	for i, s := range r.stalls {
		if s.ID == stallID && s.Status == 1 {
			stallIndex = i
			break
		}
	}
	if stallIndex < 0 {
		return nil, ErrNotFound
	}

	if _, ok := r.ratings[userID]; !ok {
		r.ratings[userID] = make(map[int64]int)
	}
	previousScore, hadPrevious := r.ratings[userID][stallID]
	r.ratings[userID][stallID] = score

	target := &r.stalls[stallIndex]
	if hadPrevious {
		if target.RatingCount <= 0 {
			return nil, errors.New("invalid aggregate state")
		}
		total := target.AvgRating*float64(target.RatingCount) - float64(previousScore) + float64(score)
		target.AvgRating = total / float64(target.RatingCount)
	} else {
		total := target.AvgRating*float64(target.RatingCount) + float64(score)
		target.RatingCount++
		target.AvgRating = total / float64(target.RatingCount)
	}

	clone := *target
	return &clone, nil
}
