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

func RequireAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		sess, ok := SessionFromContext(c)
		if !ok {
			httpresp.Fail(c, apperr.New(apperr.CodeUnauthorized, "未登录"))
			c.Abort()
			return
		}
		if !sess.IsAdmin {
			httpresp.Fail(c, apperr.New(apperr.CodeForbidden, "无权访问"))
			c.Abort()
			return
		}
		c.Next()
	}
}
