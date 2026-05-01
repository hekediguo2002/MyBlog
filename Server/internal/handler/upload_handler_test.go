package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/wjr/blog/server/internal/middleware"
)

type fakeUploadSvc struct {
	called bool
	url    string
	err    error
}

func (f *fakeUploadSvc) SaveImage(ctx context.Context, fh *multipart.FileHeader) (string, error) {
	f.called = true
	return f.url, f.err
}

func makePNGForm(t *testing.T) (*bytes.Buffer, string) {
	t.Helper()
	body := &bytes.Buffer{}
	w := multipart.NewWriter(body)
	part, err := w.CreateFormFile("file", "x.png")
	require.NoError(t, err)
	require.NoError(t, png.Encode(part, image.NewRGBA(image.Rect(0, 0, 1, 1))))
	require.NoError(t, w.Close())
	return body, w.FormDataContentType()
}

func TestUploadHandler_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &fakeUploadSvc{url: "/uploads/abc.png"}
	h := NewUploadHandler(svc)
	r := gin.New()
	r.POST("/api/v1/uploads/image",
		func(c *gin.Context) {
			middleware.AttachSession(c, middleware.Session{UserID: 1, Name: "u1"})
			c.Next()
		},
		h.Image)

	body, ct := makePNGForm(t)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/uploads/image", body)
	req.Header.Set("Content-Type", ct)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var res struct {
		Code int `json:"code"`
		Data struct {
			URL string `json:"url"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &res))
	require.Equal(t, 0, res.Code)
	require.Equal(t, "/uploads/abc.png", res.Data.URL)
	require.True(t, svc.called)
}

func TestUploadHandler_MissingFile(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewUploadHandler(&fakeUploadSvc{})
	r := gin.New()
	r.POST("/api/v1/uploads/image",
		func(c *gin.Context) {
			middleware.AttachSession(c, middleware.Session{UserID: 1})
			c.Next()
		}, h.Image)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/uploads/image", io.NopCloser(bytes.NewReader(nil)))
	req.Header.Set("Content-Type", "multipart/form-data; boundary=xxx")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var res struct{ Code int `json:"code"` }
	_ = json.Unmarshal(w.Body.Bytes(), &res)
	require.Equal(t, 1001, res.Code)
}
