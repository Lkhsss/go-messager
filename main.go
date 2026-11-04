package main

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"

	// "context"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

//go:embed template/dist
var frontendFS embed.FS

func main() {
	r := gin.Default()
	r.Use(ErrorHandler()) //加入错误处理中间件

	db, err := gorm.Open(sqlite.Open("data.db"), &gorm.Config{})
	if err != nil {
		panic(err) // 或 log.Fatal(err)
	}

	// ctx := context.Background()

	// 自动建表
	db.AutoMigrate(&User{}, &Post{}, &Setting{})

	distFS, err := fs.Sub(frontendFS, "template/dist")
	if err != nil {
		panic(err)
	}

	// 挂载api
	api(r, db)

	// 前端静态资源 + SPA 兜底
	// 注意：不能用 r.StaticFS("/", ...)，它注册的 /*filepath 是 catchAll 节点，
	// 注册后无法再加 /api/xxx 这样的精确路由，
	// 否则 panic: '/api/ping' conflicts with existing wildcard '/*filepath'。
	// 所以统一放到 NoRoute 处理：先找静态文件，找不到再回退 index.html。
	r.NoRoute(spaHandler(distFS))

	addr := "0.0.0.0:8080"
	if gin.Mode() == gin.DebugMode {
		addr = "127.0.0.1:8080"
	}

	r.Run(addr)

}

// spaHandler 兜底处理所有未匹配的路由：
//   - /api 开头的路径返回 JSON 404，不做前端回退；
//   - 静态文件存在则直接返回；
//   - 其余路径回退到 index.html，交给前端路由（vue-router）处理。
func spaHandler(distFS fs.FS) gin.HandlerFunc {
	return func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/api") {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}

		name := strings.TrimPrefix(c.Request.URL.Path, "/")
		if name == "" {
			name = "index.html"
		}

		// 静态文件存在则直接返回，否则回退 index.html
		if serveFile(c, distFS, name) {
			return
		}

		serveFile(c, distFS, "index.html")
	}
}

// serveFile 把 distFS 中的静态文件写入响应。
// 不用 c.FileFromFS：它内部的 http.FileServer 会对 /、目录等路径做 301 重定向，
// 导致 SPA 根路径跳转异常。这里直接把文件内容写回，并设置 Content-Type。
func serveFile(c *gin.Context, distFS fs.FS, name string) bool {
	info, err := fs.Stat(distFS, name)
	if err != nil || info.IsDir() {
		return false
	}

	data, err := fs.ReadFile(distFS, name)
	if err != nil {
		return false
	}

	c.Data(http.StatusOK, contentType(name), data)
	return true
}

func contentType(name string) string {
	switch strings.ToLower(path.Ext(name)) {
	case ".html":
		return "text/html; charset=utf-8"
	case ".js", ".mjs":
		return "text/javascript; charset=utf-8"
	case ".css":
		return "text/css; charset=utf-8"
	case ".json":
		return "application/json; charset=utf-8"
	case ".svg":
		return "image/svg+xml"
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".webp":
		return "image/webp"
	case ".ico":
		return "image/x-icon"
	case ".woff":
		return "font/woff"
	case ".woff2":
		return "font/woff2"
	case ".map":
		return "application/json; charset=utf-8"
	default:
		return "application/octet-stream"
	}
}
