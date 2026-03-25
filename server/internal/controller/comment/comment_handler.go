package controller

import (
	"CampusCanteenRank/server/internal/controller/shared"
	dto "CampusCanteenRank/server/internal/dto/comment"
	"CampusCanteenRank/server/internal/pkg/response"
	service "CampusCanteenRank/server/internal/service/comment"
	"github.com/gin-gonic/gin"
)

type CommentHandler struct {
	service *service.CommentService
}

func NewCommentHandler(service *service.CommentService) *CommentHandler {
	return &CommentHandler{service: service}
}

func (h *CommentHandler) CreateComment(c *gin.Context) {
	stallID, err := shared.ParamInt64(c, "stallId")
	if err != nil {
		shared.FailInvalidParams(c)
		return
	}
	userID, ok := shared.CurrentUserID(c)
	if !ok {
		shared.FailUnauthorized(c)
		return
	}

	var req dto.CreateCommentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		shared.FailInvalidParams(c)
		return
	}

	data, serviceErr := h.service.CreateComment(c.Request.Context(), userID, stallID, req)
	if serviceErr != nil {
		shared.WriteError(c, serviceErr)
		return
	}
	response.OK(c, data)
}

func (h *CommentHandler) ListTopLevelComments(c *gin.Context) {
	stallID, err := shared.ParamInt64(c, "stallId")
	if err != nil {
		shared.FailInvalidParams(c)
		return
	}

	limit, err := shared.QueryInt(c.Query("limit"), 0)
	if err != nil {
		shared.FailInvalidParams(c)
		return
	}

	viewerUserID := int64(0)
	if userID, ok := shared.CurrentUserID(c); ok {
		viewerUserID = userID
	}

	data, serviceErr := h.service.ListTopLevelComments(c.Request.Context(), viewerUserID, stallID, limit, c.Query("cursor"), c.Query("sort"))
	if serviceErr != nil {
		shared.WriteError(c, serviceErr)
		return
	}
	response.OK(c, data)
}

func (h *CommentHandler) ListReplies(c *gin.Context) {
	rootCommentID, err := shared.ParamInt64(c, "rootCommentId")
	if err != nil {
		shared.FailInvalidParams(c)
		return
	}

	limit, err := shared.QueryInt(c.Query("limit"), 0)
	if err != nil {
		shared.FailInvalidParams(c)
		return
	}

	viewerUserID := int64(0)
	if userID, ok := shared.CurrentUserID(c); ok {
		viewerUserID = userID
	}

	data, serviceErr := h.service.ListReplies(c.Request.Context(), viewerUserID, rootCommentID, limit, c.Query("cursor"))
	if serviceErr != nil {
		shared.WriteError(c, serviceErr)
		return
	}
	response.OK(c, data)
}

func (h *CommentHandler) LikeComment(c *gin.Context) {
	commentID, err := shared.ParamInt64(c, "commentId")
	if err != nil {
		shared.FailInvalidParams(c)
		return
	}
	userID, ok := shared.CurrentUserID(c)
	if !ok {
		shared.FailUnauthorized(c)
		return
	}

	data, serviceErr := h.service.LikeComment(c.Request.Context(), userID, commentID)
	if serviceErr != nil {
		shared.WriteError(c, serviceErr)
		return
	}
	response.OK(c, data)
}

func (h *CommentHandler) UnlikeComment(c *gin.Context) {
	commentID, err := shared.ParamInt64(c, "commentId")
	if err != nil {
		shared.FailInvalidParams(c)
		return
	}
	userID, ok := shared.CurrentUserID(c)
	if !ok {
		shared.FailUnauthorized(c)
		return
	}

	data, serviceErr := h.service.UnlikeComment(c.Request.Context(), userID, commentID)
	if serviceErr != nil {
		shared.WriteError(c, serviceErr)
		return
	}
	response.OK(c, data)
}
