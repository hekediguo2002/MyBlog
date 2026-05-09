package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/wjr/blog/server/internal/apperr"
	"github.com/wjr/blog/server/internal/pkg/httpresp"
	"github.com/wjr/blog/server/internal/service"
)

type AdminHandler struct {
	svc service.AdminService
}

func NewAdminHandler(svc service.AdminService) *AdminHandler {
	return &AdminHandler{svc: svc}
}

func (h *AdminHandler) ListUsers(c *gin.Context) {
	users, err := h.svc.ListUsers(c.Request.Context())
	if err != nil {
		httpresp.Fail(c, err)
		return
	}
	httpresp.OK(c, users)
}

func (h *AdminHandler) DeleteUser(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		httpresp.Fail(c, apperr.New(apperr.CodeInvalidParam, "id 非法"))
		return
	}
	if err := h.svc.DeleteUser(c.Request.Context(), id); err != nil {
		httpresp.Fail(c, err)
		return
	}
	httpresp.OK(c, nil)
}
