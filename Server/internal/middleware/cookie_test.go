package middleware

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestSetSessionCookies_PersistsBoth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/", nil)

	SetSessionCookies(c, "sid-val", "csrf-val", 1800, false)
	cookies := w.Result().Cookies()
	require.Len(t, cookies, 2)
	bySid := map[string]string{}
	for _, ck := range cookies {
		bySid[ck.Name] = ck.Value
	}
	require.Equal(t, "sid-val", bySid["sid"])
	require.Equal(t, "csrf-val", bySid["csrf_token"])
	for _, ck := range cookies {
		require.Equal(t, 1800, ck.MaxAge)
	}
}

func TestClearSessionCookies(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/", nil)
	ClearSessionCookies(c, false)
	for _, ck := range w.Result().Cookies() {
		require.LessOrEqual(t, ck.MaxAge, 0)
	}
}
