package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/wjr/blog/server/internal/apperr"
	"github.com/wjr/blog/server/internal/service"
)

type fakeTagSvc struct {
	out []service.TagView
	err error
}

func (f *fakeTagSvc) List(ctx context.Context) ([]service.TagView, error) {
	return f.out, f.err
}

func TestTagHandler_List_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &fakeTagSvc{out: []service.TagView{{Name: "go", ArticleCount: 3}}}
	h := NewTagHandler(svc)

	r := gin.New()
	r.GET("/api/v1/tags", h.List)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/tags", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var body struct {
		Code int               `json:"code"`
		Data []service.TagView `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.Equal(t, 0, body.Code)
	require.Len(t, body.Data, 1)
	require.Equal(t, "go", body.Data[0].Name)
}

func TestTagHandler_List_ServiceError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &fakeTagSvc{err: apperr.New(apperr.CodeDBError, "db down")}
	h := NewTagHandler(svc)

	r := gin.New()
	r.GET("/api/v1/tags", h.List)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/tags", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusInternalServerError, w.Code)
	require.Contains(t, w.Body.String(), `"code":5001`)
}

func TestTagHandler_List_Empty(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &fakeTagSvc{out: []service.TagView{}}
	h := NewTagHandler(svc)

	r := gin.New()
	r.GET("/api/v1/tags", h.List)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/tags", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), `"data":[]`)
}
