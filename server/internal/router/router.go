package router

import (
	"CampusCanteenRank/server/internal/auth/controller"
	"CampusCanteenRank/server/internal/auth/repository"
	"CampusCanteenRank/server/internal/auth/service"
	"github.com/gin-gonic/gin"
)

func NewEngine(secret string) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())

	userRepo := repository.NewMemoryUserRepository()
	refreshRepo := repository.NewMemoryRefreshTokenRepository()
	authService := service.NewAuthService(userRepo, refreshRepo, secret)
	authHandler := controller.NewAuthHandler(authService)

	v1 := r.Group("/api/v1")
	authGroup := v1.Group("/auth")
	authGroup.POST("/register", authHandler.Register)
	authGroup.POST("/login", authHandler.Login)
	authGroup.POST("/refresh", authHandler.Refresh)

	return r
}
