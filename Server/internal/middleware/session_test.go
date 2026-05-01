package middleware

import (
	"context"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func newRedisForTest(t *testing.T) *redis.Client {
	t.Helper()
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		t.Skip("REDIS_ADDR not set")
	}
	r := redis.NewClient(&redis.Options{Addr: addr})
	require.NoError(t, r.FlushDB(context.Background()).Err())
	t.Cleanup(func() { _ = r.Close() })
	return r
}

func TestSessionStore_PutGetTouchDelete(t *testing.T) {
	rdb := newRedisForTest(t)
	store := NewSessionStore(rdb, 30)
	ctx := context.Background()

	sid, csrf, err := store.Create(ctx, Session{UserID: 7, Name: "alice"})
	require.NoError(t, err)
	require.Len(t, sid, 32)
	require.Len(t, csrf, 32)

	got, err := store.Get(ctx, sid)
	require.NoError(t, err)
	require.Equal(t, uint64(7), got.UserID)

	gotCsrf, err := store.GetCSRF(ctx, sid)
	require.NoError(t, err)
	require.Equal(t, csrf, gotCsrf)

	require.NoError(t, store.Touch(ctx, sid))
	require.NoError(t, store.Delete(ctx, sid))

	_, err = store.Get(ctx, sid)
	require.Error(t, err)
}

func TestSessionStore_Get_Missing(t *testing.T) {
	rdb := newRedisForTest(t)
	store := NewSessionStore(rdb, 30)
	_, err := store.Get(context.Background(), "no-such")
	require.Error(t, err)
}

func TestSessionFromContext_AndAttach(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	AttachSession(c, Session{UserID: 9, Name: "bob"})
	got, ok := SessionFromContext(c)
	require.True(t, ok)
	require.Equal(t, uint64(9), got.UserID)
}
