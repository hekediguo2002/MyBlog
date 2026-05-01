package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/wjr/blog/server/internal/apperr"
	"github.com/wjr/blog/server/internal/middleware"
	"github.com/wjr/blog/server/internal/service"
)

type fakeArticleSvc struct {
	createIn  service.CreateArticleInput
	createOut *service.ArticleView
	createErr error

	updateIn  service.UpdateArticleInput
	updateOut *service.ArticleView
	updateErr error

	deleteErr error

	getOut *service.ArticleView
	getErr error

	listIn    service.ListArticlesInput
	listOut   []service.ArticleView
	listTotal int64
	listErr   error
}

func (f *fakeArticleSvc) Create(ctx context.Context, uid uint64, in service.CreateArticleInput) (*service.ArticleView, error) {
	f.createIn = in
	return f.createOut, f.createErr
}
func (f *fakeArticleSvc) Update(ctx context.Context, uid, id uint64, in service.UpdateArticleInput) (*service.ArticleView, error) {
	f.updateIn = in
	return f.updateOut, f.updateErr
}
func (f *fakeArticleSvc) Delete(ctx context.Context, uid, id uint64) error { return f.deleteErr }
func (f *fakeArticleSvc) GetByID(ctx context.Context, id uint64, incView bool) (*service.ArticleView, error) {
	return f.getOut, f.getErr
}
func (f *fakeArticleSvc) List(ctx context.Context, in service.ListArticlesInput) ([]service.ArticleView, int64, error) {
	f.listIn = in
	return f.listOut, f.listTotal, f.listErr
}

func withSession(uid uint64) gin.HandlerFunc {
	return func(c *gin.Context) {
		middleware.AttachSession(c, middleware.Session{UserID: uid, Name: "tester"})
		c.Next()
	}
}

func TestArticleHandler_Create_RequiresAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewArticleHandler(&fakeArticleSvc{})
	r := gin.New()
	r.POST("/api/v1/articles", h.Create)
	body := bytes.NewBufferString(`{"title":"t","content":"x"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/articles", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var res struct{ Code int `json:"code"` }
	_ = json.Unmarshal(w.Body.Bytes(), &res)
	require.Equal(t, apperr.CodeUnauthorized, res.Code)
}

func TestArticleHandler_Create_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &fakeArticleSvc{createOut: &service.ArticleView{ID: 1, Title: "T"}}
	h := NewArticleHandler(svc)
	r := gin.New()
	r.POST("/api/v1/articles", withSession(1), h.Create)

	body := bytes.NewBufferString(`{"title":"hello","content":"world world world","tags":["a","b"]}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/articles", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "hello", svc.createIn.Title)
	require.Equal(t, []string{"a", "b"}, svc.createIn.Tags)
}

func TestArticleHandler_GetByID_IncrementsView(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &fakeArticleSvc{getOut: &service.ArticleView{ID: 1, Title: "T", ViewCount: 7}}
	h := NewArticleHandler(svc)
	r := gin.New()
	r.GET("/api/v1/articles/:id", h.GetByID)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/articles/1", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var res struct {
		Code int                  `json:"code"`
		Data *service.ArticleView `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &res))
	require.Equal(t, 0, res.Code)
	require.Equal(t, int64(7), res.Data.ViewCount)
}

func TestArticleHandler_List_PaginationDefaults(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &fakeArticleSvc{listOut: []service.ArticleView{{ID: 1}}, listTotal: 1}
	h := NewArticleHandler(svc)
	r := gin.New()
	r.GET("/api/v1/articles", h.List)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/articles", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, 1, svc.listIn.Page)
	require.Equal(t, 10, svc.listIn.Size)
}

func TestArticleHandler_List_QueryParams(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &fakeArticleSvc{listOut: []service.ArticleView{{ID: 2}}, listTotal: 5}
	h := NewArticleHandler(svc)
	r := gin.New()
	r.GET("/api/v1/articles", h.List)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/articles?page=2&size=5&tag=go&user_id=3", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, 2, svc.listIn.Page)
	require.Equal(t, 5, svc.listIn.Size)
	require.Equal(t, "go", svc.listIn.Tag)
	require.Equal(t, uint64(3), svc.listIn.UserID)
}

func TestArticleHandler_Update_ForbidsOtherUser(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &fakeArticleSvc{updateErr: apperr.New(apperr.CodeForbidden, "无权操作")}
	h := NewArticleHandler(svc)
	r := gin.New()
	r.PUT("/api/v1/articles/:id", withSession(2), h.Update)

	body := bytes.NewBufferString(`{"title":"new","content":"body body body body"}`)
	req := httptest.NewRequest(http.MethodPut, "/api/v1/articles/1", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusForbidden, w.Code)
	require.Contains(t, w.Body.String(), `"code":2002`)
}

func TestArticleHandler_Delete_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &fakeArticleSvc{}
	h := NewArticleHandler(svc)
	r := gin.New()
	r.DELETE("/api/v1/articles/:id", withSession(1), h.Delete)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/articles/5", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
}
