package main

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func api(r *gin.Engine, db *gorm.DB) {
	data := &Data{DB: db}

	g := r.Group("/api")

	g.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "pong",
		})
	})
	g.POST("/authorize", data.Authorize)
	g.POST("/register", data.Register)
	g.POST("/post/new", data.NewPost)
	g.GET("/post", data.ListPost)
}
