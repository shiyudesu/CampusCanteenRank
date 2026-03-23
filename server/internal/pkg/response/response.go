package response

import (
	"net/http"

	errpkg "CampusCanteenRank/server/internal/pkg/errors"
	"github.com/gin-gonic/gin"
)

type Envelope struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

func OK(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Envelope{Code: errpkg.CodeOK, Message: "ok", Data: data})
}

func Fail(c *gin.Context, httpStatus int, code int, message string) {
	c.JSON(httpStatus, Envelope{Code: code, Message: message, Data: struct{}{}})
}
