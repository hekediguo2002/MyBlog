package middleware

import "github.com/gin-gonic/gin"

const (
	CookieSID  = "sid"
	CookieCSRF = "csrf_token"
)

func SetSessionCookies(c *gin.Context, sid, csrf string, maxAgeSec int, secure bool) {
	c.SetSameSite(2) // SameSiteLaxMode
	c.SetCookie(CookieSID, sid, maxAgeSec, "/", "", secure, true)
	c.SetCookie(CookieCSRF, csrf, maxAgeSec, "/", "", secure, false)
}

func ClearSessionCookies(c *gin.Context, secure bool) {
	c.SetSameSite(2)
	c.SetCookie(CookieSID, "", -1, "/", "", secure, true)
	c.SetCookie(CookieCSRF, "", -1, "/", "", secure, false)
}
