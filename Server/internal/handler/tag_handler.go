package handler

import (
	"context"

	"github.com/gin-gonic/gin"

	"github.com/wjr/blog/server/internal/pkg/httpresp"
	"github.com/wjr/blog/server/internal/service"
)

type TagSvc interface {
	List(ctx context.Context) ([]service.TagView, error)
}

type TagHandler struct {
	svc TagSvc
}

func NewTagHandler(svc TagSvc) *TagHandler {
	return &TagHandler{svc: svc}
}

func (h *TagHandler) List(c *gin.Context) {
	tags, err := h.svc.List(c.Request.Context())
	if err != nil {
		httpresp.Fail(c, err)
		return
	}
	httpresp.OK(c, tags)
}
