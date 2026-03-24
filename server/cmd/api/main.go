package main

import (
	"context"
	"log"
	"time"

	"CampusCanteenRank/server/internal/config"
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

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load runtime config failed: %v", err)
	}
	if err := logpkg.Init(cfg.LogLevel); err != nil {
		log.Fatalf("init logger failed: %v", err)
	}
	logpkg.SetSensitiveFields(cfg.LogSensitiveFields)

	userRepo, refreshRepo, stallRepository, commentRepository, rankingRepository, cleanup := buildRepositories(cfg)
	defer cleanup()

	r := router.NewEngineWithAllRepositories(cfg.JWTSecret, userRepo, refreshRepo, stallRepository, commentRepository, rankingRepository)
	if err := r.Run(":" + cfg.ServerPort); err != nil {
		log.Fatalf("server startup failed: %v", err)
	}
}

func buildRepositories(cfg config.RuntimeConfig) (
	authrepo.UserRepository,
	authrepo.RefreshTokenRepository,
	stallrepo.StallRepository,
	commentrepo.CommentRepository,
	rankingrepo.RankingRepository,
	func(),
) {
	mysqlDSN := cfg.MySQLDSN
	redisAddr := cfg.RedisAddr
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

	redisClient := redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if pingErr := redisClient.Ping(ctx).Err(); pingErr != nil {
		log.Printf("redis init failed, fallback to memory: %v", pingErr)
		_ = redisClient.Close()
		return memoryRepositories()
	}

	refreshRepo, err := authrepo.NewRedisRefreshTokenRepository(redisClient, cfg.RedisRefreshPrefix)
	if err != nil {
		log.Printf("redis refresh repository init failed, fallback to memory: %v", err)
		_ = redisClient.Close()
		return memoryRepositories()
	}

	rankingRepository = rankingrepo.NewCachedRankingRepository(
		rankingRepository,
		redisClient,
		cfg.RedisRankingPrefix,
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
