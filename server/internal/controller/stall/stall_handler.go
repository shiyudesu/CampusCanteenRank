package controller

import (
	"CampusCanteenRank/server/internal/controller/shared"
	dto "CampusCanteenRank/server/internal/dto/stall"
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
		shared.WriteError(c, err)
		return
	}
	response.OK(c, data)
}

func (h *StallHandler) ListStalls(c *gin.Context) {
	limit, err := shared.QueryInt(c.Query("limit"), 0)
	if err != nil {
		shared.FailInvalidParams(c)
		return
	}
	canteenID, err := shared.QueryInt64(c.Query("canteenId"), 0)
	if err != nil {
		shared.FailInvalidParams(c)
		return
	}
	foodTypeID, err := shared.QueryInt64(c.Query("foodTypeId"), 0)
	if err != nil {
		shared.FailInvalidParams(c)
		return
	}
	data, err := h.service.ListStalls(c.Request.Context(), limit, c.Query("cursor"), canteenID, foodTypeID, c.Query("sort"))
	if err != nil {
		shared.WriteError(c, err)
		return
	}
	response.OK(c, data)
}

func (h *StallHandler) GetStallDetail(c *gin.Context) {
	stallID, err := shared.ParamInt64(c, "stallId")
	if err != nil {
		shared.FailInvalidParams(c)
		return
	}
	var userID *int64
	if parsed, ok := shared.CurrentUserID(c); ok {
		userID = &parsed
	}
	data, serviceErr := h.service.GetStallDetail(c.Request.Context(), stallID, userID)
	if serviceErr != nil {
		shared.WriteError(c, serviceErr)
		return
	}
	response.OK(c, data)
}

func (h *StallHandler) UpsertUserRating(c *gin.Context) {
	stallID, err := shared.ParamInt64(c, "stallId")
	if err != nil {
		shared.FailInvalidParams(c)
		return
	}

	var req dto.UpsertRatingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		shared.FailInvalidParams(c)
		return
	}

	userID, ok := shared.CurrentUserID(c)
	if !ok {
		shared.FailUnauthorized(c)
		return
	}

	data, serviceErr := h.service.UpsertUserRating(c.Request.Context(), userID, stallID, req.Score)
	if serviceErr != nil {
		shared.WriteError(c, serviceErr)
		return
	}
	response.OK(c, data)
}
