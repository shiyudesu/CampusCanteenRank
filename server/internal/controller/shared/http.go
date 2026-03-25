package shared

import (
	"errors"
	"net/http"
	"strconv"

	errpkg "CampusCanteenRank/server/internal/pkg/errors"
	"CampusCanteenRank/server/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

func FailInvalidParams(c *gin.Context) {
	response.Fail(c, http.StatusBadRequest, errpkg.CodeBadRequest, "invalid params")
}

func FailUnauthorized(c *gin.Context) {
	response.Fail(c, http.StatusUnauthorized, errpkg.CodeUnauthorized, "unauthorized")
}

func WriteError(c *gin.Context, err error) {
	var appErr *errpkg.AppError
	if !errors.As(err, &appErr) {
		response.Fail(c, http.StatusInternalServerError, errpkg.CodeInternal, "internal error")
		return
	}

	switch appErr.Code {
	case errpkg.CodeBadRequest:
		response.Fail(c, http.StatusBadRequest, appErr.Code, appErr.Message)
	case errpkg.CodeUnauthorized:
		response.Fail(c, http.StatusUnauthorized, appErr.Code, appErr.Message)
	case errpkg.CodeForbidden:
		response.Fail(c, http.StatusForbidden, appErr.Code, appErr.Message)
	case errpkg.CodeNotFound:
		response.Fail(c, http.StatusNotFound, appErr.Code, appErr.Message)
	case errpkg.CodeConflict:
		response.Fail(c, http.StatusConflict, appErr.Code, appErr.Message)
	case errpkg.CodeTooMany:
		response.Fail(c, http.StatusTooManyRequests, appErr.Code, appErr.Message)
	default:
		response.Fail(c, http.StatusInternalServerError, errpkg.CodeInternal, "internal error")
	}
}

func CurrentUserID(c *gin.Context) (int64, bool) {
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

func QueryInt(raw string, def int) (int, error) {
	if raw == "" {
		return def, nil
	}
	return strconv.Atoi(raw)
}

func QueryInt64(raw string, def int64) (int64, error) {
	if raw == "" {
		return def, nil
	}
	return strconv.ParseInt(raw, 10, 64)
}

func ParamInt64(c *gin.Context, name string) (int64, error) {
	return strconv.ParseInt(c.Param(name), 10, 64)
}
