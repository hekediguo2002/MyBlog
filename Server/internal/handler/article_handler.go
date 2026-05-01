package handler

import (
	"context"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/wjr/blog/server/internal/apperr"
	"github.com/wjr/blog/server/internal/middleware"
	"github.com/wjr/blog/server/internal/pkg/httpresp"
	"github.com/wjr/blog/server/internal/service"
)

type ArticleSvc interface {
	Create(ctx context.Context, uid uint64, in service.CreateArticleInput) (*service.ArticleView, error)
	Update(ctx context.Context, uid, id uint64, in service.UpdateArticleInput) (*service.ArticleView, error)
	Delete(ctx context.Context, uid, id uint64) error
	GetByID(ctx context.Context, id uint64, incView bool) (*service.ArticleView, error)
	List(ctx context.Context, in service.ListArticlesInput) ([]service.ArticleView, int64, error)
}

type ArticleHandler struct {
	svc ArticleSvc
}

func NewArticleHandler(svc ArticleSvc) *ArticleHandler {
	return &ArticleHandler{svc: svc}
}

type articleCreateReq struct {
	Title   string   `json:"title"`
	Content string   `json:"content"`
	Tags    []string `json:"tags"`
}

type articleUpdateReq = articleCreateReq

type listResp struct {
	Items []service.ArticleView `json:"items"`
	Total int64                 `json:"total"`
	Page  int                   `json:"page"`
	Size  int                   `json:"size"`
}

func (h *ArticleHandler) Create(c *gin.Context) {
	sess, ok := middleware.SessionFromContext(c)
	if !ok {
		httpresp.Fail(c, apperr.New(apperr.CodeUnauthorized, "未登录"))
		return
	}
	var req articleCreateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		httpresp.Fail(c, apperr.New(apperr.CodeInvalidParam, "请求体格式错误"))
		return
	}
	out, err := h.svc.Create(c.Request.Context(), sess.UserID, service.CreateArticleInput{
		Title: req.Title, Content: req.Content, Tags: req.Tags,
	})
	if err != nil {
		httpresp.Fail(c, err)
		return
	}
	httpresp.OK(c, out)
}

func (h *ArticleHandler) Update(c *gin.Context) {
	sess, ok := middleware.SessionFromContext(c)
	if !ok {
		httpresp.Fail(c, apperr.New(apperr.CodeUnauthorized, "未登录"))
		return
	}
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		httpresp.Fail(c, apperr.New(apperr.CodeInvalidParam, "id 非法"))
		return
	}
	var req articleUpdateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		httpresp.Fail(c, apperr.New(apperr.CodeInvalidParam, "请求体格式错误"))
		return
	}
	out, err := h.svc.Update(c.Request.Context(), sess.UserID, id, service.UpdateArticleInput{
		Title: req.Title, Content: req.Content, Tags: req.Tags,
	})
	if err != nil {
		httpresp.Fail(c, err)
		return
	}
	httpresp.OK(c, out)
}

func (h *ArticleHandler) Delete(c *gin.Context) {
	sess, ok := middleware.SessionFromContext(c)
	if !ok {
		httpresp.Fail(c, apperr.New(apperr.CodeUnauthorized, "未登录"))
		return
	}
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		httpresp.Fail(c, apperr.New(apperr.CodeInvalidParam, "id 非法"))
		return
	}
	if err := h.svc.Delete(c.Request.Context(), sess.UserID, id); err != nil {
		httpresp.Fail(c, err)
		return
	}
	httpresp.OK(c, nil)
}

func (h *ArticleHandler) GetByID(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		httpresp.Fail(c, apperr.New(apperr.CodeInvalidParam, "id 非法"))
		return
	}
	out, err := h.svc.GetByID(c.Request.Context(), id, true)
	if err != nil {
		httpresp.Fail(c, err)
		return
	}
	httpresp.OK(c, out)
}

func (h *ArticleHandler) List(c *gin.Context) {
	in := service.ListArticlesInput{
		Page: parseIntDefault(c.Query("page"), 1),
		Size: parseIntDefault(c.Query("size"), 10),
		Tag:  strings.TrimSpace(c.Query("tag")),
	}
	if idStr := c.Param("id"); idStr != "" {
		if uid, err := strconv.ParseUint(idStr, 10, 64); err == nil {
			in.UserID = uid
		}
	}
	if in.UserID == 0 {
		if uid, err := strconv.ParseUint(c.Query("user_id"), 10, 64); err == nil {
			in.UserID = uid
		}
	}
	if in.Page < 1 {
		in.Page = 1
	}
	if in.Size < 1 || in.Size > 50 {
		in.Size = 10
	}
	items, total, err := h.svc.List(c.Request.Context(), in)
	if err != nil {
		httpresp.Fail(c, err)
		return
	}
	httpresp.OK(c, listResp{Items: items, Total: total, Page: in.Page, Size: in.Size})
}

func parseIntDefault(s string, def int) int {
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return n
}
