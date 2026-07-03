package routers

import (
	"html/template"
	"time"

	gintemplate "github.com/foolin/gin-template"
	// 用于配置静态文件服务
	"github.com/gin-contrib/static"
	"github.com/gin-gonic/gin"

	"github.com/go-eagle/eagle/internal/web"
	webUser "github.com/go-eagle/eagle/internal/web/user"
	"github.com/go-eagle/eagle/pkg/flash"
	"github.com/go-eagle/eagle/pkg/log"
)

// LoadWebRouter loads the middlewares, routes, handlers.
func LoadWebRouter(g *gin.Engine) *gin.Engine {
	router := g

	// Middlewares.

	// 404 Handler.
	// 处理请求未配置的路由以及未配置的方法的请求返回404
	router.NoRoute(func(c *gin.Context) {
		web.Error404(c)
	})
	router.NoMethod(func(c *gin.Context) {
		web.Error404(c)
	})

	// 使用static中间件，静态文件服务
	router.Use(static.Serve("/static", static.LocalFile("./static", false)))

	//new template engine
	// 用的Gin Template引擎
	// 正式应用应该是独立的前端工程，这里只是为了方便演示
	router.HTMLRender = gintemplate.New(gintemplate.TemplateConfig{
		Root:      "internal/templates",
		Extension: ".html",
		Master:    "layouts/master",
		Partials:  []string{},
		Funcs: template.FuncMap{
			// 判断是否是当前链接
			"isActive": func(ctx *gin.Context, currentUri string) string {
				if ctx.Request.RequestURI == currentUri {
					return "is-active"
				}
				return ""
			},
			// 全局消息
			"flashMessage": func(ctx *gin.Context) string {
				errorMessage, err := flash.GetMessage(ctx.Writer, ctx.Request)
				if err != nil {
					log.Warnf("[router] get flash message err: %v", err)
					return ""
				}
				return string(errorMessage)
			},
			"hasFlash": func(ctx *gin.Context) bool {
				return flash.HasFlash(ctx.Request)
			},
			"copy": func() string {
				return time.Now().Format("2006")
			},
		},
		DisableCache: true,
	})

	router.GET("/", web.Index)

	// login
	router.GET("/login", webUser.GetLogin)

	// 绑定接口，这里的接口是传统 SSR 网站会保留 /login 的这类页面路由
	router.POST("/login", webUser.DoLogin)
	router.GET("/logout", webUser.Logout)

	// register
	router.GET("/register", webUser.GetRegister)
	router.POST("/register", webUser.DoRegister)

	return router
}
