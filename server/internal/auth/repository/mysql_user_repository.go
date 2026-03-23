package repository

import (
	"context"
	"errors"
	"time"

	"CampusCanteenRank/server/internal/auth/model"
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
	if err := db.AutoMigrate(&mysqlUserRecord{}); err != nil {
		return nil, err
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

func isMySQLDuplicate(err error) bool {
	var mysqlErr *mysqlDriver.MySQLError
	if errors.As(err, &mysqlErr) {
		return mysqlErr.Number == 1062
	}
	return false
}
