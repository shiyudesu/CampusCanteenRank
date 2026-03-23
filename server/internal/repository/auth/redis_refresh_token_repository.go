package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisRefreshTokenRepository struct {
	client redis.Cmdable
	prefix string
}

func NewRedisRefreshTokenRepository(client redis.Cmdable, prefix string) (*RedisRefreshTokenRepository, error) {
	if client == nil {
		return nil, errors.New("nil redis client")
	}
	if prefix == "" {
		prefix = "auth:refresh"
	}
	return &RedisRefreshTokenRepository{client: client, prefix: prefix}, nil
}

func (r *RedisRefreshTokenRepository) Save(ctx context.Context, record RefreshTokenRecord) error {
	ttl := time.Until(record.ExpiredAt)
	if ttl <= 0 {
		return ErrNotFound
	}
	return r.client.Set(ctx, r.key(record.UserID, record.TokenJTI), "1", ttl).Err()
}

func (r *RedisRefreshTokenRepository) Consume(ctx context.Context, userID int64, tokenJTI string) error {
	deleted, err := r.client.Del(ctx, r.key(userID, tokenJTI)).Result()
	if err != nil {
		return err
	}
	if deleted == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *RedisRefreshTokenRepository) key(userID int64, tokenJTI string) string {
	return fmt.Sprintf("%s:%d:%s", r.prefix, userID, tokenJTI)
}
