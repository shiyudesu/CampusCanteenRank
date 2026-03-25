package controller

import (
	"CampusCanteenRank/server/internal/controller/shared"
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
	userID, ok := shared.CurrentUserID(c)
	if !ok {
		shared.FailUnauthorized(c)
		return
	}

	limit, err := shared.QueryInt(c.Query("limit"), 0)
	if err != nil {
		shared.FailInvalidParams(c)
		return
	}

	data, serviceErr := h.service.ListMyComments(c.Request.Context(), userID, limit, c.Query("cursor"))
	if serviceErr != nil {
		shared.WriteError(c, serviceErr)
		return
	}
	response.OK(c, data)
}

func (h *MeHandler) ListMyRatings(c *gin.Context) {
	userID, ok := shared.CurrentUserID(c)
	if !ok {
		shared.FailUnauthorized(c)
		return
	}

	limit, err := shared.QueryInt(c.Query("limit"), 0)
	if err != nil {
		shared.FailInvalidParams(c)
		return
	}

	data, serviceErr := h.service.ListMyRatings(c.Request.Context(), userID, limit, c.Query("cursor"))
	if serviceErr != nil {
		shared.WriteError(c, serviceErr)
		return
	}
	response.OK(c, data)
}
