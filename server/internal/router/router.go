package router

import (
	authcontroller "CampusCanteenRank/server/internal/controller/auth"
	commentcontroller "CampusCanteenRank/server/internal/controller/comment"
	mecontroller "CampusCanteenRank/server/internal/controller/me"
	rankingcontroller "CampusCanteenRank/server/internal/controller/ranking"
	stallcontroller "CampusCanteenRank/server/internal/controller/stall"
	"CampusCanteenRank/server/internal/middleware"
	authrepo "CampusCanteenRank/server/internal/repository/auth"
	commentrepo "CampusCanteenRank/server/internal/repository/comment"
	rankingrepo "CampusCanteenRank/server/internal/repository/ranking"
	stallrepo "CampusCanteenRank/server/internal/repository/stall"
	authservice "CampusCanteenRank/server/internal/service/auth"
	commentservice "CampusCanteenRank/server/internal/service/comment"
	meservice "CampusCanteenRank/server/internal/service/me"
	rankingservice "CampusCanteenRank/server/internal/service/ranking"
	stallservice "CampusCanteenRank/server/internal/service/stall"
	"github.com/gin-gonic/gin"
	"time"
)

func NewEngine(secret string) *gin.Engine {
	return NewEngineWithAllRepositories(
		secret,
		authrepo.NewMemoryUserRepository(),
		authrepo.NewMemoryRefreshTokenRepository(),
		stallrepo.NewMemoryStallRepository(),
		commentrepo.NewMemoryCommentRepository(),
		rankingrepo.NewMemoryRankingRepository(),
	)
}

func NewEngineWithRepositories(
	secret string,
	userRepo authrepo.UserRepository,
	refreshRepo authrepo.RefreshTokenRepository,
) *gin.Engine {
	return NewEngineWithAllRepositories(
		secret,
		userRepo,
		refreshRepo,
		stallrepo.NewMemoryStallRepository(),
		commentrepo.NewMemoryCommentRepository(),
		rankingrepo.NewMemoryRankingRepository(),
	)
}

func NewEngineWithAllRepositories(
	secret string,
	userRepo authrepo.UserRepository,
	refreshRepo authrepo.RefreshTokenRepository,
	stallRepository stallrepo.StallRepository,
	commentRepository commentrepo.CommentRepository,
	rankingRepository rankingrepo.RankingRepository,
) *gin.Engine {
	r := gin.New()
	r.Use(middleware.TraceID())
	r.Use(middleware.RequestLogger())
	r.Use(middleware.Recover())

	if userRepo == nil {
		userRepo = authrepo.NewMemoryUserRepository()
	}
	if refreshRepo == nil {
		refreshRepo = authrepo.NewMemoryRefreshTokenRepository()
	}
	if stallRepository == nil {
		stallRepository = stallrepo.NewMemoryStallRepository()
	}
	if commentRepository == nil {
		commentRepository = commentrepo.NewMemoryCommentRepository()
	}
	if rankingRepository == nil {
		rankingRepository = rankingrepo.NewMemoryRankingRepository()
	}

	authService := authservice.NewAuthService(userRepo, refreshRepo, secret)
	authHandler := authcontroller.NewAuthHandler(authService)
	stallHandler := stallcontroller.NewStallHandler(stallservice.NewStallService(stallRepository))
	commentHandler := commentcontroller.NewCommentHandler(commentservice.NewCommentService(commentRepository, stallRepository, userRepo))
	meHandler := mecontroller.NewMeHandler(meservice.NewMeService(commentRepository, stallRepository, userRepo))
	rankingHandler := rankingcontroller.NewRankingHandler(rankingservice.NewRankingService(rankingRepository))

	v1 := r.Group("/api/v1")
	authGroup := v1.Group("/auth")
	authGroup.POST("/register", authHandler.Register)
	authGroup.POST("/login", middleware.RateLimitByClient("auth_login", 20, time.Minute), authHandler.Login)
	authGroup.POST("/refresh", authHandler.Refresh)
	authGroup.POST("/logout", authHandler.Logout)
	v1.GET("/canteens", stallHandler.ListCanteens)
	v1.GET("/stalls", stallHandler.ListStalls)
	v1.GET("/stalls/:stallId", middleware.OptionalAuth(secret), stallHandler.GetStallDetail)
	v1.POST("/stalls/:stallId/ratings", middleware.RateLimitByClient("stall_rating_upsert", 60, time.Minute), middleware.Auth(secret), stallHandler.UpsertUserRating)
	v1.POST("/stalls/:stallId/comments", middleware.RateLimitByClient("comment_create", 60, time.Minute), middleware.Auth(secret), commentHandler.CreateComment)
	v1.GET("/stalls/:stallId/comments", middleware.OptionalAuth(secret), commentHandler.ListTopLevelComments)
	v1.GET("/comments/:rootCommentId/replies", middleware.OptionalAuth(secret), commentHandler.ListReplies)
	v1.POST("/comments/:commentId/like", middleware.RateLimitByClient("comment_like", 120, time.Minute), middleware.Auth(secret), commentHandler.LikeComment)
	v1.DELETE("/comments/:commentId/like", middleware.RateLimitByClient("comment_unlike", 120, time.Minute), middleware.Auth(secret), commentHandler.UnlikeComment)
	v1.GET("/rankings", rankingHandler.ListRankings)
	v1.GET("/me/comments", middleware.Auth(secret), meHandler.ListMyComments)
	v1.GET("/me/ratings", middleware.Auth(secret), meHandler.ListMyRatings)

	return r
}
