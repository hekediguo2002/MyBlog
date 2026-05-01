package httpresp

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/wjr/blog/server/internal/apperr"
)

func TestOK_WritesEnvelope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	OK(c, gin.H{"id": 7})

	require.Equal(t, 200, w.Code)
	var got map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	require.Equal(t, float64(0), got["code"])
	require.Equal(t, "ok", got["msg"])
	data := got["data"].(map[string]any)
	require.Equal(t, float64(7), data["id"])
}

func TestFail_RendersAppErr(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	Fail(c, apperr.New(apperr.CodeUsernameTaken, "用户名已被使用"))

	require.Equal(t, 409, w.Code)
	var got map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	require.Equal(t, float64(1010), got["code"])
	require.Equal(t, "用户名已被使用", got["msg"])
	require.Nil(t, got["data"])
}

func TestFail_NonAppErr_RendersUnknown(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	Fail(c, errAdHoc("boom"))
	require.Equal(t, 500, w.Code)
	var got map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	require.Equal(t, float64(5099), got["code"])
}

type errAdHoc string

func (e errAdHoc) Error() string { return string(e) }
