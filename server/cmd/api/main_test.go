package main

import (
	"testing"

	authrepo "CampusCanteenRank/server/internal/repository/auth"
)

func TestBuildAuthRepositoriesFallsBackToMemoryWhenEnvMissing(t *testing.T) {
	t.Setenv("MYSQL_DSN", "")
	t.Setenv("REDIS_ADDR", "")

	userRepo, refreshRepo, cleanup := buildAuthRepositories()
	t.Cleanup(cleanup)

	if _, ok := userRepo.(*authrepo.MemoryUserRepository); !ok {
		t.Fatalf("expected MemoryUserRepository fallback when env missing")
	}
	if _, ok := refreshRepo.(*authrepo.MemoryRefreshTokenRepository); !ok {
		t.Fatalf("expected MemoryRefreshTokenRepository fallback when env missing")
	}
}

func TestBuildAuthRepositoriesFallsBackToMemoryWhenRedisMissing(t *testing.T) {
	t.Setenv("MYSQL_DSN", "root:pass@tcp(127.0.0.1:3306)/canteen")
	t.Setenv("REDIS_ADDR", "")

	userRepo, refreshRepo, cleanup := buildAuthRepositories()
	t.Cleanup(cleanup)

	if _, ok := userRepo.(*authrepo.MemoryUserRepository); !ok {
		t.Fatalf("expected MemoryUserRepository fallback when redis env missing")
	}
	if _, ok := refreshRepo.(*authrepo.MemoryRefreshTokenRepository); !ok {
		t.Fatalf("expected MemoryRefreshTokenRepository fallback when redis env missing")
	}
}
