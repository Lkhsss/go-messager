package main

import (
	"embed"
	"io/fs"
	"net/http"

	"github.com/gin-gonic/gin"
)

//go:embed template/dist
var frontendFS embed.FS

func main() {
	r := gin.Default()

	distFS, err := fs.Sub(frontendFS, "template/dist")
	if err != nil {
		panic(err)
	}

	// 挂载到 /
	r.StaticFS("/", http.FS(distFS))

	r.Run()
}
