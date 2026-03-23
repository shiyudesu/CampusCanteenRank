package router

import (
	authcontroller "CampusCanteenRank/server/internal/controller/auth"
	commentcontroller "CampusCanteenRank/server/internal/controller/comment"
	stallcontroller "CampusCanteenRank/server/internal/controller/stall"
	"CampusCanteenRank/server/internal/middleware"
	authrepo "CampusCanteenRank/server/internal/repository/auth"
	commentrepo "CampusCanteenRank/server/internal/repository/comment"
	stallrepo "CampusCanteenRank/server/internal/repository/stall"
	authservice "CampusCanteenRank/server/internal/service/auth"
	commentservice "CampusCanteenRank/server/internal/service/comment"
	stallservice "CampusCanteenRank/server/internal/service/stall"
	"github.com/gin-gonic/gin"
)

func NewEngine(secret string) *gin.Engine {
	return NewEngineWithRepositories(secret, authrepo.NewMemoryUserRepository(), authrepo.NewMemoryRefreshTokenRepository())
}

func NewEngineWithRepositories(
	secret string,
	userRepo authrepo.UserRepository,
	refreshRepo authrepo.RefreshTokenRepository,
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

	authService := authservice.NewAuthService(userRepo, refreshRepo, secret)
	authHandler := authcontroller.NewAuthHandler(authService)
	stallRepository := stallrepo.NewMemoryStallRepository()
	commentRepository := commentrepo.NewMemoryCommentRepository()
	stallHandler := stallcontroller.NewStallHandler(stallservice.NewStallService(stallRepository))
	commentHandler := commentcontroller.NewCommentHandler(commentservice.NewCommentService(commentRepository, stallRepository, userRepo))

	v1 := r.Group("/api/v1")
	authGroup := v1.Group("/auth")
	authGroup.POST("/register", authHandler.Register)
	authGroup.POST("/login", authHandler.Login)
	authGroup.POST("/refresh", authHandler.Refresh)
	v1.GET("/canteens", stallHandler.ListCanteens)
	v1.GET("/stalls", stallHandler.ListStalls)
	v1.GET("/stalls/:stallId", middleware.OptionalAuth(secret), stallHandler.GetStallDetail)
	v1.POST("/stalls/:stallId/ratings", middleware.Auth(secret), stallHandler.UpsertUserRating)
	v1.POST("/stalls/:stallId/comments", middleware.Auth(secret), commentHandler.CreateComment)
	v1.GET("/stalls/:stallId/comments", middleware.OptionalAuth(secret), commentHandler.ListTopLevelComments)
	v1.GET("/comments/:rootCommentId/replies", middleware.OptionalAuth(secret), commentHandler.ListReplies)
	v1.POST("/comments/:commentId/like", middleware.Auth(secret), commentHandler.LikeComment)
	v1.DELETE("/comments/:commentId/like", middleware.Auth(secret), commentHandler.UnlikeComment)

	return r
}
