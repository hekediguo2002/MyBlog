package middleware

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestRateLimit_BlocksAfterMax(t *testing.T) {
	rdb := newRedisForTest(t)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RateLimit(rdb, RateLimitOpts{
		Name:   "test",
		Max:    3,
		Window: 60,
		KeyFn:  func(c *gin.Context) string { return "k" },
	}))
	r.POST("/x", func(c *gin.Context) { c.String(200, "ok") })

	hit := func() int {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("POST", "/x", nil))
		return w.Code
	}
	require.Equal(t, 200, hit())
	require.Equal(t, 200, hit())
	require.Equal(t, 200, hit())
	require.Equal(t, 429, hit())

	_ = rdb.Del(context.Background(), "rl:test:k").Err()
}

func TestRateLimit_FailsOpenWhenRedisDown(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	rdb := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1"})
	r.Use(RateLimit(rdb, RateLimitOpts{
		Name: "test", Max: 1, Window: 60, KeyFn: func(c *gin.Context) string { return "k" },
	}))
	r.POST("/x", func(c *gin.Context) { c.String(200, "ok") })
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("POST", "/x", nil))
	require.Equal(t, 200, w.Code)
}
