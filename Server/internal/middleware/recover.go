package middleware

import (
	"runtime/debug"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"

	"github.com/wjr/blog/server/internal/apperr"
	"github.com/wjr/blog/server/internal/pkg/httpresp"
)

func Recover() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if rec := recover(); rec != nil {
				log.Error().
					Str("path", c.Request.URL.Path).
					Bytes("stack", debug.Stack()).
					Interface("panic", rec).
					Msg("panic recovered")
				httpresp.Fail(c, apperr.New(apperr.CodeUnknown, "系统繁忙"))
			}
		}()
		c.Next()
	}
}
