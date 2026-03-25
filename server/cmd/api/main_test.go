package main

import (
	"testing"

	"CampusCanteenRank/server/internal/config"
)

func TestBuildRepositoriesFailsWhenEnvMissing(t *testing.T) {
	cfg := config.RuntimeConfig{MySQLDSN: "", RedisAddr: ""}

	_, _, _, _, _, cleanup, err := buildRepositories(cfg)
	t.Cleanup(cleanup)
	if err == nil {
		t.Fatalf("expected error when MYSQL_DSN or REDIS_ADDR is missing")
	}
}

func TestBuildRepositoriesFailsWhenRedisMissing(t *testing.T) {
	cfg := config.RuntimeConfig{MySQLDSN: "root:pass@tcp(127.0.0.1:3306)/canteen", RedisAddr: ""}

	_, _, _, _, _, cleanup, err := buildRepositories(cfg)
	t.Cleanup(cleanup)
	if err == nil {
		t.Fatalf("expected error when redis env is missing")
	}
}
