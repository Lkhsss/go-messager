package main

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// UpdateAvatar 上传/替换头像 PUT /api/me/avatar
// 请求体是前端压缩后的 data URL（JPEG/PNG），服务端只校验并原样存库。
func (d *Data) UpdateAvatar(c *gin.Context) {
	user, ok := CurrentUser(c)
	if !ok {
		Response{}.Fail(c, http.StatusUnauthorized, "UNAUTHORIZED", "请先登录")
		return
	}

	var req AvatarInput
	if err := c.ShouldBindJSON(&req); err != nil {
		Response{}.Fail(c, http.StatusBadRequest, "INVALID_BODY", err.Error())
		return
	}

	avatar, err := normalizeAvatar(req.Avatar)
	if err != nil {
		code := "INVALID_AVATAR"
		switch {
		case errors.Is(err, errUnsupportedAvatar):
			code = "INVALID_AVATAR_TYPE"
		case errors.Is(err, errAvatarTooLarge):
			code = "AVATAR_TOO_LARGE"
		}
		Response{}.Fail(c, http.StatusBadRequest, code, err.Error())
		return
	}

	if err := d.UpdateUserAvatar(user.ID, avatar); err != nil {
		Response{}.Fail(c, http.StatusInternalServerError, "UPDATE_FAILED", err.Error())
		return
	}

	user.Avatar = avatar
	Response{}.Ok(c, http.StatusOK, user.View())
}

// DeleteAvatar 清除头像 DELETE /api/me/avatar
func (d *Data) DeleteAvatar(c *gin.Context) {
	user, ok := CurrentUser(c)
	if !ok {
		Response{}.Fail(c, http.StatusUnauthorized, "UNAUTHORIZED", "请先登录")
		return
	}

	if err := d.UpdateUserAvatar(user.ID, ""); err != nil {
		Response{}.Fail(c, http.StatusInternalServerError, "UPDATE_FAILED", err.Error())
		return
	}

	user.Avatar = ""
	Response{}.Ok(c, http.StatusOK, user.View())
}

// GetAvatar 读取用户头像 GET /api/avatar?name=alice
// 必须公开：<img> 标签无法携带 Bearer token，且头像本身不是敏感信息。
func (d *Data) GetAvatar(c *gin.Context) {
	name := strings.TrimSpace(c.Query("name"))
	if name == "" {
		Response{}.Fail(c, http.StatusBadRequest, "INVALID_NAME", "用户名不能为空")
		return
	}

	var user User
	if err := d.DB.Select("avatar").Where("name = ?", name).Take(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			Response{}.Fail(c, http.StatusNotFound, "AVATAR_NOT_FOUND", "该用户没有头像")
			return
		}
		Response{}.Fail(c, http.StatusInternalServerError, "QUERY_FAILED", err.Error())
		return
	}
	if user.Avatar == "" {
		Response{}.Fail(c, http.StatusNotFound, "AVATAR_NOT_FOUND", "该用户没有头像")
		return
	}

	mime, data, err := decodeAvatarDataURL(user.Avatar)
	if err != nil {
		Response{}.Fail(c, http.StatusNotFound, "AVATAR_NOT_FOUND", "头像不可用")
		return
	}

	etag := `"` + avatarVersion(user.Avatar) + `"`
	c.Header("Cache-Control", "public, max-age=86400")
	c.Header("ETag", etag)
	if c.GetHeader("If-None-Match") == etag {
		c.Status(http.StatusNotModified)
		return
	}

	c.Data(http.StatusOK, mime, data)
}
