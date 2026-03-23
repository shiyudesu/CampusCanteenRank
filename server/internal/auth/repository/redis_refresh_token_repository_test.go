package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func TestNewRedisRefreshTokenRepositoryWithNilClient(t *testing.T) {
	repo, err := NewRedisRefreshTokenRepository(nil, "")
	if err == nil {
		t.Fatalf("expected error when redis client is nil")
	}
	if repo != nil {
		t.Fatalf("expected nil repository when client is nil")
	}
}

func TestRedisRefreshTokenRepositorySaveExpiredRecord(t *testing.T) {
	client := redis.NewClient(&redis.Options{Addr: "127.0.0.1:0"})
	defer func() { _ = client.Close() }()
	repo, err := NewRedisRefreshTokenRepository(client, "")
	if err != nil {
		t.Fatalf("new repo failed: %v", err)
	}
	err = repo.Save(context.Background(), RefreshTokenRecord{
		UserID:    1,
		TokenJTI:  "jti",
		ExpiredAt: time.Now().UTC().Add(-time.Second),
	})
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound for expired record, got %v", err)
	}
}

func TestRedisRefreshTokenRepositoryDefaultPrefix(t *testing.T) {
	client := redis.NewClient(&redis.Options{Addr: "127.0.0.1:0"})
	defer func() { _ = client.Close() }()
	repo, err := NewRedisRefreshTokenRepository(client, "")
	if err != nil {
		t.Fatalf("new repo failed: %v", err)
	}
	if got := repo.key(12, "abc"); got != "auth:refresh:12:abc" {
		t.Fatalf("unexpected default key: %s", got)
	}
}
