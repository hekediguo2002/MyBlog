package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/wjr/blog/server/internal/middleware"
	"github.com/wjr/blog/server/internal/model"
	"github.com/wjr/blog/server/internal/repository"
	"github.com/wjr/blog/server/internal/service"
)

func setupAuthEnv(t *testing.T) (*gin.Engine, *redis.Client) {
	t.Helper()
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		t.Skip("REDIS_ADDR not set")
	}
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.Article{}, &model.Tag{}))
	rdb := redis.NewClient(&redis.Options{Addr: addr})
	require.NoError(t, rdb.FlushDB(context.Background()).Err())

	store := middleware.NewSessionStore(rdb, 30)
	uRepo := repository.NewUserRepo(db)
	authSvc := service.NewAuthService(uRepo)
	h := NewAuthHandler(authSvc, store, false)

	r := gin.New()
	r.Use(store.WithSession(false))
	g := r.Group("/api/v1/auth")
	{
		g.POST("/register", h.Register)
		g.POST("/login", h.Login)
		g.POST("/logout", h.Logout)
		g.GET("/me", h.Me)
	}
	t.Cleanup(func() {
		_ = db.Migrator().DropTable(&model.User{}, &model.Article{}, &model.Tag{}, "article_tags")
	})
	return r, rdb
}

func doJSON(r *gin.Engine, method, path string, body any, sid, csrf string) *httptest.ResponseRecorder {
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if sid != "" {
		req.AddCookie(&http.Cookie{Name: "sid", Value: sid})
		req.AddCookie(&http.Cookie{Name: "csrf_token", Value: csrf})
		req.Header.Set("X-CSRF-Token", csrf)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestAuth_Register_SetsCookies(t *testing.T) {
	r, _ := setupAuthEnv(t)
	w := doJSON(r, "POST", "/api/v1/auth/register", map[string]string{
		"username": "alice", "password": "Password123", "name": "爱丽丝",
	}, "", "")
	require.Equal(t, 200, w.Code)
	cookies := w.Result().Cookies()
	gotNames := map[string]bool{}
	for _, ck := range cookies {
		gotNames[ck.Name] = true
		require.Equal(t, 1800, ck.MaxAge)
	}
	require.True(t, gotNames["sid"])
	require.True(t, gotNames["csrf_token"])
}

func TestAuth_Register_Duplicate(t *testing.T) {
	r, _ := setupAuthEnv(t)
	body := map[string]string{"username": "alice", "password": "Password123", "name": "爱丽丝"}
	doJSON(r, "POST", "/api/v1/auth/register", body, "", "")
	w := doJSON(r, "POST", "/api/v1/auth/register", body, "", "")
	require.Equal(t, 409, w.Code)
	require.Contains(t, w.Body.String(), `"code":1010`)
}

func TestAuth_Login_BadPassword(t *testing.T) {
	r, _ := setupAuthEnv(t)
	doJSON(r, "POST", "/api/v1/auth/register", map[string]string{
		"username": "alice", "password": "Password123", "name": "A",
	}, "", "")
	w := doJSON(r, "POST", "/api/v1/auth/login", map[string]string{
		"username": "alice", "password": "wrong",
	}, "", "")
	require.Equal(t, 401, w.Code)
	require.Contains(t, w.Body.String(), `"code":2010`)
}

func TestAuth_Me_Returns401Without_Login(t *testing.T) {
	r, _ := setupAuthEnv(t)
	w := doJSON(r, "GET", "/api/v1/auth/me", nil, "", "")
	require.Equal(t, 401, w.Code)
}

func TestAuth_LoginThenMeThenLogout(t *testing.T) {
	r, _ := setupAuthEnv(t)
	doJSON(r, "POST", "/api/v1/auth/register", map[string]string{
		"username": "alice", "password": "Password123", "name": "A",
	}, "", "")
	w := doJSON(r, "POST", "/api/v1/auth/login", map[string]string{
		"username": "alice", "password": "Password123",
	}, "", "")
	require.Equal(t, 200, w.Code)
	var sid, csrf string
	for _, ck := range w.Result().Cookies() {
		switch ck.Name {
		case "sid":
			sid = ck.Value
		case "csrf_token":
			csrf = ck.Value
		}
	}
	require.NotEmpty(t, sid)

	w2 := doJSON(r, "GET", "/api/v1/auth/me", nil, sid, csrf)
	require.Equal(t, 200, w2.Code)
	require.Contains(t, w2.Body.String(), `"username":"alice"`)

	w3 := doJSON(r, "POST", "/api/v1/auth/logout", nil, sid, csrf)
	require.Equal(t, 200, w3.Code)

	w4 := doJSON(r, "GET", "/api/v1/auth/me", nil, sid, csrf)
	require.Equal(t, 401, w4.Code)
}
