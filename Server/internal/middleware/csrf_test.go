package middleware

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestCSRF_AllowsGET(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(CSRFGuard())
	r.GET("/x", func(c *gin.Context) { c.String(200, "ok") })
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/x", nil))
	require.Equal(t, 200, w.Code)
}

func TestCSRF_BlocksPOST_Mismatch(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("blog.csrf", "expected"); c.Next() })
	r.Use(CSRFGuard())
	r.POST("/x", func(c *gin.Context) { c.String(200, "ok") })
	req := httptest.NewRequest("POST", "/x", strings.NewReader(""))
	req.Header.Set("X-CSRF-Token", "wrong")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, 403, w.Code)
	require.Contains(t, w.Body.String(), `"code":2030`)
}

func TestCSRF_AllowsPOST_Match(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("blog.csrf", "ok-token"); c.Next() })
	r.Use(CSRFGuard())
	r.POST("/x", func(c *gin.Context) { c.String(200, "yes") })
	req := httptest.NewRequest("POST", "/x", strings.NewReader(""))
	req.Header.Set("X-CSRF-Token", "ok-token")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, 200, w.Code)
}
