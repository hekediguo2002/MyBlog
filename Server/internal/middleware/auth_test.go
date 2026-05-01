package middleware

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestRequireAuth_BlocksWhenNoSession(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/x", RequireAuth(), func(c *gin.Context) { c.String(200, "ok") })
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/x", nil))
	require.Equal(t, 401, w.Code)
	require.Contains(t, w.Body.String(), `"code":2001`)
}

func TestRequireAuth_AllowsWhenSessionAttached(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/x",
		func(c *gin.Context) { AttachSession(c, Session{UserID: 1, Name: "a"}); c.Next() },
		RequireAuth(),
		func(c *gin.Context) { c.String(200, "ok") })
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/x", nil))
	require.Equal(t, 200, w.Code)
}
