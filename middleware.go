package main

import (
	"errors"
	"net/http"

	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"gorm.io/gorm"
)

// ctxUserKey 是 AuthRequired 写入当前用户的 context key
const ctxUserKey = "auth.user"

// AuthRequired 校验 Bearer token，并把当前用户放进 context。
// 角色从数据库读取而不是信任 token 里的 role，这样封禁/降权可以立即生效。
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

// AdminOnly 要求当前用户的角色是 admin，需挂在 AuthRequired 之后
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

// CurrentUser 取出 AuthRequired 写入的当前用户
func CurrentUser(c *gin.Context) (User, bool) {
	value, ok := c.Get(ctxUserKey)
	if !ok {
		return User{}, false
	}

	user, ok := value.(User)
	return user, ok
}

// minGzipLength 是启用压缩的最小响应体字节数。
// 更小的响应经 gzip 压缩后往往反而更大，直接原样返回。
const minGzipLength = 1024

// gzipExcludedExtensions 是本身已压缩、再 gzip 没有收益的扩展名。
// 注意：WithExcludedExtensions 会替换库的默认排除表，所以 png/jpg/gif/jpeg 也要显式列出。
var gzipExcludedExtensions = []string{
	".png", ".jpg", ".jpeg", ".gif", ".webp", ".ico",
	".woff", ".woff2", ".gz", ".zip",
}

// gzipCompression 返回全局 gzip 压缩中间件：
//   - 仅当请求头带 Accept-Encoding: gzip 时压缩，并返回 Vary: Accept-Encoding；
//   - 小于 minGzipLength 的响应不压缩；
//   - 头像接口直接返回 JPEG/PNG，按路径前缀排除，避免无意义的二次压缩。
func gzipCompression() gin.HandlerFunc {
	return gzip.Gzip(
		gzip.DefaultCompression,
		gzip.WithMinLength(minGzipLength),
		gzip.WithExcludedExtensions(gzipExcludedExtensions),
		gzip.WithExcludedPaths([]string{"/api/avatar"}),
	)
}

// ErrorHandler 捕获 handler 通过 c.Error 追加的错误，并返回统一格式的错误响应。
// 注意：handler 自己已经写过响应（c.Writer.Written()）时不再重复写。
func ErrorHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next() // 先执行后续 handler

		if len(c.Errors) > 0 && !c.Writer.Written() {
			err := c.Errors.Last().Err
			Response{}.Fail(c, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		}
	}
}
