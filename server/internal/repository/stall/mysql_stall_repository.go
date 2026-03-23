package repository

import (
	"context"
	"errors"
	"math"
	"time"

	model "CampusCanteenRank/server/internal/model/stall"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type mysqlCanteenRecord struct {
	ID     int64  `gorm:"column:id;primaryKey;autoIncrement"`
	Name   string `gorm:"column:name;type:varchar(128);not null"`
	Campus string `gorm:"column:campus;type:varchar(64);not null"`
	Status int8   `gorm:"column:status;type:tinyint;not null;default:1;index"`
}

func (mysqlCanteenRecord) TableName() string {
	return "canteens"
}

type mysqlStallRecord struct {
	ID          int64     `gorm:"column:id;primaryKey;autoIncrement"`
	CanteenID   int64     `gorm:"column:canteen_id;not null;index"`
	FoodTypeID  int64     `gorm:"column:food_type_id;not null;index"`
	Name        string    `gorm:"column:name;type:varchar(128);not null"`
	AvgRating   float64   `gorm:"column:avg_rating;type:decimal(4,2);not null;default:0;index"`
	RatingCount int64     `gorm:"column:rating_count;not null;default:0"`
	Status      int8      `gorm:"column:status;type:tinyint;not null;default:1;index"`
	CreatedAt   time.Time `gorm:"column:created_at;autoCreateTime;index"`
}

func (mysqlStallRecord) TableName() string {
	return "stalls"
}

type mysqlUserRatingRecord struct {
	ID        int64     `gorm:"column:id;primaryKey;autoIncrement"`
	UserID    int64     `gorm:"column:user_id;not null;index:idx_user_updated,priority:1;index:uk_user_stall,unique,priority:1"`
	StallID   int64     `gorm:"column:stall_id;not null;index:idx_user_updated,priority:3;index:uk_user_stall,unique,priority:2"`
	Score     int       `gorm:"column:score;type:tinyint;not null"`
	CreatedAt time.Time `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt time.Time `gorm:"column:updated_at;autoUpdateTime;index:idx_user_updated,priority:2"`
}

func (mysqlUserRatingRecord) TableName() string {
	return "ratings"
}

type MySQLStallRepository struct {
	db *gorm.DB
}

func NewMySQLStallRepository(db *gorm.DB) (*MySQLStallRepository, error) {
	if db == nil {
		return nil, errors.New("nil mysql db")
	}
	if err := db.AutoMigrate(&mysqlCanteenRecord{}, &mysqlStallRecord{}, &mysqlUserRatingRecord{}); err != nil {
		return nil, err
	}
	return &MySQLStallRepository{db: db}, nil
}

func (r *MySQLStallRepository) ListCanteens(ctx context.Context) ([]model.Canteen, error) {
	var records []mysqlCanteenRecord
	if err := r.db.WithContext(ctx).Where("status = ?", 1).Order("id ASC").Find(&records).Error; err != nil {
		return nil, err
	}
	out := make([]model.Canteen, 0, len(records))
	for _, rec := range records {
		out = append(out, model.Canteen{ID: rec.ID, Name: rec.Name, Campus: rec.Campus, Status: rec.Status})
	}
	return out, nil
}

func (r *MySQLStallRepository) ListStalls(ctx context.Context, options StallListOptions) ([]model.Stall, bool, error) {
	limit := options.Limit
	if limit <= 0 {
		limit = 20
	}

	query := r.db.WithContext(ctx).
		Model(&mysqlStallRecord{}).
		Where("status = ?", 1)
	if options.Filter.CanteenID > 0 {
		query = query.Where("canteen_id = ?", options.Filter.CanteenID)
	}
	if options.Filter.FoodTypeID > 0 {
		query = query.Where("food_type_id = ?", options.Filter.FoodTypeID)
	}
	if options.Cursor != nil {
		query = query.Where("(avg_rating < ?) OR (avg_rating = ? AND id < ?)", options.Cursor.AvgRating, options.Cursor.AvgRating, options.Cursor.ID)
	}

	var records []mysqlStallRecord
	if err := query.Order("avg_rating DESC, id DESC").Limit(limit + 1).Find(&records).Error; err != nil {
		return nil, false, err
	}

	hasMore := len(records) > limit
	if hasMore {
		records = records[:limit]
	}
	out := make([]model.Stall, 0, len(records))
	for _, rec := range records {
		out = append(out, model.Stall{
			ID:          rec.ID,
			CanteenID:   rec.CanteenID,
			FoodTypeID:  rec.FoodTypeID,
			Name:        rec.Name,
			AvgRating:   math.Round(rec.AvgRating*100) / 100,
			RatingCount: rec.RatingCount,
			Status:      rec.Status,
			CreatedAt:   rec.CreatedAt,
		})
	}
	return out, hasMore, nil
}

func (r *MySQLStallRepository) GetStallByID(ctx context.Context, stallID int64) (*model.Stall, error) {
	var rec mysqlStallRecord
	err := r.db.WithContext(ctx).Where("id = ? AND status = ?", stallID, 1).First(&rec).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &model.Stall{
		ID:          rec.ID,
		CanteenID:   rec.CanteenID,
		FoodTypeID:  rec.FoodTypeID,
		Name:        rec.Name,
		AvgRating:   math.Round(rec.AvgRating*100) / 100,
		RatingCount: rec.RatingCount,
		Status:      rec.Status,
		CreatedAt:   rec.CreatedAt,
	}, nil
}

func (r *MySQLStallRepository) GetUserRating(ctx context.Context, userID int64, stallID int64) (*int, error) {
	var rec mysqlUserRatingRecord
	err := r.db.WithContext(ctx).Where("user_id = ? AND stall_id = ?", userID, stallID).First(&rec).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	score := rec.Score
	return &score, nil
}

func (r *MySQLStallRepository) UpsertUserRating(ctx context.Context, userID int64, stallID int64, score int) (*model.Stall, error) {
	var updated model.Stall
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var stallRec mysqlStallRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND status = ?", stallID, 1).
			First(&stallRec).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrNotFound
			}
			return err
		}

		var ratingRec mysqlUserRatingRecord
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("user_id = ? AND stall_id = ?", userID, stallID).
			First(&ratingRec).Error

		if errors.Is(err, gorm.ErrRecordNotFound) {
			ratingRec = mysqlUserRatingRecord{UserID: userID, StallID: stallID, Score: score}
			if err := tx.Create(&ratingRec).Error; err != nil {
				return err
			}
			total := stallRec.AvgRating*float64(stallRec.RatingCount) + float64(score)
			stallRec.RatingCount++
			stallRec.AvgRating = total / float64(stallRec.RatingCount)
		} else if err != nil {
			return err
		} else {
			if stallRec.RatingCount <= 0 {
				return errors.New("invalid aggregate state")
			}
			total := stallRec.AvgRating*float64(stallRec.RatingCount) - float64(ratingRec.Score) + float64(score)
			stallRec.AvgRating = total / float64(stallRec.RatingCount)
			ratingRec.Score = score
			if err := tx.Save(&ratingRec).Error; err != nil {
				return err
			}
		}

		if err := tx.Model(&mysqlStallRecord{}).
			Where("id = ?", stallRec.ID).
			Updates(map[string]interface{}{"avg_rating": stallRec.AvgRating, "rating_count": stallRec.RatingCount}).Error; err != nil {
			return err
		}

		updated = model.Stall{
			ID:          stallRec.ID,
			CanteenID:   stallRec.CanteenID,
			FoodTypeID:  stallRec.FoodTypeID,
			Name:        stallRec.Name,
			AvgRating:   math.Round(stallRec.AvgRating*100) / 100,
			RatingCount: stallRec.RatingCount,
			Status:      stallRec.Status,
			CreatedAt:   stallRec.CreatedAt,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &updated, nil
}

func (r *MySQLStallRepository) ListUserRatings(ctx context.Context, userID int64, limit int, cursor *UserRatingCursor) ([]model.UserRating, bool, error) {
	if limit <= 0 {
		limit = 20
	}

	query := r.db.WithContext(ctx).Model(&mysqlUserRatingRecord{}).Where("user_id = ?", userID)
	if cursor != nil {
		query = query.Where("(updated_at < ?) OR (updated_at = ? AND stall_id < ?)", cursor.UpdatedAt, cursor.UpdatedAt, cursor.StallID)
	}

	var records []mysqlUserRatingRecord
	if err := query.Order("updated_at DESC, stall_id DESC").Limit(limit + 1).Find(&records).Error; err != nil {
		return nil, false, err
	}

	hasMore := len(records) > limit
	if hasMore {
		records = records[:limit]
	}
	out := make([]model.UserRating, 0, len(records))
	for _, rec := range records {
		out = append(out, model.UserRating{UserID: rec.UserID, StallID: rec.StallID, Score: rec.Score, UpdatedAt: rec.UpdatedAt})
	}
	return out, hasMore, nil
}
