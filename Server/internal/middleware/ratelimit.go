package middleware

import (
	"context"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"

	"github.com/wjr/blog/server/internal/apperr"
	"github.com/wjr/blog/server/internal/pkg/httpresp"
)

type RateLimitOpts struct {
	Name   string                  // "login" / "upload" / "global"
	Max    int                     // 窗口内最大次数
	Window int                     // 秒
	KeyFn  func(c *gin.Context) string
}

func RateLimit(rdb *redis.Client, opts RateLimitOpts) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 200*time.Millisecond)
		defer cancel()

		key := "rl:" + opts.Name + ":" + opts.KeyFn(c)
		val, err := rdb.Incr(ctx, key).Result()
		if err != nil {
			c.Next() // fail open
			return
		}
		if val == 1 {
			_ = rdb.Expire(ctx, key, time.Duration(opts.Window)*time.Second).Err()
		}
		if val > int64(opts.Max) {
			c.Header("Retry-After", strconv.Itoa(opts.Window))
			httpresp.Fail(c, apperr.New(apperr.CodeRateLimited, "操作过于频繁,请稍后再试"))
			return
		}
		c.Next()
	}
}

func IPKey(c *gin.Context) string { return c.ClientIP() }

func UserKey(c *gin.Context) string {
	if s, ok := SessionFromContext(c); ok {
		return strconv.FormatUint(s.UserID, 10)
	}
	return "anon"
}
