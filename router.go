package main

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func api(r *gin.Engine, db *gorm.DB) {
	data := &Data{DB: db}

	g := r.Group("/api")

	g.POST("/authorize", data.Authorize)
	g.POST("/register", data.Register)
	g.POST("/post/new", data.NewPost)
	g.GET("/post", data.GetPost)
	g.GET("/uptime", data.Uptime)
}
