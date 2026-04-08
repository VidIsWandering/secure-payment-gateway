package handler

import (
	"io/fs"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"secure-payment-gateway/web"
)

// UIHandler is responsible for serving the frontend UI files
type UIHandler struct {
	serverMode string
}

func NewUIHandler(serverMode string) *UIHandler {
	return &UIHandler{serverMode: serverMode}
}

// RegisterRoutes registers all HTML and static file routes
func (h *UIHandler) RegisterRoutes(r *gin.Engine) {
	// Root Static Files (CSS, JS, Images)
	if h.serverMode == "debug" {
		// Live reload in development mode via volume mount
		if _, err := os.Stat("web"); err == nil {
			r.StaticFS("/public", http.Dir("web/public"))
			r.StaticFS("/css", http.Dir("web/css"))
			r.StaticFS("/js", http.Dir("web/js"))
			
			// Pages
			r.GET("/", func(c *gin.Context) { c.File("web/pages/index.html") })
			r.GET("/login", func(c *gin.Context) { c.File("web/pages/auth/login.html") })
			r.GET("/register", func(c *gin.Context) { c.File("web/pages/auth/register.html") })
			r.GET("/dashboard", func(c *gin.Context) { c.File("web/pages/dashboard/index.html") })
			r.GET("/checkout-demo", func(c *gin.Context) { c.File("web/pages/checkout/demo.html") })
			return
		}
	}

	// Production Embedded Files Mode
	publicFS, _ := fs.Sub(web.FS, "public")
	cssFS, _ := fs.Sub(web.FS, "css")
	jsFS, _ := fs.Sub(web.FS, "js")
	pagesFS, _ := fs.Sub(web.FS, "pages")

	r.StaticFS("/public", http.FS(publicFS))
	r.StaticFS("/css", http.FS(cssFS))
	r.StaticFS("/js", http.FS(jsFS))

	r.GET("/", func(c *gin.Context) {
		page, _ := fs.ReadFile(pagesFS, "index.html")
		c.Data(http.StatusOK, "text/html; charset=utf-8", page)
	})
	r.GET("/login", func(c *gin.Context) {
		page, _ := fs.ReadFile(pagesFS, "auth/login.html")
		c.Data(http.StatusOK, "text/html; charset=utf-8", page)
	})
	r.GET("/register", func(c *gin.Context) {
		page, _ := fs.ReadFile(pagesFS, "auth/register.html")
		c.Data(http.StatusOK, "text/html; charset=utf-8", page)
	})
	r.GET("/dashboard", func(c *gin.Context) {
		page, _ := fs.ReadFile(pagesFS, "dashboard/index.html")
		c.Data(http.StatusOK, "text/html; charset=utf-8", page)
	})
	r.GET("/checkout-demo", func(c *gin.Context) {
		page, _ := fs.ReadFile(pagesFS, "checkout/demo.html")
		c.Data(http.StatusOK, "text/html; charset=utf-8", page)
	})
}
