package repository

import (
	"context"
	"errors"
	"sort"
	"time"

	"CampusCanteenRank/server/internal/model/ranking"
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

type MemoryRankingRepository struct {
	items []model.RankingItem
}

func NewMemoryRankingRepository() *MemoryRankingRepository {
	now := time.Now().UTC()
	return &MemoryRankingRepository{
		items: []model.RankingItem{
			{StallID: 101, StallName: "川味小炒", CanteenID: 1, CanteenName: "一食堂", FoodTypeID: 2, FoodTypeName: "川菜", AvgRating: 4.61, RatingCount: 532, ReviewCount: 221, HotScore: 97.4, LastActiveAt: now.Add(-10 * time.Minute)},
			{StallID: 201, StallName: "轻食沙拉", CanteenID: 2, CanteenName: "二食堂", FoodTypeID: 1, FoodTypeName: "轻食", AvgRating: 4.80, RatingCount: 188, ReviewCount: 95, HotScore: 93.1, LastActiveAt: now.Add(-15 * time.Minute)},
			{StallID: 102, StallName: "北方面食", CanteenID: 1, CanteenName: "一食堂", FoodTypeID: 3, FoodTypeName: "面食", AvgRating: 4.20, RatingCount: 321, ReviewCount: 160, HotScore: 90.2, LastActiveAt: now.Add(-20 * time.Minute)},
			{StallID: 301, StallName: "麻辣香锅", CanteenID: 3, CanteenName: "三食堂", FoodTypeID: 2, FoodTypeName: "川菜", AvgRating: 4.35, RatingCount: 280, ReviewCount: 143, HotScore: 91.8, LastActiveAt: now.Add(-25 * time.Minute)},
			{StallID: 302, StallName: "石锅拌饭", CanteenID: 3, CanteenName: "三食堂", FoodTypeID: 4, FoodTypeName: "韩餐", AvgRating: 4.05, RatingCount: 170, ReviewCount: 88, HotScore: 84.6, LastActiveAt: now.Add(-40 * time.Minute)},
		},
	}
}

func (r *MemoryRankingRepository) ListRankings(_ context.Context, options RankingListOptions) ([]model.RankingItem, bool, error) {
	if options.Filter.Scope != "global" && options.Filter.Scope != "canteen" && options.Filter.Scope != "foodType" {
		return nil, false, ErrInvalidScope
	}

	base := make([]model.RankingItem, 0, len(r.items))
	for _, item := range r.items {
		if options.Filter.Scope == "canteen" && item.CanteenID != options.Filter.ScopeID {
			continue
		}
		if options.Filter.Scope == "foodType" && item.FoodTypeID != options.Filter.ScopeID {
			continue
		}
		if options.Filter.FoodTypeID > 0 && item.FoodTypeID != options.Filter.FoodTypeID {
			continue
		}
		base = append(base, item)
	}

	if options.Filter.Sort == "hot_desc" {
		sort.Slice(base, func(i, j int) bool {
			if base[i].HotScore == base[j].HotScore {
				if base[i].LastActiveAt.Equal(base[j].LastActiveAt) {
					return base[i].StallID > base[j].StallID
				}
				return base[i].LastActiveAt.After(base[j].LastActiveAt)
			}
			return base[i].HotScore > base[j].HotScore
		})
	} else {
		sort.Slice(base, func(i, j int) bool {
			if base[i].AvgRating == base[j].AvgRating {
				if base[i].LastActiveAt.Equal(base[j].LastActiveAt) {
					return base[i].StallID > base[j].StallID
				}
				return base[i].LastActiveAt.After(base[j].LastActiveAt)
			}
			return base[i].AvgRating > base[j].AvgRating
		})
	}

	filtered := make([]model.RankingItem, 0, len(base))
	for _, item := range base {
		if options.Cursor == nil {
			filtered = append(filtered, item)
			continue
		}
		value := item.AvgRating
		if options.Filter.Sort == "hot_desc" {
			value = item.HotScore
		}
		if value > options.Cursor.SortValue {
			continue
		}
		if value == options.Cursor.SortValue {
			if item.LastActiveAt.After(options.Cursor.LastActiveAt) {
				continue
			}
			if item.LastActiveAt.Equal(options.Cursor.LastActiveAt) && item.StallID >= options.Cursor.StallID {
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
	out := make([]model.RankingItem, len(filtered))
	copy(out, filtered)
	return out, hasMore, nil
}
