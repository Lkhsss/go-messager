package main

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// 登录限流参数。
const (
	// 每个 IP 每分钟的登录尝试上限（成功与失败都计数）。
	loginIPPerMinute = 20
	// 同一 IP + 用户名的失败次数上限与统计窗口。
	loginFailureLimit  = 5
	loginIPWindow      = time.Minute
	loginFailureWindow = 15 * time.Minute
	// 同一用户名跨 IP 的失败上限，防止分布式爆破；成功登录清零。
	loginAccountFailureLimit = 30
)

// dummyPasswordHash 用于不存在用户时仍执行一次 bcrypt，避免通过响应时间枚举账号。
var dummyPasswordHash = func() []byte {
	hash, err := bcrypt.GenerateFromPassword([]byte("go-messager-dummy-password"), bcrypt.DefaultCost)
	if err != nil {
		return nil
	}
	return hash
}()

// validateUsername 校验用户名长度与字符。
func validateUsername(name string, limits LimitsConfig) (string, string) {
	runes := []rune(name)
	if len(runes) < limits.MinUsernameRunes || len(runes) > limits.MaxUsernameRunes {
		return "INVALID_USERNAME", "用户名长度需要在 " + strconv.Itoa(limits.MinUsernameRunes) + "-" + strconv.Itoa(limits.MaxUsernameRunes) + " 个字符之间"
	}
	for _, r := range runes {
		if unicode.IsSpace(r) || unicode.IsControl(r) {
			return "INVALID_USERNAME", "用户名不能包含空白或控制字符"
		}
	}
	return "", ""
}

// validatePassword 校验密码字节长度；bcrypt 最多接受 72 字节。
func validatePassword(password string, limits LimitsConfig) (string, string) {
	size := len([]byte(password))
	if size < limits.MinPasswordBytes {
		return "WEAK_PASSWORD", "密码至少需要 " + strconv.Itoa(limits.MinPasswordBytes) + " 个字节"
	}
	if size > 72 {
		return "PASSWORD_TOO_LONG", "密码最多 72 个字节"
	}
	return "", ""
}

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
		respondBindError(c, err, "INVALID_BODY")
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" || req.Password == "" {
		Response{}.Fail(c, http.StatusBadRequest, "NEED_CREDENTIALS", "用户名和密码不能为空")
		return
	}

	// 被爆破时先限流，避免产生无意义的数据库查询和 bcrypt 计算。
	clientIP := c.ClientIP()
	failKey := loginFailureKey(clientIP, req.Name)
	accountKey := strings.ToLower(req.Name)
	if !d.loginIPLimiter.Allow(clientIP, loginIPPerMinute) {
		c.Header("Retry-After", strconv.Itoa(int(loginIPWindow/time.Second)))
		Response{}.Fail(c, http.StatusTooManyRequests, "LOGIN_RATE_LIMITED", "登录尝试过于频繁，请稍后再试")
		return
	}
	if d.loginFailLimiter.Exceeded(failKey, loginFailureLimit) ||
		d.loginAccountLimiter.Exceeded(accountKey, loginAccountFailureLimit) {
		c.Header("Retry-After", strconv.Itoa(int(loginFailureWindow/time.Second)))
		Response{}.Fail(c, http.StatusTooManyRequests, "LOGIN_LOCKED", "登录失败次数过多，请稍后再试")
		return
	}

	user, err := d.GetUserByName(req.Name)
	if err != nil {
		// 不暴露账号是否存在：不存在时也做一次 bcrypt，再返回统一错误。
		if errors.Is(err, gorm.ErrRecordNotFound) {
			if len(dummyPasswordHash) > 0 {
				_ = bcrypt.CompareHashAndPassword(dummyPasswordHash, []byte(req.Password))
			}
			d.loginFailLimiter.Allow(failKey, loginFailureLimit)
			d.loginAccountLimiter.Allow(accountKey, loginAccountFailureLimit)
			Response{}.Fail(c, http.StatusUnauthorized, "INVALID_CREDENTIALS", "用户名或密码错误")
			return
		}
		Response{}.Fail(c, http.StatusInternalServerError, "QUERY_FAILED", err.Error())
		return
	}

	// 先校验密码再判断停用，避免用 403 探测用户。
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		d.loginFailLimiter.Allow(failKey, loginFailureLimit)
		d.loginAccountLimiter.Allow(accountKey, loginAccountFailureLimit)
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

	// 登录成功清零失败计数。
	d.loginFailLimiter.Reset(failKey)
	d.loginAccountLimiter.Reset(accountKey)

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

// Register 注册；首个用户自动成为 admin，且免人机验证。
func (d *Data) Register(c *gin.Context) {
	var req RegisterInput
	if err := c.ShouldBind(&req); err != nil {
		respondBindError(c, err, "INVALID_BODY")
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" || req.Password == "" {
		Response{}.Fail(c, http.StatusBadRequest, "NEED_CREDENTIALS", "用户名和密码不能为空")
		return
	}
	if code, message := validateUsername(req.Name, d.limits); code != "" {
		Response{}.Fail(c, http.StatusBadRequest, code, message)
		return
	}
	if code, message := validatePassword(req.Password, d.limits); code != "" {
		Response{}.Fail(c, http.StatusBadRequest, code, message)
		return
	}

	// 用户总数同时用于维护模式和首用户判断，只查一次。
	var userCount int64
	if err := d.DB.Model(&User{}).Count(&userCount).Error; err != nil {
		Response{}.Fail(c, http.StatusInternalServerError, "COUNT_FAILED", err.Error())
		return
	}

	// 维护模式下停止注册；全新库没有用户时放行，避免无法初始化管理员。
	if state := d.MaintenanceState(); state.Enabled && userCount > 0 {
		Response{}.Fail(c, http.StatusServiceUnavailable, "MAINTENANCE", state.Title)
		return
	}

	// 关闭注册后同样保留首个管理员初始化，其余账号一律拒绝。
	if userCount > 0 && !d.RegisterEnabled {
		Response{}.Fail(c, http.StatusForbidden, "REGISTER_DISABLED", "注册功能已关闭")
		return
	}

	// 每个 IP 的成功注册次数限制；首个管理员初始化不受限。
	clientIP := c.ClientIP()
	if userCount > 0 &&
		(d.registerHourLimiter.Exceeded(clientIP, d.limits.RegisterPerIPPerHour) ||
			d.registerDayLimiter.Exceeded(clientIP, d.limits.RegisterPerIPPerDay)) {
		c.Header("Retry-After", strconv.Itoa(int(time.Hour/time.Second)))
		Response{}.Fail(c, http.StatusTooManyRequests, "REGISTER_RATE_LIMITED", "注册过于频繁，请稍后再试")
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

	// 非首个用户必须带人机验证签发的 ticket；ticket 一次性，用后即作废。
	if d.CaptchaEnabled && userCount > 0 && !d.captcha.ConsumeTicket(clientIP, req.CaptchaTicket) {
		Response{}.Fail(c, http.StatusBadRequest, "CAPTCHA_REQUIRED", "请先完成人机验证")
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

	// 只统计成功注册，避免验证码输错也消耗 IP 配额。
	if userCount > 0 {
		d.registerHourLimiter.Allow(clientIP, d.limits.RegisterPerIPPerHour)
		d.registerDayLimiter.Allow(clientIP, d.limits.RegisterPerIPPerDay)
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

// loginFailureKey 失败次数按 IP + 用户名（忽略大小写）隔离。
func loginFailureKey(ip, name string) string {
	return ip + "|" + strings.ToLower(name)
}
