package main

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type Data struct {
	DB *gorm.DB
}


// 创建文章 POST /api/post
func (d *Data) NewPost(c *gin.Context) {
	var post Post
	if err := c.ShouldBindJSON(&post); err != nil {
		c.Error(errors.New("Need JSON"))
		return
	}

	if strings.TrimSpace(post.Title) == "" {
		c.Error(errors.New("Need Title"))
		return
	}
	if strings.TrimSpace(post.User) == "" {
		c.Error(errors.New("Need User"))
		return
	}
	if post.Body == "" {
		c.Error(errors.New("Need Body"))
		return
	}

	if err := d.DB.Create(&post).Error; err != nil {
		c.Error(errors.New("Create Record Error!"))
		return
	}

	c.JSON(http.StatusCreated, post)
}

func (d *Data) ListPost(c *gin.Context) {
	_, err := gorm.G[Post](d.DB).Find(c)

	if err==nil{
		c.Error(errors.New("Get Record Error!"))
		return
	}

	
}
