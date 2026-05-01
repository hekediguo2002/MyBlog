package middleware

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

func RequestLog() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		dur := time.Since(start)
		ev := log.Info()
		if c.Writer.Status() >= 500 {
			ev = log.Error()
		} else if c.Writer.Status() >= 400 {
			ev = log.Warn()
		}
		ev.Str("method", c.Request.Method).
			Str("path", c.Request.URL.Path).
			Int("status", c.Writer.Status()).
			Dur("dur", dur).
			Str("ip", c.ClientIP()).
			Msg("http")
	}
}
