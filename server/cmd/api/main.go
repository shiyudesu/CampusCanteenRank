package main

import (
	"context"
	"log"
	"os"
	"strconv"
	"time"

	"CampusCanteenRank/server/internal/auth/repository"
	logpkg "CampusCanteenRank/server/internal/pkg/logger"
	"CampusCanteenRank/server/internal/router"
	"github.com/redis/go-redis/v9"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

const defaultRedisDB = 0

func main() {
	logpkg.InitFromEnv()

	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		secret = "dev-only-secret-change-me-please-1234567890"
	}

	userRepo, refreshRepo, cleanup := buildAuthRepositories()
	defer cleanup()

	r := router.NewEngineWithRepositories(secret, userRepo, refreshRepo)
	if err := r.Run(":8080"); err != nil {
		log.Fatalf("server startup failed: %v", err)
	}
}

func buildAuthRepositories() (repository.UserRepository, repository.RefreshTokenRepository, func()) {
	mysqlDSN := os.Getenv("MYSQL_DSN")
	redisAddr := os.Getenv("REDIS_ADDR")
	if mysqlDSN == "" || redisAddr == "" {
		log.Println("auth repository mode: memory (MYSQL_DSN or REDIS_ADDR missing)")
		return repository.NewMemoryUserRepository(), repository.NewMemoryRefreshTokenRepository(), func() {}
	}

	db, err := gorm.Open(mysql.Open(mysqlDSN), &gorm.Config{})
	if err != nil {
		log.Printf("mysql init failed, fallback to memory: %v", err)
		return repository.NewMemoryUserRepository(), repository.NewMemoryRefreshTokenRepository(), func() {}
	}
	userRepo, err := repository.NewMySQLUserRepository(db)
	if err != nil {
		log.Printf("mysql user repository init failed, fallback to memory: %v", err)
		return repository.NewMemoryUserRepository(), repository.NewMemoryRefreshTokenRepository(), func() {}
	}

	redisDB := defaultRedisDB
	if rawDB := os.Getenv("REDIS_DB"); rawDB != "" {
		parsed, parseErr := strconv.Atoi(rawDB)
		if parseErr != nil {
			log.Printf("invalid REDIS_DB=%q, use default=%d", rawDB, defaultRedisDB)
		} else {
			redisDB = parsed
		}
	}

	redisClient := redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Password: os.Getenv("REDIS_PASSWORD"),
		DB:       redisDB,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if pingErr := redisClient.Ping(ctx).Err(); pingErr != nil {
		log.Printf("redis init failed, fallback to memory: %v", pingErr)
		_ = redisClient.Close()
		return repository.NewMemoryUserRepository(), repository.NewMemoryRefreshTokenRepository(), func() {}
	}

	refreshRepo, err := repository.NewRedisRefreshTokenRepository(redisClient, os.Getenv("REDIS_REFRESH_PREFIX"))
	if err != nil {
		log.Printf("redis refresh repository init failed, fallback to memory: %v", err)
		_ = redisClient.Close()
		return repository.NewMemoryUserRepository(), repository.NewMemoryRefreshTokenRepository(), func() {}
	}

	log.Println("auth repository mode: persistent (MySQL + Redis)")
	return userRepo, refreshRepo, func() {
		_ = redisClient.Close()
	}
}
