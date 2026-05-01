package handler

import (
	"context"
	"mime/multipart"

	"github.com/gin-gonic/gin"

	"github.com/wjr/blog/server/internal/apperr"
	"github.com/wjr/blog/server/internal/middleware"
	"github.com/wjr/blog/server/internal/pkg/httpresp"
)

type UploadSvc interface {
	SaveImage(ctx context.Context, fh *multipart.FileHeader) (string, error)
}

type UploadHandler struct {
	svc UploadSvc
}

func NewUploadHandler(svc UploadSvc) *UploadHandler {
	return &UploadHandler{svc: svc}
}

type uploadResp struct {
	URL string `json:"url"`
}

func (h *UploadHandler) Image(c *gin.Context) {
	if _, ok := middleware.SessionFromContext(c); !ok {
		httpresp.Fail(c, apperr.New(apperr.CodeUnauthorized, "未登录"))
		return
	}
	fh, err := c.FormFile("file")
	if err != nil {
		httpresp.Fail(c, apperr.New(apperr.CodeInvalidParam, "缺少 file 字段"))
		return
	}
	url, err := h.svc.SaveImage(c.Request.Context(), fh)
	if err != nil {
		httpresp.Fail(c, err)
		return
	}
	httpresp.OK(c, uploadResp{URL: url})
}
