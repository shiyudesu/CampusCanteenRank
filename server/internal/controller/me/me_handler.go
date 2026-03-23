package controller

import (
	"errors"
	"net/http"
	"strconv"

	errpkg "CampusCanteenRank/server/internal/pkg/errors"
	"CampusCanteenRank/server/internal/pkg/response"
	service "CampusCanteenRank/server/internal/service/me"
	"github.com/gin-gonic/gin"
)

type MeHandler struct {
	service *service.MeService
}

func NewMeHandler(service *service.MeService) *MeHandler {
	return &MeHandler{service: service}
}

func (h *MeHandler) ListMyComments(c *gin.Context) {
	userID, ok := getMeUserID(c)
	if !ok {
		response.Fail(c, http.StatusUnauthorized, errpkg.CodeUnauthorized, "unauthorized")
		return
	}

	limit := 0
	if rawLimit := c.Query("limit"); rawLimit != "" {
		parsed, parseErr := strconv.Atoi(rawLimit)
		if parseErr != nil {
			response.Fail(c, http.StatusBadRequest, errpkg.CodeBadRequest, "invalid params")
			return
		}
		limit = parsed
	}

	data, serviceErr := h.service.ListMyComments(c.Request.Context(), userID, limit, c.Query("cursor"))
	if serviceErr != nil {
		h.writeError(c, serviceErr)
		return
	}
	response.OK(c, data)
}

func (h *MeHandler) ListMyRatings(c *gin.Context) {
	userID, ok := getMeUserID(c)
	if !ok {
		response.Fail(c, http.StatusUnauthorized, errpkg.CodeUnauthorized, "unauthorized")
		return
	}

	limit := 0
	if rawLimit := c.Query("limit"); rawLimit != "" {
		parsed, parseErr := strconv.Atoi(rawLimit)
		if parseErr != nil {
			response.Fail(c, http.StatusBadRequest, errpkg.CodeBadRequest, "invalid params")
			return
		}
		limit = parsed
	}

	data, serviceErr := h.service.ListMyRatings(c.Request.Context(), userID, limit, c.Query("cursor"))
	if serviceErr != nil {
		h.writeError(c, serviceErr)
		return
	}
	response.OK(c, data)
}

func (h *MeHandler) writeError(c *gin.Context, err error) {
	var appErr *errpkg.AppError
	if errors.As(err, &appErr) {
		switch appErr.Code {
		case errpkg.CodeBadRequest:
			response.Fail(c, http.StatusBadRequest, appErr.Code, appErr.Message)
		case errpkg.CodeUnauthorized:
			response.Fail(c, http.StatusUnauthorized, appErr.Code, appErr.Message)
		case errpkg.CodeNotFound:
			response.Fail(c, http.StatusNotFound, appErr.Code, appErr.Message)
		default:
			response.Fail(c, http.StatusInternalServerError, errpkg.CodeInternal, "internal error")
		}
		return
	}
	response.Fail(c, http.StatusInternalServerError, errpkg.CodeInternal, "internal error")
}

func getMeUserID(c *gin.Context) (int64, bool) {
	rawUserID, ok := c.Get("userId")
	if !ok {
		return 0, false
	}
	userID, ok := rawUserID.(int64)
	if !ok || userID <= 0 {
		return 0, false
	}
	return userID, true
}
