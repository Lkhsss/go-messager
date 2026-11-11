package main

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// AuthResult 登录/注册结果。
type AuthResult struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
	User      UserView  `json:"user"`
	// 每日登录经验，重复登录为 nil。
	XP *XPGain `json:"xp,omitempty"`
}

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

	user, err := d.GetUserByName(req.Name)
	if err != nil {
		// 不暴露账号是否存在。
		if errors.Is(err, gorm.ErrRecordNotFound) {
			Response{}.Fail(c, http.StatusUnauthorized, "INVALID_CREDENTIALS", "用户名或密码错误")
			return
		}
		Response{}.Fail(c, http.StatusInternalServerError, "QUERY_FAILED", err.Error())
		return
	}

	// 先校验密码再判断停用，避免用 403 探测用户。
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		Response{}.Fail(c, http.StatusUnauthorized, "INVALID_CREDENTIALS", "用户名或密码错误")
		return
	}

	if user.Role == RoleNone {
		Response{}.Fail(c, http.StatusForbidden, "ACCOUNT_DISABLED", "账号已被停用")
		return
	}

	// 维护模式下只允许管理员登录，方便管理员恢复站点。
	if state := d.MaintenanceState(); state.Enabled && user.Role != RoleAdmin {
		Response{}.Fail(c, http.StatusServiceUnavailable, "MAINTENANCE", state.Title)
		return
	}

	token, expiresAt, err := d.issueToken(user)
	if err != nil {
		Response{}.Fail(c, http.StatusInternalServerError, "TOKEN_FAILED", err.Error())
		return
	}

	// 统计失败不影响登录。
	if err := d.DB.Model(&user).Updates(map[string]any{
		"login_count":   gorm.Expr("login_count + 1"),
		"last_login_at": time.Now(),
	}).Error; err != nil {
		// 统计更新失败时仍允许登录
	}

	result := AuthResult{Token: token, ExpiresAt: expiresAt, User: user.View()}
	// 每日登录经验，发放失败不影响登录。
	if gain, xpErr := d.awardDailyTask(user.ID, DailyTaskLogin, DailyLoginXP, time.Now()); xpErr == nil && gain != nil {
		user.Exp = gain.Exp
		result.User = user.View()
		result.XP = gain
	}

	Response{}.Ok(c, http.StatusOK, result)
}

// Register 首个用户自动成为 admin。
func (d *Data) Register(c *gin.Context) {
	var req RegisterInput
	if err := c.ShouldBind(&req); err != nil {
		Response{}.Fail(c, http.StatusBadRequest, "INVALID_BODY", err.Error())
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" || req.Password == "" {
		Response{}.Fail(c, http.StatusBadRequest, "NEED_CREDENTIALS", "用户名和密码不能为空")
		return
	}

	// 维护模式下停止注册；全新库没有用户时放行，避免无法初始化管理员。
	if state := d.MaintenanceState(); state.Enabled {
		var count int64
		if err := d.DB.Model(&User{}).Count(&count).Error; err != nil {
			Response{}.Fail(c, http.StatusInternalServerError, "COUNT_FAILED", err.Error())
			return
		}
		if count > 0 {
			Response{}.Fail(c, http.StatusServiceUnavailable, "MAINTENANCE", state.Title)
			return
		}
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

	user, err := d.CreateUser(req.Name, string(hashedPassword))
	if err != nil {
		Response{}.Fail(c, http.StatusInternalServerError, "CREATE_FAILED", err.Error())
		return
	}

	// 注册后直接签发 token。
	token, expiresAt, err := d.issueToken(user)
	if err != nil {
		Response{}.Fail(c, http.StatusInternalServerError, "TOKEN_FAILED", err.Error())
		return
	}

	result := AuthResult{Token: token, ExpiresAt: expiresAt, User: user.View()}
	// 注册即登录，同样计每日登录经验。
	if gain, xpErr := d.awardDailyTask(user.ID, DailyTaskLogin, DailyLoginXP, time.Now()); xpErr == nil && gain != nil {
		user.Exp = gain.Exp
		result.User = user.View()
		result.XP = gain
	}

	Response{}.Ok(c, http.StatusCreated, result)
}

// Me 当前用户，前端用来校验本地 token；跨天保持登录时顺带补发每日登录经验。
func (d *Data) Me(c *gin.Context) {
	user, ok := CurrentUser(c)
	if !ok {
		Response{}.Fail(c, http.StatusUnauthorized, "UNAUTHORIZED", "请先登录")
		return
	}

	// 没退出、token 也没过期的情况下跨天访问，同样算每日登录；发放失败不影响响应。
	now := time.Now()
	if pending, err := d.dailyLoginPending(user.ID, dailyTaskDate(now)); err == nil && pending {
		if gain, err := d.awardDailyTask(user.ID, DailyTaskLogin, DailyLoginXP, now); err == nil && gain != nil {
			user.Exp = gain.Exp
		}
	}

	Response{}.Ok(c, http.StatusOK, user.View())
}
