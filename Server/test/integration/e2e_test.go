//go:build integration
// +build integration

package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/mysql"
	"github.com/testcontainers/testcontainers-go/modules/redis"

	"github.com/wjr/blog/server/internal/cache"
	"github.com/wjr/blog/server/internal/db"
	"github.com/wjr/blog/server/internal/handler"
	"github.com/wjr/blog/server/internal/middleware"
	"github.com/wjr/blog/server/internal/model"
	"github.com/wjr/blog/server/internal/repository"
	"github.com/wjr/blog/server/internal/router"
	"github.com/wjr/blog/server/internal/service"
)

func startStack(t *testing.T) (mysqlDSN string, redisAddr string) {
	t.Helper()
	ctx := context.Background()
	myc, err := mysql.RunContainer(ctx,
		testcontainers.WithImage("mysql:8.0"),
		mysql.WithDatabase("blog"),
		mysql.WithUsername("blog"),
		mysql.WithPassword("blog"),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = myc.Terminate(ctx) })
	mysqlDSN, err = myc.ConnectionString(ctx, "parseTime=true&charset=utf8mb4")
	require.NoError(t, err)

	rc, err := redis.RunContainer(ctx, testcontainers.WithImage("redis:7-alpine"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = rc.Terminate(ctx) })
	host, _ := rc.Host(ctx)
	port, _ := rc.MappedPort(ctx, "6379/tcp")
	redisAddr = fmt.Sprintf("%s:%s", host, port.Port())
	return
}

func setupServer(t *testing.T) (*httptest.Server, http.CookieJar) {
	t.Helper()
	dsn, raddr := startStack(t)
	gdb, err := db.Open(db.DefaultOptions(dsn))
	require.NoError(t, err)
	rdb, err := cache.Open(cache.Default(raddr, 0))
	require.NoError(t, err)

	require.NoError(t, gdb.AutoMigrate(
		&model.User{}, &model.Article{}, &model.Tag{},
	))

	userRepo := repository.NewUserRepo(gdb)
	tagRepo := repository.NewTagRepo(gdb)
	articleRepo := repository.NewArticleRepo(gdb)
	counterRepo := repository.NewCounterRepo(rdb)
	sessions := middleware.NewSessionStore(rdb, 30)

	authH := handler.NewAuthHandler(service.NewAuthService(userRepo), sessions, false)
	articleH := handler.NewArticleHandler(service.NewArticleService(articleRepo, tagRepo, userRepo, counterRepo))
	tagH := handler.NewTagHandler(service.NewTagService(tagRepo))
	uploadH := handler.NewUploadHandler(service.NewUploadService(service.UploadOptions{
		Dir: t.TempDir(), MaxBytes: 5 << 20, AllowedMIME: []string{"image/png", "image/jpeg", "image/webp"},
	}))

	r := router.New(router.Deps{
		Auth: authH, Article: articleH, Tag: tagH, Upload: uploadH,
		Sessions: sessions, RDB: rdb,
		StaticWebDir:    t.TempDir(),
		StaticUploadDir: t.TempDir(),
		SecureCookies:   false,
		RateLimitIP:     middleware.RateLimitOpts{Name: "global", Max: 1000, Window: 60, KeyFn: middleware.IPKey},
		RateLimitUser:   middleware.RateLimitOpts{Name: "upload", Max: 1000, Window: 60, KeyFn: middleware.UserKey},
	})

	ts := httptest.NewServer(r)
	t.Cleanup(ts.Close)
	jar, _ := cookiejar.New(nil)
	return ts, jar
}

func TestE2E_RegisterLoginPostArticleViewCount(t *testing.T) {
	ts, jar := setupServer(t)
	c := &http.Client{Jar: jar}

	mustPOST(t, c, ts.URL+"/api/v1/auth/register",
		`{"username":"alice","password":"abc12345","name":"Alice"}`)
	csrf := getCookie(jar, ts.URL, "csrf_token")
	require.NotEmpty(t, csrf)

	body := mustPOSTWithCSRF(t, c, ts.URL+"/api/v1/articles", csrf,
		`{"title":"Hello","content":"# Hi\n\n这是一篇测试文章 12345","tags":["go","test"]}`)
	var created struct {
		Code int                `json:"code"`
		Data struct{ ID uint64 `json:"id"` } `json:"data"`
	}
	require.NoError(t, json.Unmarshal(body, &created))
	require.Equal(t, 0, created.Code)
	require.NotZero(t, created.Data.ID)

	for i := 0; i < 2; i++ {
		mustGET(t, c, fmt.Sprintf("%s/api/v1/articles/%d", ts.URL, created.Data.ID))
	}
	body = mustGET(t, c, fmt.Sprintf("%s/api/v1/articles/%d", ts.URL, created.Data.ID))
	var got struct {
		Code int `json:"code"`
		Data struct {
			ViewCount int64 `json:"view_count"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(body, &got))
	require.GreaterOrEqual(t, got.Data.ViewCount, int64(3))
}

func mustGET(t *testing.T, c *http.Client, u string) []byte {
	req, _ := http.NewRequest(http.MethodGet, u, nil)
	res, err := c.Do(req)
	require.NoError(t, err)
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	return b
}
func mustPOST(t *testing.T, c *http.Client, u, body string) []byte {
	req, _ := http.NewRequest(http.MethodPost, u, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	res, err := c.Do(req)
	require.NoError(t, err)
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	return b
}
func mustPOSTWithCSRF(t *testing.T, c *http.Client, u, csrf, body string) []byte {
	req, _ := http.NewRequest(http.MethodPost, u, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Token", csrf)
	res, err := c.Do(req)
	require.NoError(t, err)
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	return b
}
func getCookie(jar http.CookieJar, base, name string) string {
	u, _ := url.Parse(base)
	for _, c := range jar.Cookies(u) {
		if c.Name == name {
			return c.Value
		}
	}
	return ""
}
