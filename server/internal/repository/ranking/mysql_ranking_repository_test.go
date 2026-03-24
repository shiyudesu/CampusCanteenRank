package repository

import (
	"context"
	"testing"
)

func TestNewMySQLRankingRepositoryNilDB(t *testing.T) {
	repo, err := NewMySQLRankingRepository(nil)
	if err == nil {
		t.Fatalf("expected error when db is nil")
	}
	if repo != nil {
		t.Fatalf("repo should be nil when db is nil")
	}
}

func TestMySQLRankingRepositoryInvalidScope(t *testing.T) {
	repo := &MySQLRankingRepository{}
	_, _, err := repo.ListRankings(context.Background(), RankingListOptions{
		Limit: 20,
		Filter: RankingFilter{
			Scope: "invalid",
			Days:  30,
			Sort:  "score_desc",
		},
	})
	if err == nil {
		t.Fatalf("expected invalid scope error")
	}
	if err != ErrInvalidScope {
		t.Fatalf("unexpected error: %v", err)
	}
}
