package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	model "CampusCanteenRank/server/internal/model/ranking"

	"gorm.io/gorm"
)

type MySQLRankingRepository struct {
	db *gorm.DB
}

type rankingAggregates struct {
	commentAgg *gorm.DB
	ratingAgg  *gorm.DB
}

type rankingSortStrategy struct {
	sortExpr string
}

const (
	rankingHotScoreExpr   = "(s.avg_rating * 0.75 + LOG10(COALESCE(ca.review_count, 0) + 1) * 0.25)"
	rankingLastActiveExpr = "GREATEST(COALESCE(ca.last_comment_at, TIMESTAMP('1970-01-01 00:00:01')), COALESCE(ra.last_rating_at, TIMESTAMP('1970-01-01 00:00:01')), COALESCE(s.created_at, TIMESTAMP('1970-01-01 00:00:01')))"
)

type mysqlRankingRecord struct {
	StallID      int64     `gorm:"column:stall_id"`
	StallName    string    `gorm:"column:stall_name"`
	CanteenID    int64     `gorm:"column:canteen_id"`
	CanteenName  string    `gorm:"column:canteen_name"`
	FoodTypeID   int64     `gorm:"column:food_type_id"`
	FoodTypeName string    `gorm:"column:food_type_name"`
	AvgRating    float64   `gorm:"column:avg_rating"`
	RatingCount  int64     `gorm:"column:rating_count"`
	ReviewCount  int64     `gorm:"column:review_count"`
	HotScore     float64   `gorm:"column:hot_score"`
	LastActiveAt time.Time `gorm:"column:last_active_at"`
}

func NewMySQLRankingRepository(db *gorm.DB) (*MySQLRankingRepository, error) {
	if db == nil {
		return nil, errors.New("nil mysql db")
	}
	return &MySQLRankingRepository{db: db}, nil
}

func (r *MySQLRankingRepository) ListRankings(ctx context.Context, options RankingListOptions) ([]model.RankingItem, bool, error) {
	if options.Filter.Scope != "global" && options.Filter.Scope != "canteen" && options.Filter.Scope != "foodType" {
		return nil, false, ErrInvalidScope
	}

	limit := normalizeRankingLimit(options.Limit)
	windowStart := resolveRankingWindowStart(options.Filter.Days)
	strategy := resolveRankingSortStrategy(options.Filter.Sort)
	aggregates := r.buildRankingAggregates(ctx, windowStart)

	base := r.buildRankingBaseQuery(ctx, aggregates)
	base = applyRankingFilters(base, options.Filter)
	base = applyRankingCursor(base, options.Cursor, strategy)

	var records []mysqlRankingRecord
	if err := applyRankingOrderAndLimit(base, strategy, limit).Scan(&records).Error; err != nil {
		return nil, false, err
	}

	items, hasMore := mapRankingItems(records, limit)
	return items, hasMore, nil
}

func normalizeRankingLimit(limit int) int {
	if limit <= 0 {
		return 20
	}
	return limit
}

func resolveRankingWindowStart(days int) time.Time {
	if days <= 0 {
		days = 30
	}
	return time.Now().UTC().Add(-time.Duration(days) * 24 * time.Hour)
}

func resolveRankingSortStrategy(sort string) rankingSortStrategy {
	strategy := rankingSortStrategy{sortExpr: "s.avg_rating"}
	if sort == "hot_desc" {
		strategy.sortExpr = rankingHotScoreExpr
	}
	return strategy
}

func (r *MySQLRankingRepository) buildRankingAggregates(ctx context.Context, windowStart time.Time) rankingAggregates {
	return rankingAggregates{
		commentAgg: r.db.WithContext(ctx).
			Table("comments").
			Select("stall_id, COUNT(*) AS review_count, MAX(created_at) AS last_comment_at").
			Where("status = ? AND created_at >= ?", 1, windowStart).
			Group("stall_id"),
		ratingAgg: r.db.WithContext(ctx).
			Table("ratings").
			Select("stall_id, MAX(updated_at) AS last_rating_at").
			Where("updated_at >= ?", windowStart).
			Group("stall_id"),
	}
}

func (r *MySQLRankingRepository) buildRankingBaseQuery(ctx context.Context, aggregates rankingAggregates) *gorm.DB {
	return r.db.WithContext(ctx).
		Table("stalls AS s").
		Select(
			"s.id AS stall_id, s.name AS stall_name, s.canteen_id, COALESCE(c.name, '') AS canteen_name, "+
				"s.food_type_id, COALESCE(ft.name, '') AS food_type_name, s.avg_rating, s.rating_count, COALESCE(ca.review_count, 0) AS review_count, "+
				rankingHotScoreExpr+" AS hot_score, "+
				rankingLastActiveExpr+" AS last_active_at",
		).
		Joins("LEFT JOIN canteens AS c ON c.id = s.canteen_id").
		Joins("LEFT JOIN food_types AS ft ON ft.id = s.food_type_id").
		Joins("LEFT JOIN (?) AS ca ON ca.stall_id = s.id", aggregates.commentAgg).
		Joins("LEFT JOIN (?) AS ra ON ra.stall_id = s.id", aggregates.ratingAgg).
		Where("s.status = ?", 1)
}

func applyRankingFilters(base *gorm.DB, filter RankingFilter) *gorm.DB {
	if filter.Scope == "canteen" {
		base = base.Where("s.canteen_id = ?", filter.ScopeID)
	}
	if filter.Scope == "foodType" {
		base = base.Where("s.food_type_id = ?", filter.ScopeID)
	}
	if filter.FoodTypeID > 0 {
		base = base.Where("s.food_type_id = ?", filter.FoodTypeID)
	}
	return base
}

func applyRankingCursor(base *gorm.DB, cursor *RankingCursor, strategy rankingSortStrategy) *gorm.DB {
	if cursor == nil {
		return base
	}

	// Cursor comparison follows the same ordering tuple: sort value, last_active_at, stall id.
	cursorWhere := fmt.Sprintf(
		"(%s < ?) OR (%s = ? AND (%s < ? OR (%s = ? AND s.id < ?)))",
		strategy.sortExpr,
		strategy.sortExpr,
		rankingLastActiveExpr,
		rankingLastActiveExpr,
	)
	return base.Where(cursorWhere, cursor.SortValue, cursor.SortValue, cursor.LastActiveAt, cursor.LastActiveAt, cursor.StallID)
}

func applyRankingOrderAndLimit(base *gorm.DB, strategy rankingSortStrategy, limit int) *gorm.DB {
	return base.Order(strategy.sortExpr + " DESC").Order("last_active_at DESC").Order("s.id DESC").Limit(limit + 1)
}

func mapRankingItems(records []mysqlRankingRecord, limit int) ([]model.RankingItem, bool) {
	hasMore := len(records) > limit
	if hasMore {
		records = records[:limit]
	}

	items := make([]model.RankingItem, 0, len(records))
	for _, rec := range records {
		items = append(items, model.RankingItem{
			StallID:      rec.StallID,
			StallName:    rec.StallName,
			CanteenID:    rec.CanteenID,
			CanteenName:  rec.CanteenName,
			FoodTypeID:   rec.FoodTypeID,
			FoodTypeName: rec.FoodTypeName,
			AvgRating:    rec.AvgRating,
			RatingCount:  rec.RatingCount,
			ReviewCount:  rec.ReviewCount,
			HotScore:     rec.HotScore,
			LastActiveAt: rec.LastActiveAt.UTC(),
		})
	}
	return items, hasMore
}
