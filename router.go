package main

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func api(r *gin.Engine, db *gorm.DB, cfg *Config) error {
	data, err := NewData(db, cfg.JWT)
	if err != nil {
		return err
	}

	g := r.Group("/api")

	// 公开接口
	g.GET("/status", data.Status)    // 是否已初始化（有没有第一个用户）
	g.GET("/avatar", data.GetAvatar) // 公开：<img> 无法携带 Bearer
	g.POST("/authorize", data.Authorize)
	g.POST("/register", data.Register)
	g.GET("/uptime", data.Uptime)

	// 需要登录的接口
	authed := g.Group("")
	authed.Use(data.AuthRequired())
	authed.GET("/me", data.Me)
	authed.PUT("/me/avatar", data.UpdateAvatar)
	authed.DELETE("/me/avatar", data.DeleteAvatar)
	authed.GET("/post", data.GetPost)
	authed.GET("/recent-post-count", data.GetRecentPostCount)
	authed.GET("/post/:id", data.GetPostByID)
	authed.POST("/post/new", data.NewPost)
	authed.POST("/post/:id/reaction", data.ToggleReaction)
	authed.GET("/tag", data.GetTags)

	// 需要管理员权限的接口
	admin := g.Group("")
	admin.Use(data.AuthRequired(), data.AdminOnly())
	admin.POST("/tag", data.CreateTag)
	admin.PUT("/tag/:id", data.UpdateTag)
	admin.DELETE("/tag/:id", data.DeleteTag)

	// 用户管理
	admin.GET("/admin/users", data.GetAdminUsers)
	admin.GET("/admin/users/:id", data.GetAdminUserDetail)
	admin.PATCH("/admin/users/:id/status", data.SetAdminUserStatus)
	admin.PUT("/admin/users/:id/password", data.SetAdminUserPassword)
	admin.DELETE("/admin/users/:id", data.DeleteAdminUser)

	authed.GET("/me/posts", data.GetMyPosts)
	authed.PUT("/post/:id", data.UpdatePost)
	authed.DELETE("/post/:id", data.DeletePost)
	admin.PATCH("/post/:id/private", data.SetPostPrivate)
	authed.GET("/post/:id/comments", data.GetComments)
	authed.POST("/post/:id/comments", data.CreateComment)
	authed.GET("/me/stats", data.GetMyStats)
	admin.GET("/admin/stats", data.GetAdminStats)
	authed.GET("/leaderboard", data.GetLeaderboard)
	return nil
}
