package main

import (
	"testing"

	authrepo "CampusCanteenRank/server/internal/repository/auth"
	commentrepo "CampusCanteenRank/server/internal/repository/comment"
	stallrepo "CampusCanteenRank/server/internal/repository/stall"
)

func TestBuildRepositoriesFallsBackToMemoryWhenEnvMissing(t *testing.T) {
	t.Setenv("MYSQL_DSN", "")
	t.Setenv("REDIS_ADDR", "")

	userRepo, refreshRepo, stallRepository, commentRepository, cleanup := buildRepositories()
	t.Cleanup(cleanup)

	if _, ok := userRepo.(*authrepo.MemoryUserRepository); !ok {
		t.Fatalf("expected MemoryUserRepository fallback when env missing")
	}
	if _, ok := refreshRepo.(*authrepo.MemoryRefreshTokenRepository); !ok {
		t.Fatalf("expected MemoryRefreshTokenRepository fallback when env missing")
	}
	if _, ok := stallRepository.(*stallrepo.MemoryStallRepository); !ok {
		t.Fatalf("expected MemoryStallRepository fallback when env missing")
	}
	if _, ok := commentRepository.(*commentrepo.MemoryCommentRepository); !ok {
		t.Fatalf("expected MemoryCommentRepository fallback when env missing")
	}
}

func TestBuildRepositoriesFallsBackToMemoryWhenRedisMissing(t *testing.T) {
	t.Setenv("MYSQL_DSN", "root:pass@tcp(127.0.0.1:3306)/canteen")
	t.Setenv("REDIS_ADDR", "")

	userRepo, refreshRepo, stallRepository, commentRepository, cleanup := buildRepositories()
	t.Cleanup(cleanup)

	if _, ok := userRepo.(*authrepo.MemoryUserRepository); !ok {
		t.Fatalf("expected MemoryUserRepository fallback when redis env missing")
	}
	if _, ok := refreshRepo.(*authrepo.MemoryRefreshTokenRepository); !ok {
		t.Fatalf("expected MemoryRefreshTokenRepository fallback when redis env missing")
	}
	if _, ok := stallRepository.(*stallrepo.MemoryStallRepository); !ok {
		t.Fatalf("expected MemoryStallRepository fallback when redis env missing")
	}
	if _, ok := commentRepository.(*commentrepo.MemoryCommentRepository); !ok {
		t.Fatalf("expected MemoryCommentRepository fallback when redis env missing")
	}
}
