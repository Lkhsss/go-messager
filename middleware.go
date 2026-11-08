package main

import (
	"errors"
	"net/http"

	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"gorm.io/gorm"
)

// 当前用户在 context 中的 key。
const ctxUserKey = "auth.user"

// AuthRequired 校验 token，并从数据库加载用户（角色以库为准）。
func (d *Data) AuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		raw, ok := bearerToken(c.GetHeader("Authorization"))
		if !ok {
			Response{}.Fail(c, http.StatusUnauthorized, "UNAUTHORIZED", "请先登录")
			c.Abort()
			return
		}

		claims, err := d.parseToken(raw)
		if err != nil {
			code, message := "INVALID_TOKEN", "登录状态无效，请重新登录"
			if errors.Is(err, jwt.ErrTokenExpired) {
				code, message = "TOKEN_EXPIRED", "登录已过期，请重新登录"
			}
			Response{}.Fail(c, http.StatusUnauthorized, code, message)
			c.Abort()
			return
		}

		id, err := userIDFromClaims(claims)
		if err != nil {
			Response{}.Fail(c, http.StatusUnauthorized, "INVALID_TOKEN", "登录状态无效，请重新登录")
			c.Abort()
			return
		}

		user, err := d.GetUserByID(id)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				Response{}.Fail(c, http.StatusUnauthorized, "INVALID_TOKEN", "登录状态无效，请重新登录")
			} else {
				Response{}.Fail(c, http.StatusInternalServerError, "QUERY_FAILED", err.Error())
			}
			c.Abort()
			return
		}

		if user.Role == RoleNone {
			Response{}.Fail(c, http.StatusForbidden, "ACCOUNT_DISABLED", "账号已被停用")
			c.Abort()
			return
		}

		c.Set(ctxUserKey, user)
		c.Next()
	}
}

// AdminOnly 要求管理员，需在 AuthRequired 之后。
func (d *Data) AdminOnly() gin.HandlerFunc {
	return func(c *gin.Context) {
		user, ok := CurrentUser(c)
		if !ok {
			Response{}.Fail(c, http.StatusUnauthorized, "UNAUTHORIZED", "请先登录")
			c.Abort()
			return
		}

		if user.Role != RoleAdmin {
			Response{}.Fail(c, http.StatusForbidden, "FORBIDDEN", "需要管理员权限")
			c.Abort()
			return
		}

		c.Next()
	}
}

// CurrentUser 取出当前用户。
func CurrentUser(c *gin.Context) (User, bool) {
	value, ok := c.Get(ctxUserKey)
	if !ok {
		return User{}, false
	}

	user, ok := value.(User)
	return user, ok
}

// 已压缩格式，跳过 gzip。
var gzipExcludedExtensions = []string{
	".png", ".jpg", ".jpeg", ".gif", ".webp", ".ico",
	".woff", ".woff2", ".gz", ".zip",
}

// gzipCompression 全局 gzip。不使用 WithMinLength：v1.2.9 会让小响应
// 声明 Content-Encoding: gzip 但正文未压缩，导致前端解码失败。
func gzipCompression() gin.HandlerFunc {
	return gzip.Gzip(
		gzip.DefaultCompression,
		gzip.WithExcludedExtensions(gzipExcludedExtensions),
		gzip.WithExcludedPaths([]string{"/api/avatar"}),
	)
}
