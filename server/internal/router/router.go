package router

import (
	"CampusCanteenRank/server/internal/auth/controller"
	"CampusCanteenRank/server/internal/auth/repository"
	"CampusCanteenRank/server/internal/auth/service"
	"CampusCanteenRank/server/internal/middleware"
	"github.com/gin-gonic/gin"
)

func NewEngine(secret string) *gin.Engine {
	return NewEngineWithRepositories(secret, repository.NewMemoryUserRepository(), repository.NewMemoryRefreshTokenRepository())
}

func NewEngineWithRepositories(
	secret string,
	userRepo repository.UserRepository,
	refreshRepo repository.RefreshTokenRepository,
) *gin.Engine {
	r := gin.New()
	r.Use(middleware.TraceID())
	r.Use(middleware.RequestLogger())
	r.Use(middleware.Recover())

	if userRepo == nil {
		userRepo = repository.NewMemoryUserRepository()
	}
	if refreshRepo == nil {
		refreshRepo = repository.NewMemoryRefreshTokenRepository()
	}

	authService := service.NewAuthService(userRepo, refreshRepo, secret)
	authHandler := controller.NewAuthHandler(authService)

	v1 := r.Group("/api/v1")
	authGroup := v1.Group("/auth")
	authGroup.POST("/register", authHandler.Register)
	authGroup.POST("/login", authHandler.Login)
	authGroup.POST("/refresh", authHandler.Refresh)

	return r
}
