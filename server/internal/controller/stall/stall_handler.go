package controller

import (
	"errors"
	"net/http"
	"strconv"

	errpkg "CampusCanteenRank/server/internal/pkg/errors"
	"CampusCanteenRank/server/internal/pkg/response"
	"CampusCanteenRank/server/internal/service/stall"
	"github.com/gin-gonic/gin"
)

type StallHandler struct {
	service *service.StallService
}

func NewStallHandler(service *service.StallService) *StallHandler {
	return &StallHandler{service: service}
}

func (h *StallHandler) ListCanteens(c *gin.Context) {
	data, err := h.service.ListCanteens(c.Request.Context())
	if err != nil {
		h.writeError(c, err)
		return
	}
	response.OK(c, data)
}

func (h *StallHandler) ListStalls(c *gin.Context) {
	limit, err := parseIntQuery(c.Query("limit"), 0)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, errpkg.CodeBadRequest, "invalid params")
		return
	}
	canteenID, err := parseInt64Query(c.Query("canteenId"), 0)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, errpkg.CodeBadRequest, "invalid params")
		return
	}
	foodTypeID, err := parseInt64Query(c.Query("foodTypeId"), 0)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, errpkg.CodeBadRequest, "invalid params")
		return
	}
	data, err := h.service.ListStalls(c.Request.Context(), limit, c.Query("cursor"), canteenID, foodTypeID, c.Query("sort"))
	if err != nil {
		h.writeError(c, err)
		return
	}
	response.OK(c, data)
}

func (h *StallHandler) GetStallDetail(c *gin.Context) {
	stallID, err := strconv.ParseInt(c.Param("stallId"), 10, 64)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, errpkg.CodeBadRequest, "invalid params")
		return
	}
	var userID *int64
	if rawUserID, ok := c.Get("userId"); ok {
		if parsed, ok := rawUserID.(int64); ok {
			userID = &parsed
		}
	}
	data, serviceErr := h.service.GetStallDetail(c.Request.Context(), stallID, userID)
	if serviceErr != nil {
		h.writeError(c, serviceErr)
		return
	}
	response.OK(c, data)
}

func parseIntQuery(raw string, def int) (int, error) {
	if raw == "" {
		return def, nil
	}
	parsed, err := strconv.Atoi(raw)
	if err != nil {
		return 0, err
	}
	return parsed, nil
}

func parseInt64Query(raw string, def int64) (int64, error) {
	if raw == "" {
		return def, nil
	}
	parsed, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, err
	}
	return parsed, nil
}

func (h *StallHandler) writeError(c *gin.Context, err error) {
	var appErr *errpkg.AppError
	if errors.As(err, &appErr) {
		switch appErr.Code {
		case errpkg.CodeBadRequest:
			response.Fail(c, http.StatusBadRequest, appErr.Code, appErr.Message)
		case errpkg.CodeNotFound:
			response.Fail(c, http.StatusNotFound, appErr.Code, appErr.Message)
		default:
			response.Fail(c, http.StatusInternalServerError, errpkg.CodeInternal, "internal error")
		}
		return
	}
	response.Fail(c, http.StatusInternalServerError, errpkg.CodeInternal, "internal error")
}
