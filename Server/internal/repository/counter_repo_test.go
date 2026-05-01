package repository

import (
	"context"
	"os"
	"testing"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func skipIfNoRedis(t *testing.T) *redis.Client {
	t.Helper()
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		t.Skip("REDIS_ADDR not set")
	}
	rdb := redis.NewClient(&redis.Options{Addr: addr})
	require.NoError(t, rdb.Ping(context.Background()).Err())
	require.NoError(t, rdb.FlushDB(context.Background()).Err())
	t.Cleanup(func() { _ = rdb.Close() })
	return rdb
}

func TestCounterRepo_IncAndGet(t *testing.T) {
	rdb := skipIfNoRedis(t)
	c := NewCounterRepo(rdb)
	ctx := context.Background()

	require.NoError(t, c.Inc(ctx, 42))
	require.NoError(t, c.Inc(ctx, 42))
	require.NoError(t, c.Inc(ctx, 43))

	got, err := c.GetIncrement(ctx, 42)
	require.NoError(t, err)
	require.Equal(t, int64(2), got)

	dirty, err := c.DirtyMembers(ctx)
	require.NoError(t, err)
	require.ElementsMatch(t, []uint64{42, 43}, dirty)
}

func TestCounterRepo_DrainAndAck(t *testing.T) {
	rdb := skipIfNoRedis(t)
	c := NewCounterRepo(rdb)
	ctx := context.Background()
	for i := 0; i < 5; i++ {
		require.NoError(t, c.Inc(ctx, 7))
	}
	delta, err := c.DrainIncrement(ctx, 7)
	require.NoError(t, err)
	require.Equal(t, int64(5), delta)

	again, err := c.GetIncrement(ctx, 7)
	require.NoError(t, err)
	require.Equal(t, int64(0), again)

	require.NoError(t, c.Ack(ctx, []uint64{7}))
	dirty, _ := c.DirtyMembers(ctx)
	require.Empty(t, dirty)
}

func TestCounterRepo_RestoreOnFlushFail(t *testing.T) {
	rdb := skipIfNoRedis(t)
	c := NewCounterRepo(rdb)
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		require.NoError(t, c.Inc(ctx, 9))
	}
	delta, _ := c.DrainIncrement(ctx, 9)
	require.Equal(t, int64(3), delta)
	require.NoError(t, c.Restore(ctx, 9, delta))
	got, _ := c.GetIncrement(ctx, 9)
	require.Equal(t, int64(3), got)
}
