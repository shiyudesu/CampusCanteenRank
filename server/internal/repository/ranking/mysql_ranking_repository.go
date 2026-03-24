package repository

import (
	"context"
	"errors"
	"time"

	model "CampusCanteenRank/server/internal/model/ranking"
	"gorm.io/gorm"
)

type MySQLRankingRepository struct {
	db *gorm.DB
}

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

	limit := options.Limit
	if limit <= 0 {
		limit = 20
	}

	windowStart := time.Now().UTC().Add(-time.Duration(options.Filter.Days) * 24 * time.Hour)
	if options.Filter.Days <= 0 {
		windowStart = time.Now().UTC().Add(-30 * 24 * time.Hour)
	}

	commentAgg := r.db.WithContext(ctx).
		Table("comments").
		Select("stall_id, COUNT(*) AS review_count, MAX(created_at) AS last_comment_at").
		Where("status = ? AND created_at >= ?", 1, windowStart).
		Group("stall_id")

	ratingAgg := r.db.WithContext(ctx).
		Table("ratings").
		Select("stall_id, MAX(updated_at) AS last_rating_at").
		Where("updated_at >= ?", windowStart).
		Group("stall_id")

	sortExpr := "s.avg_rating"
	if options.Filter.Sort == "hot_desc" {
		sortExpr = "(s.avg_rating * 0.75 + LOG10(COALESCE(ca.review_count, 0) + 1) * 0.25)"
	}

	base := r.db.WithContext(ctx).
		Table("stalls AS s").
		Select(
			"s.id AS stall_id, s.name AS stall_name, s.canteen_id, COALESCE(c.name, '') AS canteen_name, "+
				"s.food_type_id, COALESCE(ft.name, '') AS food_type_name, s.avg_rating, s.rating_count, COALESCE(ca.review_count, 0) AS review_count, "+
				sortExpr+" AS hot_score, "+
				"GREATEST(COALESCE(ca.last_comment_at, TIMESTAMP('1970-01-01 00:00:01')), COALESCE(ra.last_rating_at, TIMESTAMP('1970-01-01 00:00:01')), COALESCE(s.created_at, TIMESTAMP('1970-01-01 00:00:01'))) AS last_active_at",
		).
		Joins("LEFT JOIN canteens AS c ON c.id = s.canteen_id").
		Joins("LEFT JOIN food_types AS ft ON ft.id = s.food_type_id").
		Joins("LEFT JOIN (?) AS ca ON ca.stall_id = s.id", commentAgg).
		Joins("LEFT JOIN (?) AS ra ON ra.stall_id = s.id", ratingAgg).
		Where("s.status = ?", 1)

	if options.Filter.Scope == "canteen" {
		base = base.Where("s.canteen_id = ?", options.Filter.ScopeID)
	}
	if options.Filter.Scope == "foodType" {
		base = base.Where("s.food_type_id = ?", options.Filter.ScopeID)
	}
	if options.Filter.FoodTypeID > 0 {
		base = base.Where("s.food_type_id = ?", options.Filter.FoodTypeID)
	}

	if options.Cursor != nil {
		base = base.Where(
			"("+sortExpr+" < ?) OR ("+sortExpr+" = ? AND ("+
				"GREATEST(COALESCE(ca.last_comment_at, TIMESTAMP('1970-01-01 00:00:01')), COALESCE(ra.last_rating_at, TIMESTAMP('1970-01-01 00:00:01')), COALESCE(s.created_at, TIMESTAMP('1970-01-01 00:00:01'))) < ? OR ("+
				"GREATEST(COALESCE(ca.last_comment_at, TIMESTAMP('1970-01-01 00:00:01')), COALESCE(ra.last_rating_at, TIMESTAMP('1970-01-01 00:00:01')), COALESCE(s.created_at, TIMESTAMP('1970-01-01 00:00:01'))) = ? AND s.id < ?)))",
			options.Cursor.SortValue,
			options.Cursor.SortValue,
			options.Cursor.LastActiveAt,
			options.Cursor.LastActiveAt,
			options.Cursor.StallID,
		)
	}

	var records []mysqlRankingRecord
	if err := base.Order(sortExpr + " DESC").Order("last_active_at DESC").Order("s.id DESC").Limit(limit + 1).Scan(&records).Error; err != nil {
		return nil, false, err
	}

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

	return items, hasMore, nil
}
