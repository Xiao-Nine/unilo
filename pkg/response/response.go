package response

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"unilo/pkg/apperror"
)

type Envelope struct {
	Code int         `json:"code"`
	Msg  string      `json:"msg"`
	Data interface{} `json:"data"`
}

func Success(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Envelope{Code: 200, Msg: "success", Data: data})
}

func Error(c *gin.Context, err error) {
	appErr := apperror.From(err)
	c.JSON(apperror.HTTPStatus(appErr.Code), Envelope{Code: appErr.Code, Msg: appErr.Message, Data: gin.H{}})
}
