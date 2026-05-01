package middleware

import (
	"github.com/gin-gonic/gin"

	"github.com/wjr/blog/server/internal/apperr"
	"github.com/wjr/blog/server/internal/pkg/httpresp"
)

func RequireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		if _, ok := SessionFromContext(c); !ok {
			httpresp.Fail(c, apperr.New(apperr.CodeUnauthorized, "未登录"))
			return
		}
		c.Next()
	}
}
