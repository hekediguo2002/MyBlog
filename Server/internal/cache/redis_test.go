package cache

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestOpen_PingsLocalRedis(t *testing.T) {
	if os.Getenv("REDIS_ADDR") == "" {
		t.Skip("REDIS_ADDR not set; skipping smoke test")
	}
	rdb, err := Open(Options{Addr: os.Getenv("REDIS_ADDR"), DB: 0})
	require.NoError(t, err)
	t.Cleanup(func() { _ = rdb.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	require.NoError(t, rdb.Ping(ctx).Err())
}
