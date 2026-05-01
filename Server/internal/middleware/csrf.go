package middleware

import (
	"github.com/gin-gonic/gin"

	"github.com/wjr/blog/server/internal/apperr"
	"github.com/wjr/blog/server/internal/pkg/httpresp"
)

func CSRFGuard() gin.HandlerFunc {
	return func(c *gin.Context) {
		switch c.Request.Method {
		case "POST", "PUT", "DELETE", "PATCH":
		default:
			c.Next()
			return
		}
		expected, _ := c.Get("blog.csrf")
		header := c.GetHeader("X-CSRF-Token")
		exp, _ := expected.(string)
		if exp == "" || header == "" || exp != header {
			httpresp.Fail(c, apperr.New(apperr.CodeCSRFInvalid, "安全校验失败"))
			return
		}
		c.Next()
	}
}
