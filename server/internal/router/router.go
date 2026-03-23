package router

import (
	authcontroller "CampusCanteenRank/server/internal/controller/auth"
	stallcontroller "CampusCanteenRank/server/internal/controller/stall"
	"CampusCanteenRank/server/internal/middleware"
	authrepo "CampusCanteenRank/server/internal/repository/auth"
	stallrepo "CampusCanteenRank/server/internal/repository/stall"
	authservice "CampusCanteenRank/server/internal/service/auth"
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
	stallHandler := stallcontroller.NewStallHandler(stallservice.NewStallService(stallRepository))

	v1 := r.Group("/api/v1")
	authGroup := v1.Group("/auth")
	authGroup.POST("/register", authHandler.Register)
	authGroup.POST("/login", authHandler.Login)
	authGroup.POST("/refresh", authHandler.Refresh)
	v1.GET("/canteens", stallHandler.ListCanteens)
	v1.GET("/stalls", stallHandler.ListStalls)
	v1.GET("/stalls/:stallId", middleware.OptionalAuth(secret), stallHandler.GetStallDetail)
	v1.POST("/stalls/:stallId/ratings", middleware.Auth(secret), stallHandler.UpsertUserRating)

	return r
}
