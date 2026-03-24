package config

import (
	"testing"
)

func TestLoadUsesDefaults(t *testing.T) {
	t.Setenv("SERVER_PORT", "")
	t.Setenv("JWT_SECRET", "")
	t.Setenv("MYSQL_DSN", "")
	t.Setenv("REDIS_ADDR", "")
	t.Setenv("REDIS_PASSWORD", "")
	t.Setenv("REDIS_DB", "")
	t.Setenv("REDIS_REFRESH_PREFIX", "")
	t.Setenv("REDIS_RANKING_PREFIX", "")
	t.Setenv("LOG_LEVEL", "")
	t.Setenv("LOG_SENSITIVE_FIELDS", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load config failed: %v", err)
	}

	if cfg.ServerPort != "8080" {
		t.Fatalf("server port = %q, want 8080", cfg.ServerPort)
	}
	if cfg.JWTSecret == "" {
		t.Fatalf("jwt secret should not be empty")
	}
	if cfg.LogLevel != "info" {
		t.Fatalf("log level = %q, want info", cfg.LogLevel)
	}
	if cfg.RedisDB != 0 {
		t.Fatalf("redis db = %d, want 0", cfg.RedisDB)
	}
	if cfg.RedisRefreshPrefix == "" {
		t.Fatalf("redis refresh prefix should not be empty")
	}
	if cfg.RedisRankingPrefix == "" {
		t.Fatalf("redis ranking prefix should not be empty")
	}
	if len(cfg.LogSensitiveFields) == 0 {
		t.Fatalf("sensitive field list should not be empty")
	}
}

func TestLoadReadsEnvironment(t *testing.T) {
	t.Setenv("SERVER_PORT", "9090")
	t.Setenv("JWT_SECRET", "test-secret")
	t.Setenv("MYSQL_DSN", "dsn")
	t.Setenv("REDIS_ADDR", "127.0.0.1:6379")
	t.Setenv("REDIS_PASSWORD", "pwd")
	t.Setenv("REDIS_DB", "3")
	t.Setenv("REDIS_REFRESH_PREFIX", "auth:r")
	t.Setenv("REDIS_RANKING_PREFIX", "rank:r")
	t.Setenv("LOG_LEVEL", "debug")
	t.Setenv("LOG_SENSITIVE_FIELDS", "authorization,token")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load config failed: %v", err)
	}

	if cfg.ServerPort != "9090" {
		t.Fatalf("server port = %q, want 9090", cfg.ServerPort)
	}
	if cfg.JWTSecret != "test-secret" {
		t.Fatalf("jwt secret mismatch")
	}
	if cfg.MySQLDSN != "dsn" {
		t.Fatalf("mysql dsn mismatch")
	}
	if cfg.RedisAddr != "127.0.0.1:6379" {
		t.Fatalf("redis addr mismatch")
	}
	if cfg.RedisPassword != "pwd" {
		t.Fatalf("redis password mismatch")
	}
	if cfg.RedisDB != 3 {
		t.Fatalf("redis db = %d, want 3", cfg.RedisDB)
	}
	if cfg.RedisRefreshPrefix != "auth:r" {
		t.Fatalf("redis refresh prefix mismatch")
	}
	if cfg.RedisRankingPrefix != "rank:r" {
		t.Fatalf("redis ranking prefix mismatch")
	}
	if cfg.LogLevel != "debug" {
		t.Fatalf("log level = %q, want debug", cfg.LogLevel)
	}
	if len(cfg.LogSensitiveFields) != 2 {
		t.Fatalf("sensitive fields len = %d, want 2", len(cfg.LogSensitiveFields))
	}
}
