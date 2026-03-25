package controller

import (
	"CampusCanteenRank/server/internal/controller/shared"
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
	limit, err := shared.QueryInt(c.Query("limit"), 0)
	if err != nil {
		shared.FailInvalidParams(c)
		return
	}
	scopeID, err := shared.QueryInt64(c.Query("scopeId"), 0)
	if err != nil {
		shared.FailInvalidParams(c)
		return
	}
	foodTypeID, err := shared.QueryInt64(c.Query("foodTypeId"), 0)
	if err != nil {
		shared.FailInvalidParams(c)
		return
	}
	days, err := shared.QueryInt(c.Query("days"), 0)
	if err != nil {
		shared.FailInvalidParams(c)
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
		shared.WriteError(c, serviceErr)
		return
	}
	response.OK(c, data)
}
