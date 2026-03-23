package controller

import (
	"errors"
	"net/http"
	"strconv"

	errpkg "CampusCanteenRank/server/internal/pkg/errors"
	"CampusCanteenRank/server/internal/pkg/response"
	service "CampusCanteenRank/server/internal/service/ranking"
	"github.com/gin-gonic/gin"
)

type RankingHandler struct {
	service *service.RankingService
}

func NewRankingHandler(service *service.RankingService) *RankingHandler {
	return &RankingHandler{service: service}
}

func (h *RankingHandler) ListRankings(c *gin.Context) {
	limit, err := parseInt(c.Query("limit"))
	if err != nil {
		response.Fail(c, http.StatusBadRequest, errpkg.CodeBadRequest, "invalid params")
		return
	}
	scopeID, err := parseInt64(c.Query("scopeId"))
	if err != nil {
		response.Fail(c, http.StatusBadRequest, errpkg.CodeBadRequest, "invalid params")
		return
	}
	foodTypeID, err := parseInt64(c.Query("foodTypeId"))
	if err != nil {
		response.Fail(c, http.StatusBadRequest, errpkg.CodeBadRequest, "invalid params")
		return
	}
	days, err := parseInt(c.Query("days"))
	if err != nil {
		response.Fail(c, http.StatusBadRequest, errpkg.CodeBadRequest, "invalid params")
		return
	}

	data, serviceErr := h.service.ListRankings(
		c.Request.Context(),
		c.Query("scope"),
		scopeID,
		foodTypeID,
		days,
		c.Query("sort"),
		limit,
		c.Query("cursor"),
	)
	if serviceErr != nil {
		h.writeError(c, serviceErr)
		return
	}
	response.OK(c, data)
}

func (h *RankingHandler) writeError(c *gin.Context, err error) {
	var appErr *errpkg.AppError
	if errors.As(err, &appErr) {
		switch appErr.Code {
		case errpkg.CodeBadRequest:
			response.Fail(c, http.StatusBadRequest, appErr.Code, appErr.Message)
		default:
			response.Fail(c, http.StatusInternalServerError, errpkg.CodeInternal, "internal error")
		}
		return
	}
	response.Fail(c, http.StatusInternalServerError, errpkg.CodeInternal, "internal error")
}

func parseInt(raw string) (int, error) {
	if raw == "" {
		return 0, nil
	}
	return strconv.Atoi(raw)
}

func parseInt64(raw string) (int64, error) {
	if raw == "" {
		return 0, nil
	}
	return strconv.ParseInt(raw, 10, 64)
}
