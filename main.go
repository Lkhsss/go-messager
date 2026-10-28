package main

import (
	"embed"
	"io/fs"
	"net/http"

	// "context"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// import (
//   "go-messager/model"
// )

//go:embed template/dist
var frontendFS embed.FS

func main() {
	r := gin.Default()
	db, err := gorm.Open(sqlite.Open("test.db"), &gorm.Config{})

	if err != nil {
		panic(err) // 或 log.Fatal(err)
	}

	// ctx := context.Background()

	// 自动建表
	db.AutoMigrate(&User{})

	distFS, err := fs.Sub(frontendFS, "template/dist")
	if err != nil {
		panic(err)
	}

	// 挂载到 /
	r.StaticFS("/", http.FS(distFS))

	r.NoRoute(func(c *gin.Context) {
		// 只对非 API 路径做前端回退
		if len(c.Request.URL.Path) >= 4 && c.Request.URL.Path[:4] == "/api" {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		c.FileFromFS("index.html", http.FS(distFS))
	})

	r.Run()
}
