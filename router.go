package main

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func api(r *gin.Engine, db *gorm.DB, cfg *Config) (*Data, error) {
	r.Use(bodyLimit(defaultMaxRequestBodyBytes))

	data, err := NewData(db, cfg.JWT)
	if err != nil {
		return nil, err
	}

	data.AccessMode = cfg.Access.Mode
	if data.AccessMode == "" {
		data.AccessMode = AccessModePrivate
	}

	data.CaptchaEnabled = cfg.Captcha.Enabled
	data.RegisterEnabled = cfg.Register.Enabled
	data.limits = cfg.Limits
	data.initLimiters()

	g := r.Group("/api")

	// 维护模式下只放行白名单（状态、运行时长、登录）和管理员。
	g.Use(data.MaintenanceGuard())

	// 公开
	g.GET("/status", data.Status)    // 是否已初始化（有没有第一个用户）
	g.GET("/avatar", data.GetAvatar) // 公开：<img> 无法携带 Bearer
	g.GET("/captcha", data.GetCaptcha)
	g.POST("/captcha/verify", data.VerifyCaptcha)
	g.POST("/authorize", data.Authorize)
	g.POST("/register", data.Register)
	g.GET("/uptime", data.Uptime)

	// 只读：隐私模式必须登录，公开模式允许游客（登录后仍可看到自己的反应）。
	read := g.Group("")
	read.Use(data.ReadAuth())
	read.GET("/post", data.GetPost)
	read.GET("/post/:id", data.GetPostByID)
	read.GET("/post/:id/comments", data.GetComments)
	read.GET("/recent-post-count", data.GetRecentPostCount)
	read.GET("/tag", data.GetTags)
	read.GET("/leaderboard", data.GetLeaderboard)

	// 需登录
	authed := g.Group("")
	authed.Use(data.AuthRequired())
	authed.GET("/me", data.Me)
	authed.GET("/me/experience", data.GetExperience)
	authed.PUT("/me/avatar", data.UpdateAvatar)
	authed.DELETE("/me/avatar", data.DeleteAvatar)
	authed.POST("/post/new", data.NewPost)
	authed.POST("/post/:id/reaction", data.ToggleReaction)

	// 需管理员
	admin := g.Group("")
	admin.Use(data.AuthRequired(), data.AdminOnly())
	admin.POST("/tag", data.CreateTag)
	admin.PUT("/tag/:id", data.UpdateTag)
	admin.DELETE("/tag/:id", data.DeleteTag)

	admin.GET("/admin/users", data.GetAdminUsers)
	admin.GET("/admin/users/:id", data.GetAdminUserDetail)
	admin.PATCH("/admin/users/:id/status", data.SetAdminUserStatus)
	admin.PUT("/admin/users/:id/password", data.SetAdminUserPassword)
	admin.DELETE("/admin/users/:id", data.DeleteAdminUser)
	admin.DELETE("/admin/users/:id/posts", data.ClearAdminUserPosts)
	admin.PATCH("/admin/users/:id/trusted", data.SetAdminUserTrust)
	admin.DELETE("/admin/users/untrusted", data.DeleteAdminUntrustedUsers)
	admin.GET("/admin/cleanup", data.GetAdminCleanup)
	admin.DELETE("/admin/cleanup/:key", data.DeleteAdminCleanup)
	admin.DELETE("/admin/cleanup", data.DeleteAllAdminCleanup)
	admin.POST("/admin/cleanup/vacuum", data.VacuumAdminCleanup)
	admin.GET("/admin/maintenance", data.GetMaintenance)
	admin.PUT("/admin/maintenance", data.UpdateMaintenance)

	authed.GET("/me/posts", data.GetMyPosts)
	authed.PUT("/post/:id", data.UpdatePost)
	authed.DELETE("/post/:id", data.DeletePost)
	admin.PATCH("/post/:id/private", data.SetPostPrivate)
	authed.POST("/post/:id/comments", data.CreateComment)
	authed.GET("/me/stats", data.GetMyStats)
	admin.GET("/admin/stats", data.GetAdminStats)
	return data, nil
}
