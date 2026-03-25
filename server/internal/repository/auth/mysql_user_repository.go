package repository

import (
	"context"
	"errors"
	"time"

	model "CampusCanteenRank/server/internal/model/auth"

	mysqlDriver "github.com/go-sql-driver/mysql"
	"gorm.io/gorm"
)

type mysqlUserRecord struct {
	ID           int64     `gorm:"column:id;primaryKey;autoIncrement"`
	Nickname     string    `gorm:"column:nickname;type:varchar(64);not null"`
	Email        string    `gorm:"column:email;type:varchar(128);uniqueIndex;not null"`
	PasswordHash string    `gorm:"column:password_hash;type:varchar(255);not null"`
	Status       int       `gorm:"column:status;type:tinyint;not null;default:1"`
	CreatedAt    time.Time `gorm:"column:created_at;autoCreateTime"`
}

func (mysqlUserRecord) TableName() string {
	return "users"
}

type MySQLUserRepository struct {
	db *gorm.DB
}

func NewMySQLUserRepository(db *gorm.DB) (*MySQLUserRepository, error) {
	if db == nil {
		return nil, errors.New("nil mysql db")
	}
	return &MySQLUserRepository{db: db}, nil
}

func (r *MySQLUserRepository) Create(ctx context.Context, user *model.User) error {
	rec := mysqlUserRecord{
		Nickname:     user.Nickname,
		Email:        user.Email,
		PasswordHash: user.PasswordHash,
		Status:       user.Status,
	}
	if err := r.db.WithContext(ctx).Create(&rec).Error; err != nil {
		if isMySQLDuplicate(err) {
			return ErrAlreadyExists
		}
		return err
	}
	user.ID = rec.ID
	user.CreatedAt = rec.CreatedAt
	return nil
}

func (r *MySQLUserRepository) GetByEmail(ctx context.Context, email string) (*model.User, error) {
	var rec mysqlUserRecord
	err := r.db.WithContext(ctx).Where("email = ?", email).First(&rec).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &model.User{
		ID:           rec.ID,
		Nickname:     rec.Nickname,
		Email:        rec.Email,
		PasswordHash: rec.PasswordHash,
		Status:       rec.Status,
		CreatedAt:    rec.CreatedAt,
	}, nil
}

func (r *MySQLUserRepository) GetByID(ctx context.Context, id int64) (*model.User, error) {
	var rec mysqlUserRecord
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&rec).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &model.User{
		ID:           rec.ID,
		Nickname:     rec.Nickname,
		Email:        rec.Email,
		PasswordHash: rec.PasswordHash,
		Status:       rec.Status,
		CreatedAt:    rec.CreatedAt,
	}, nil
}

func (r *MySQLUserRepository) GetByIDs(ctx context.Context, ids []int64) (map[int64]*model.User, error) {
	result := make(map[int64]*model.User, len(ids))
	if len(ids) == 0 {
		return result, nil
	}

	unique := make(map[int64]struct{}, len(ids))
	normalizedIDs := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, exists := unique[id]; exists {
			continue
		}
		unique[id] = struct{}{}
		normalizedIDs = append(normalizedIDs, id)
	}
	if len(normalizedIDs) == 0 {
		return result, nil
	}

	var records []mysqlUserRecord
	if err := r.db.WithContext(ctx).Where("id IN ?", normalizedIDs).Find(&records).Error; err != nil {
		return nil, err
	}
	for _, rec := range records {
		item := rec
		result[item.ID] = &model.User{
			ID:           item.ID,
			Nickname:     item.Nickname,
			Email:        item.Email,
			PasswordHash: item.PasswordHash,
			Status:       item.Status,
			CreatedAt:    item.CreatedAt,
		}
	}
	return result, nil
}

func isMySQLDuplicate(err error) bool {
	var mysqlErr *mysqlDriver.MySQLError
	if errors.As(err, &mysqlErr) {
		return mysqlErr.Number == 1062
	}
	return false
}
