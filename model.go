package main

import "gorm.io/gorm"

type User struct {
	gorm.Model
	Name     string `form:"name" json:"name" binding:"required"`
	Password string `form:"password" json:"password" binding:"required"`
}

// Credentials 是登录/注册的请求体，避免客户端直接绑定到 User（防止塞 ID 等字段）
type Credentials struct {
	Name     string `form:"name" json:"name" binding:"required"`
	Password string `form:"password" json:"password" binding:"required"`
}

type Post struct {
	gorm.Model
	User     string `json:"user" binding:"required"`
	Title    string `json:"title" binding:"required"`
	Subtitle string `json:"subtitle"`
	Body     string `json:"body" binding:"required"`
}

// 设置的条目
type Setting struct {
	gorm.Model
	Key   string `json:"key"`
	Value string `json:"value"`
}
