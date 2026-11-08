package main

import (
	"embed"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/gin-gonic/gin"
)

//go:embed template/dist
var frontendFS embed.FS

func main() {
	StartTimer() // 启动计时器，记录进程启动时刻

	cfg, err := LoadConfig(defaultConfigPath)
	if err != nil {
		panic(fmt.Errorf("加载配置失败：%w", err))
	}

	// GIN_MODE 优先，未设置时用配置。
	if cfg.Server.Mode != "" && strings.TrimSpace(os.Getenv("GIN_MODE")) == "" {
		gin.SetMode(cfg.Server.Mode)
	}

	// 自动创建数据库目录。
	if err := ensureDatabaseDir(cfg.Database.Path); err != nil {
		panic(err)
	}

	// 提前检查写权限，容器里更容易定位权限问题。
	if err := checkDatabaseWritable(cfg.Database.Path); err != nil {
		panic(err)
	}

	r := gin.Default()

	// 全局 gzip。
	r.Use(gzipCompression())

	log.Printf("数据库文件：%s", cfg.Database.Path)

	db, err := openDatabase(cfg.Database.Path)
	if err != nil {
		panic(err)
	}

	// 建表。
	if err := db.AutoMigrate(&User{}, &Post{}, &Tag{}, &Reaction{}, &Setting{}); err != nil {
		panic(err)
	}

	// 启动时校准计数。
	if err := syncTagUsageCounts(db); err != nil {
		panic(err)
	}

	// 启动时校准计数。
	if err := syncReactionCounts(db); err != nil {
		panic(err)
	}

	// 注册 API。
	if err := api(r, db, cfg); err != nil {
		panic(err)
	}

	if gin.Mode() == gin.ReleaseMode {

		// 嵌入前端资源。
		distFS, err := fs.Sub(frontendFS, "template/dist")
		if err != nil {
			panic(err)
		}

		// 静态资源 + SPA 兜底。不能用 StaticFS：/*filepath 会和 /api 路由冲突。
		r.NoRoute(spaHandler(distFS))
	}

	addr := cfg.ListenAddr()
	log.Printf("监听地址：%s", addr)
	if err := r.Run(addr); err != nil {
		panic(err)
	}

}

// spaHandler：API 404，静态文件优先，其余回退 index.html。
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

		// 存在则直接返回，否则回退。
		if serveFile(c, distFS, name) {
			return
		}

		serveFile(c, distFS, "index.html")
	}
}

// serveFile 直接写文件，避免 FileFromFS 的 301 重定向。
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
