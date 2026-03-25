package main

import (
	"context"
	"errors"
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

	userRepo, refreshRepo, stallRepository, commentRepository, rankingRepository, cleanup, err := buildRepositories(cfg)
	if err != nil {
		log.Fatalf("build repositories failed: %v", err)
	}
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
	error,
) {
	mysqlDSN := cfg.MySQLDSN
	redisAddr := cfg.RedisAddr
	if mysqlDSN == "" || redisAddr == "" {
		return nil, nil, nil, nil, nil, func() {}, errors.New("MYSQL_DSN and REDIS_ADDR are required")
	}

	db, err := gorm.Open(mysql.Open(mysqlDSN), &gorm.Config{})
	if err != nil {
		return nil, nil, nil, nil, nil, func() {}, err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, nil, nil, nil, nil, func() {}, err
	}
	if err := migration.ApplySQLMigrations(db); err != nil {
		_ = sqlDB.Close()
		return nil, nil, nil, nil, nil, func() {}, err
	}
	userRepo, err := authrepo.NewMySQLUserRepository(db)
	if err != nil {
		_ = sqlDB.Close()
		return nil, nil, nil, nil, nil, func() {}, err
	}

	stallRepository, err := stallrepo.NewMySQLStallRepository(db)
	if err != nil {
		_ = sqlDB.Close()
		return nil, nil, nil, nil, nil, func() {}, err
	}

	commentRepository, err := commentrepo.NewMySQLCommentRepository(db)
	if err != nil {
		_ = sqlDB.Close()
		return nil, nil, nil, nil, nil, func() {}, err
	}

	mysqlRankingRepository, err := rankingrepo.NewMySQLRankingRepository(db)
	if err != nil {
		_ = sqlDB.Close()
		return nil, nil, nil, nil, nil, func() {}, err
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
		_ = redisClient.Close()
		_ = sqlDB.Close()
		return nil, nil, nil, nil, nil, func() {}, pingErr
	}

	refreshRepo, err := authrepo.NewRedisRefreshTokenRepository(redisClient, cfg.RedisRefreshPrefix)
	if err != nil {
		_ = redisClient.Close()
		_ = sqlDB.Close()
		return nil, nil, nil, nil, nil, func() {}, err
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
		_ = sqlDB.Close()
	}, nil
}
