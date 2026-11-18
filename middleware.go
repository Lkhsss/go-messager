package main

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"gorm.io/gorm"
)

// 当前用户在 context 中的 key。
const ctxUserKey = "auth.user"

// authFailure 认证失败信息。
type authFailure struct {
	status  int
	code    string
	message string
}

// authenticate 解析 token 并从数据库加载用户（角色以库为准）。
// 同时返回 claims，供滑动续期判断剩余有效期。
func (d *Data) authenticate(c *gin.Context) (User, *Claims, *authFailure) {
	raw, ok := bearerToken(c.GetHeader("Authorization"))
	if !ok {
		return User{}, nil, &authFailure{status: http.StatusUnauthorized, code: "UNAUTHORIZED", message: "请先登录"}
	}

	claims, err := d.parseToken(raw)
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return User{}, nil, &authFailure{status: http.StatusUnauthorized, code: "TOKEN_EXPIRED", message: "登录已过期，请重新登录"}
		}
		return User{}, nil, &authFailure{status: http.StatusUnauthorized, code: "INVALID_TOKEN", message: "登录状态无效，请重新登录"}
	}

	id, err := userIDFromClaims(claims)
	if err != nil {
		return User{}, nil, &authFailure{status: http.StatusUnauthorized, code: "INVALID_TOKEN", message: "登录状态无效，请重新登录"}
	}

	user, err := d.GetUserByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return User{}, nil, &authFailure{status: http.StatusUnauthorized, code: "INVALID_TOKEN", message: "登录状态无效，请重新登录"}
		}
		return User{}, nil, &authFailure{status: http.StatusInternalServerError, code: "QUERY_FAILED", message: err.Error()}
	}

	if user.Role == RoleNone {
		return User{}, nil, &authFailure{status: http.StatusForbidden, code: "ACCOUNT_DISABLED", message: "账号已被停用"}
	}

	return user, claims, nil
}

// 续期响应头：前端读到后替换本地 token 与过期时间。
const (
	headerAuthToken     = "X-Auth-Token"
	headerAuthExpiresAt = "X-Auth-Expires-At"
)

// renewWindow 滑动续期阈值：TTL/4，限制在 30s-12h 之间且不超过 TTL/2。
func (d *Data) renewWindow() time.Duration {
	window := d.JWTTTL / 4
	if window < 30*time.Second {
		window = 30 * time.Second
	}
	if window > 12*time.Hour {
		window = 12 * time.Hour
	}
	if half := d.JWTTTL / 2; window > half {
		window = half
	}
	return window
}

// maybeRenewToken 剩余有效期不足时换发新 token，通过响应头下发。
// 续期失败静默忽略：本次请求继续用旧 token，到期后前端照常跳登录页。
func (d *Data) maybeRenewToken(c *gin.Context, user User, claims *Claims) {
	if claims == nil || claims.ExpiresAt == nil {
		return
	}
	if time.Until(claims.ExpiresAt.Time) > d.renewWindow() {
		return
	}

	token, expiresAt, err := d.issueToken(user)
	if err != nil {
		return
	}

	c.Header(headerAuthToken, token)
	c.Header(headerAuthExpiresAt, expiresAt.UTC().Format(time.RFC3339))
}

// AuthRequired 校验 token，未登录或登录失效时中断请求。
func (d *Data) AuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		user, claims, failure := d.authenticate(c)
		if failure != nil {
			Response{}.Fail(c, failure.status, failure.code, failure.message)
			c.Abort()
			return
		}

		d.maybeRenewToken(c, user, claims)
		c.Set(ctxUserKey, user)
		c.Next()
	}
}

// OptionalAuth 有合法 token 时填充当前用户，否则按游客继续。
// 仅用于公开模式下的只读接口；写接口仍由 AuthRequired 兜底。
func (d *Data) OptionalAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		if user, claims, failure := d.authenticate(c); failure == nil {
			d.maybeRenewToken(c, user, claims)
			c.Set(ctxUserKey, user)
		}
		c.Next()
	}
}

// ReadAuth 只读接口的访问控制：隐私模式必须登录，公开模式允许游客。
func (d *Data) ReadAuth() gin.HandlerFunc {
	if d.IsPublic() {
		return d.OptionalAuth()
	}
	return d.AuthRequired()
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

// 请求体上限，避免 ShouldBindJSON 把超大 JSON 全读进内存。
const defaultMaxRequestBodyBytes int64 = 1 << 20

// bodyLimit 用 MaxBytesReader 限制请求体大小，超出时由 respondBindError 映射为 413。
func bodyLimit(maxBytes int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Body != nil {
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
		}
		c.Next()
	}
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
