package controller

import (
	"CampusCanteenRank/server/internal/controller/shared"
	dto "CampusCanteenRank/server/internal/dto/auth"
	"CampusCanteenRank/server/internal/pkg/response"
	"CampusCanteenRank/server/internal/service/auth"
	"github.com/gin-gonic/gin"
)

type AuthHandler struct {
	service *service.AuthService
}

func NewAuthHandler(s *service.AuthService) *AuthHandler {
	return &AuthHandler{service: s}
}

func (h *AuthHandler) Register(c *gin.Context) {
	var req dto.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		shared.FailInvalidParams(c)
		return
	}
	userID, err := h.service.Register(c.Request.Context(), req)
	if err != nil {
		shared.WriteError(c, err)
		return
	}
	response.OK(c, dto.RegisterData{UserID: userID})
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req dto.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		shared.FailInvalidParams(c)
		return
	}
	data, err := h.service.Login(c.Request.Context(), req)
	if err != nil {
		shared.WriteError(c, err)
		return
	}
	response.OK(c, data)
}

func (h *AuthHandler) Refresh(c *gin.Context) {
	var req dto.RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		shared.FailInvalidParams(c)
		return
	}
	data, err := h.service.Refresh(c.Request.Context(), req)
	if err != nil {
		shared.WriteError(c, err)
		return
	}
	response.OK(c, data)
}

func (h *AuthHandler) Logout(c *gin.Context) {
	var req dto.RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		shared.FailInvalidParams(c)
		return
	}
	if err := h.service.Logout(c.Request.Context(), req); err != nil {
		shared.WriteError(c, err)
		return
	}
	response.OK(c, struct{}{})
}
