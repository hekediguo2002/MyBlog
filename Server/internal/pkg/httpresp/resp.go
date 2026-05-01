package httpresp

import (
	"errors"

	"github.com/gin-gonic/gin"

	"github.com/wjr/blog/server/internal/apperr"
)

type Envelope struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data any    `json:"data"`
}

func OK(c *gin.Context, data any) {
	c.JSON(200, Envelope{Code: apperr.CodeOK, Msg: "ok", Data: data})
}

func Fail(c *gin.Context, err error) {
	var ae *apperr.AppErr
	if !errors.As(err, &ae) {
		ae = apperr.New(apperr.CodeUnknown, "internal error")
	}
	c.AbortWithStatusJSON(ae.HTTP, Envelope{Code: ae.Code, Msg: ae.Msg, Data: nil})
}
