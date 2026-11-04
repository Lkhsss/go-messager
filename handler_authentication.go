package main

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// Authorize 登录 POST /api/authorize
func (d *Data) Authorize(c *gin.Context) {
	var req Credentials
	if err := c.ShouldBind(&req); err != nil {
		Response{}.Fail(c, http.StatusBadRequest, "INVALID_BODY", err.Error())
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" || req.Password == "" {
		Response{}.Fail(c, http.StatusBadRequest, "NEED_CREDENTIALS", "用户名和密码不能为空")
		return
	}

	passwordHash, err := d.GetPassword(req.Name)
	if err != nil {
		// 用户不存在也返回 401，避免账号枚举
		if errors.Is(err, gorm.ErrRecordNotFound) {
			Response{}.Fail(c, http.StatusUnauthorized, "INVALID_CREDENTIALS", "用户名或密码错误")
			return
		}
		Response{}.Fail(c, http.StatusInternalServerError, "QUERY_FAILED", err.Error())
		return
	}

	// CompareHashAndPassword(hashedPassword, plainPassword)
	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(req.Password)); err != nil {
		Response{}.Fail(c, http.StatusUnauthorized, "INVALID_CREDENTIALS", "用户名或密码错误")
		return
	}

	Response{}.Ok(c, http.StatusOK, gin.H{"name": req.Name})
}

// Register 注册 POST /api/register
func (d *Data) Register(c *gin.Context) {
	var req Credentials
	if err := c.ShouldBind(&req); err != nil {
		Response{}.Fail(c, http.StatusBadRequest, "INVALID_BODY", err.Error())
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" || req.Password == "" {
		Response{}.Fail(c, http.StatusBadRequest, "NEED_CREDENTIALS", "用户名和密码不能为空")
		return
	}

	exists, err := d.IfUserExist(req.Name)
	if err != nil {
		Response{}.Fail(c, http.StatusInternalServerError, "QUERY_FAILED", err.Error())
		return
	}
	if exists {
		Response{}.Fail(c, http.StatusConflict, "USER_EXIST", "用户已存在")
		return
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		Response{}.Fail(c, http.StatusInternalServerError, "HASH_FAILED", err.Error())
		return
	}

	if err := d.CreateUser(req.Name, string(hashedPassword)); err != nil {
		Response{}.Fail(c, http.StatusInternalServerError, "CREATE_FAILED", err.Error())
		return
	}

	Response{}.Ok(c, http.StatusCreated, gin.H{"name": req.Name})
}
