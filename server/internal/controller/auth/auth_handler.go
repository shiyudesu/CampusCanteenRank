package controller

import (
	"errors"
	"net/http"

	dto "CampusCanteenRank/server/internal/dto/auth"
	errpkg "CampusCanteenRank/server/internal/pkg/errors"
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
		response.Fail(c, http.StatusBadRequest, errpkg.CodeBadRequest, "invalid params")
		return
	}
	userID, err := h.service.Register(c.Request.Context(), req)
	if err != nil {
		h.writeError(c, err)
		return
	}
	response.OK(c, dto.RegisterData{UserID: userID})
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req dto.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, errpkg.CodeBadRequest, "invalid params")
		return
	}
	data, err := h.service.Login(c.Request.Context(), req)
	if err != nil {
		h.writeError(c, err)
		return
	}
	response.OK(c, data)
}

func (h *AuthHandler) Refresh(c *gin.Context) {
	var req dto.RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, errpkg.CodeBadRequest, "invalid params")
		return
	}
	data, err := h.service.Refresh(c.Request.Context(), req)
	if err != nil {
		h.writeError(c, err)
		return
	}
	response.OK(c, data)
}

func (h *AuthHandler) writeError(c *gin.Context, err error) {
	var appErr *errpkg.AppError
	if errors.As(err, &appErr) {
		switch appErr.Code {
		case errpkg.CodeBadRequest:
			response.Fail(c, http.StatusBadRequest, appErr.Code, appErr.Message)
		case errpkg.CodeUnauthorized:
			response.Fail(c, http.StatusUnauthorized, appErr.Code, appErr.Message)
		case errpkg.CodeConflict:
			response.Fail(c, http.StatusConflict, appErr.Code, appErr.Message)
		default:
			response.Fail(c, http.StatusInternalServerError, errpkg.CodeInternal, "internal error")
		}
		return
	}
	response.Fail(c, http.StatusInternalServerError, errpkg.CodeInternal, "internal error")
}
