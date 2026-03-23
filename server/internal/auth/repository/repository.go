package repository

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"time"

	"CampusCanteenRank/server/internal/auth/model"
)

var ErrNotFound = errors.New("not found")

type UserRepository interface {
	Create(ctx context.Context, user *model.User) error
	GetByEmail(ctx context.Context, email string) (*model.User, error)
}

type RefreshTokenRecord struct {
	UserID    int64
	TokenJTI  string
	ExpiredAt time.Time
}

type RefreshTokenRepository interface {
	Save(ctx context.Context, record RefreshTokenRecord) error
	Consume(ctx context.Context, userID int64, tokenJTI string) error
}

type MemoryUserRepository struct {
	mu      sync.RWMutex
	nextID  int64
	byEmail map[string]*model.User
}

func NewMemoryUserRepository() *MemoryUserRepository {
	return &MemoryUserRepository{nextID: 1000, byEmail: make(map[string]*model.User)}
}

func (r *MemoryUserRepository) Create(_ context.Context, user *model.User) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.byEmail[user.Email]; exists {
		return errors.New("email exists")
	}
	r.nextID++
	clone := *user
	clone.ID = r.nextID
	clone.CreatedAt = time.Now().UTC()
	r.byEmail[user.Email] = &clone
	user.ID = clone.ID
	user.CreatedAt = clone.CreatedAt
	return nil
}

func (r *MemoryUserRepository) GetByEmail(_ context.Context, email string) (*model.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	u, ok := r.byEmail[email]
	if !ok {
		return nil, ErrNotFound
	}
	clone := *u
	return &clone, nil
}

type MemoryRefreshTokenRepository struct {
	mu     sync.Mutex
	active map[string]RefreshTokenRecord
}

func NewMemoryRefreshTokenRepository() *MemoryRefreshTokenRepository {
	return &MemoryRefreshTokenRepository{active: make(map[string]RefreshTokenRecord)}
}

func (r *MemoryRefreshTokenRepository) Save(_ context.Context, record RefreshTokenRecord) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := recordKey(record.UserID, record.TokenJTI)
	r.active[key] = record
	return nil
}

func (r *MemoryRefreshTokenRepository) Consume(_ context.Context, userID int64, tokenJTI string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := recordKey(userID, tokenJTI)
	record, ok := r.active[key]
	if !ok {
		return ErrNotFound
	}
	if time.Now().UTC().After(record.ExpiredAt) {
		delete(r.active, key)
		return ErrNotFound
	}
	delete(r.active, key)
	return nil
}

func recordKey(userID int64, tokenJTI string) string {
	return fmtInt(userID) + ":" + tokenJTI
}

func fmtInt(v int64) string {
	return strconv.FormatInt(v, 10)
}
