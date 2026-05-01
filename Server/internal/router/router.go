package router

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"

	"github.com/wjr/blog/server/internal/handler"
	"github.com/wjr/blog/server/internal/middleware"
)

type Deps struct {
	Auth            *handler.AuthHandler
	Article         *handler.ArticleHandler
	Tag             *handler.TagHandler
	Upload          *handler.UploadHandler
	Sessions        *middleware.SessionStore
	RDB             *redis.Client
	StaticWebDir    string
	StaticUploadDir string
	SecureCookies   bool
	RateLimitIP     middleware.RateLimitOpts
	RateLimitUser   middleware.RateLimitOpts
}

func New(d Deps) *gin.Engine {
	r := gin.New()
	r.Use(middleware.Recover())
	r.Use(middleware.RequestLog())

	r.Static("/assets", d.StaticWebDir+"/assets")
	r.Static("/vendor", d.StaticWebDir+"/vendor")
	r.Static("/uploads", d.StaticUploadDir)
	r.GET("/", func(c *gin.Context) { c.Redirect(http.StatusFound, "/list.html") })
	r.StaticFile("/list.html", d.StaticWebDir+"/list.html")
	r.StaticFile("/login.html", d.StaticWebDir+"/login.html")
	r.StaticFile("/register.html", d.StaticWebDir+"/register.html")
	r.StaticFile("/detail.html", d.StaticWebDir+"/detail.html")
	r.StaticFile("/editor.html", d.StaticWebDir+"/editor.html")
	r.StaticFile("/profile.html", d.StaticWebDir+"/profile.html")

	api := r.Group("/api/v1")
	api.Use(d.Sessions.WithSession(d.SecureCookies))
	api.Use(middleware.RateLimit(d.RDB, d.RateLimitIP))

	api.POST("/auth/register", d.Auth.Register)
	api.POST("/auth/login", d.Auth.Login)
	api.POST("/auth/logout", d.Auth.Logout)
	api.GET("/auth/me", d.Auth.Me)

	api.GET("/articles", d.Article.List)
	api.GET("/articles/:id", d.Article.GetByID)
	api.GET("/tags", d.Tag.List)
	api.GET("/users/:id/articles", d.Article.List)

	priv := api.Group("/", middleware.RequireAuth(), middleware.CSRFGuard(),
		middleware.RateLimit(d.RDB, d.RateLimitUser))
	priv.POST("/articles", d.Article.Create)
	priv.PUT("/articles/:id", d.Article.Update)
	priv.DELETE("/articles/:id", d.Article.Delete)
	priv.POST("/uploads/image", d.Upload.Image)

	return r
}
