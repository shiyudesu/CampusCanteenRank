package controller

import (
	"errors"
	"net/http"
	"strconv"

	dto "CampusCanteenRank/server/internal/dto/comment"
	errpkg "CampusCanteenRank/server/internal/pkg/errors"
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
	stallID, err := strconv.ParseInt(c.Param("stallId"), 10, 64)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, errpkg.CodeBadRequest, "invalid params")
		return
	}
	rawUserID, ok := c.Get("userId")
	if !ok {
		response.Fail(c, http.StatusUnauthorized, errpkg.CodeUnauthorized, "unauthorized")
		return
	}
	userID, ok := rawUserID.(int64)
	if !ok || userID <= 0 {
		response.Fail(c, http.StatusUnauthorized, errpkg.CodeUnauthorized, "unauthorized")
		return
	}

	var req dto.CreateCommentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, errpkg.CodeBadRequest, "invalid params")
		return
	}

	data, serviceErr := h.service.CreateComment(c.Request.Context(), userID, stallID, req)
	if serviceErr != nil {
		h.writeError(c, serviceErr)
		return
	}
	response.OK(c, data)
}

func (h *CommentHandler) ListTopLevelComments(c *gin.Context) {
	stallID, err := strconv.ParseInt(c.Param("stallId"), 10, 64)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, errpkg.CodeBadRequest, "invalid params")
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

	viewerUserID := int64(0)
	if userID, ok := getUserID(c); ok {
		viewerUserID = userID
	}

	data, serviceErr := h.service.ListTopLevelComments(c.Request.Context(), viewerUserID, stallID, limit, c.Query("cursor"), c.Query("sort"))
	if serviceErr != nil {
		h.writeError(c, serviceErr)
		return
	}
	response.OK(c, data)
}

func (h *CommentHandler) ListReplies(c *gin.Context) {
	rootCommentID, err := strconv.ParseInt(c.Param("rootCommentId"), 10, 64)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, errpkg.CodeBadRequest, "invalid params")
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

	viewerUserID := int64(0)
	if userID, ok := getUserID(c); ok {
		viewerUserID = userID
	}

	data, serviceErr := h.service.ListReplies(c.Request.Context(), viewerUserID, rootCommentID, limit, c.Query("cursor"))
	if serviceErr != nil {
		h.writeError(c, serviceErr)
		return
	}
	response.OK(c, data)
}

func (h *CommentHandler) LikeComment(c *gin.Context) {
	commentID, err := strconv.ParseInt(c.Param("commentId"), 10, 64)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, errpkg.CodeBadRequest, "invalid params")
		return
	}
	userID, ok := getUserID(c)
	if !ok {
		response.Fail(c, http.StatusUnauthorized, errpkg.CodeUnauthorized, "unauthorized")
		return
	}

	data, serviceErr := h.service.LikeComment(c.Request.Context(), userID, commentID)
	if serviceErr != nil {
		h.writeError(c, serviceErr)
		return
	}
	response.OK(c, data)
}

func (h *CommentHandler) UnlikeComment(c *gin.Context) {
	commentID, err := strconv.ParseInt(c.Param("commentId"), 10, 64)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, errpkg.CodeBadRequest, "invalid params")
		return
	}
	userID, ok := getUserID(c)
	if !ok {
		response.Fail(c, http.StatusUnauthorized, errpkg.CodeUnauthorized, "unauthorized")
		return
	}

	data, serviceErr := h.service.UnlikeComment(c.Request.Context(), userID, commentID)
	if serviceErr != nil {
		h.writeError(c, serviceErr)
		return
	}
	response.OK(c, data)
}

func getUserID(c *gin.Context) (int64, bool) {
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

func (h *CommentHandler) writeError(c *gin.Context, err error) {
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
