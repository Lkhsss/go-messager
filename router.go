package main

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func api(r *gin.Engine, db *gorm.DB) error {
	data, err := NewData(db)
	if err != nil {
		return err
	}

	g := r.Group("/api")

	// 公开接口
	g.GET("/status", data.Status) // 是否已初始化（有没有第一个用户）
	g.POST("/authorize", data.Authorize)
	g.POST("/register", data.Register)
	g.GET("/uptime", data.Uptime)

	// 需要登录的接口
	authed := g.Group("")
	authed.Use(data.AuthRequired())
	authed.GET("/me", data.Me)
	authed.GET("/post", data.GetPost)
	authed.GET("/post/:id", data.GetPostByID)
	authed.POST("/post/new", data.NewPost)
	authed.GET("/tag", data.GetTags)

	// 需要管理员权限的接口
	admin := g.Group("")
	admin.Use(data.AuthRequired(), data.AdminOnly())
	admin.POST("/tag", data.CreateTag)

	return nil
}

