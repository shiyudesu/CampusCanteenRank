package repository

import (
	"context"
	"errors"
	"strconv"
	"time"

	model "CampusCanteenRank/server/internal/model/auth"
)

var ErrNotFound = errors.New("not found")
var ErrAlreadyExists = errors.New("already exists")

type UserRepository interface {
	Create(ctx context.Context, user *model.User) error
	GetByEmail(ctx context.Context, email string) (*model.User, error)
	GetByID(ctx context.Context, id int64) (*model.User, error)
	GetByIDs(ctx context.Context, ids []int64) (map[int64]*model.User, error)
}

type RefreshTokenRecord struct {
	UserID    int64
	TokenJTI  string
	DeviceID  string
	ExpiredAt time.Time
}

type RefreshTokenRepository interface {
	Save(ctx context.Context, record RefreshTokenRecord) error
	Consume(ctx context.Context, userID int64, tokenJTI string, deviceID string) error
}

func recordKey(userID int64, tokenJTI string, deviceID string) string {
	return fmtInt(userID) + ":" + deviceID + ":" + tokenJTI
}

func fmtInt(v int64) string {
	return strconv.FormatInt(v, 10)
}
