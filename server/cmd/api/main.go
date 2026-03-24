package main

import (
	"context"
	"log"
	"os"
	"strconv"
	"time"

	"CampusCanteenRank/server/internal/migration"
	logpkg "CampusCanteenRank/server/internal/pkg/logger"
	authrepo "CampusCanteenRank/server/internal/repository/auth"
	commentrepo "CampusCanteenRank/server/internal/repository/comment"
	rankingrepo "CampusCanteenRank/server/internal/repository/ranking"
	stallrepo "CampusCanteenRank/server/internal/repository/stall"
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

	userRepo, refreshRepo, stallRepository, commentRepository, rankingRepository, cleanup := buildRepositories()
	defer cleanup()

	r := router.NewEngineWithAllRepositories(secret, userRepo, refreshRepo, stallRepository, commentRepository, rankingRepository)
	if err := r.Run(":8080"); err != nil {
		log.Fatalf("server startup failed: %v", err)
	}
}

func buildRepositories() (
	authrepo.UserRepository,
	authrepo.RefreshTokenRepository,
	stallrepo.StallRepository,
	commentrepo.CommentRepository,
	rankingrepo.RankingRepository,
	func(),
) {
	mysqlDSN := os.Getenv("MYSQL_DSN")
	redisAddr := os.Getenv("REDIS_ADDR")
	if mysqlDSN == "" || redisAddr == "" {
		log.Println("repository mode: memory (MYSQL_DSN or REDIS_ADDR missing)")
		return memoryRepositories()
	}

	db, err := gorm.Open(mysql.Open(mysqlDSN), &gorm.Config{})
	if err != nil {
		log.Printf("mysql init failed, fallback to memory: %v", err)
		return memoryRepositories()
	}
	if err := migration.ApplySQLMigrations(db); err != nil {
		log.Printf("sql migrations apply failed, fallback to memory: %v", err)
		return memoryRepositories()
	}
	userRepo, err := authrepo.NewMySQLUserRepository(db)
	if err != nil {
		log.Printf("mysql user repository init failed, fallback to memory: %v", err)
		return memoryRepositories()
	}

	stallRepository, err := stallrepo.NewMySQLStallRepository(db)
	if err != nil {
		log.Printf("mysql stall repository init failed, fallback to memory: %v", err)
		return memoryRepositories()
	}

	commentRepository, err := commentrepo.NewMySQLCommentRepository(db)
	if err != nil {
		log.Printf("mysql comment repository init failed, fallback to memory: %v", err)
		return memoryRepositories()
	}

	mysqlRankingRepository, err := rankingrepo.NewMySQLRankingRepository(db)
	if err != nil {
		log.Printf("mysql ranking repository init failed, fallback to memory: %v", err)
		return memoryRepositories()
	}
	var rankingRepository rankingrepo.RankingRepository = mysqlRankingRepository

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
		return memoryRepositories()
	}

	refreshRepo, err := authrepo.NewRedisRefreshTokenRepository(redisClient, os.Getenv("REDIS_REFRESH_PREFIX"))
	if err != nil {
		log.Printf("redis refresh repository init failed, fallback to memory: %v", err)
		_ = redisClient.Close()
		return memoryRepositories()
	}

	rankingRepository = rankingrepo.NewCachedRankingRepository(
		rankingRepository,
		redisClient,
		os.Getenv("REDIS_RANKING_PREFIX"),
		30*time.Second,
	)

	log.Println("repository mode: persistent (MySQL + Redis)")
	return userRepo, refreshRepo, stallRepository, commentRepository, rankingRepository, func() {
		_ = redisClient.Close()
	}
}

func memoryRepositories() (
	authrepo.UserRepository,
	authrepo.RefreshTokenRepository,
	stallrepo.StallRepository,
	commentrepo.CommentRepository,
	rankingrepo.RankingRepository,
	func(),
) {
	return authrepo.NewMemoryUserRepository(),
		authrepo.NewMemoryRefreshTokenRepository(),
		stallrepo.NewMemoryStallRepository(),
		commentrepo.NewMemoryCommentRepository(),
		rankingrepo.NewMemoryRankingRepository(),
		func() {}
}
