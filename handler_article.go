package main

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// 创建文章 POST /api/post/new
func (d *Data) NewPost(c *gin.Context) {
	var post Post
	if err := c.ShouldBindJSON(&post); err != nil {
		Response{}.Fail(c, http.StatusBadRequest, "INVALID_BODY", err.Error())
		return
	}

	if err := d.DB.Create(&post).Error; err != nil {
		Response{}.Fail(c, http.StatusInternalServerError, "CREATE_FAILED", err.Error())
		return
	}

	Response{}.Ok(c, http.StatusCreated, post)
}

// 文章列表 GET /api/post
func (d *Data) ListPost(c *gin.Context) {
	posts, err := gorm.G[Post](d.DB).Find(c)
	if err != nil {
		Response{}.Fail(c, http.StatusInternalServerError, "QUERY_FAILED", err.Error())
		return
	}

	Response{}.Ok(c, http.StatusOK, posts)
}
