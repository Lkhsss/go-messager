package main

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// AdminUserListItem 是管理员用户列表里的单行数据。
// 统计口径与 /api/me/stats 一致：文章数只算原文，互动只算原文收到的反应。
type AdminUserListItem struct {
	ID            uint       `json:"id"`
	Name          string     `json:"name"`
	Role          Role       `json:"role"`
	AvatarVersion string     `json:"avatar_version,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	LastLoginAt   *time.Time `json:"last_login_at,omitempty"`
	LastActiveAt  *time.Time `json:"last_active_at,omitempty"`
	LoginCount    int64      `json:"login_count"`
	PostCount     int64      `json:"post_count"`
	CommentCount  int64      `json:"comment_count"`
	WordCount     int64      `json:"word_count"`
	Like          int64      `json:"like"`
	Dislike       int64      `json:"dislike"`
	Question      int64      `json:"question"`
}

// AdminUserDetailResponse 用户详情 + 与个人数据页相同的统计。
type AdminUserDetailResponse struct {
	User  AdminUserListItem `json:"user"`
	Stats UserStatsResponse `json:"stats"`
}

type adminUserStatusInput struct {
	Disabled bool `json:"disabled"`
}

type adminUserPasswordInput struct {
	Password string `json:"password"`
}

// GetAdminUsers 用户列表 GET /api/admin/users?page=1&per_page=20&q=name
func (d *Data) GetAdminUsers(c *gin.Context) {
	page, perPage := parsePagination(c)
	keyword := strings.TrimSpace(c.Query("q"))

	base := func() *gorm.DB {
		db := d.DB.Model(&User{})
		if keyword != "" {
			db = db.Where("name LIKE ?", "%"+keyword+"%")
		}
		return db
	}

	var total int64
	if err := base().Count(&total).Error; err != nil {
		Response{}.Fail(c, http.StatusInternalServerError, "COUNT_FAILED", err.Error())
		return
	}

	var users []User
	if err := base().
		Order("created_at ASC, id ASC").
		Limit(perPage).
		Offset((page - 1) * perPage).
		Find(&users).Error; err != nil {
		Response{}.Fail(c, http.StatusInternalServerError, "QUERY_FAILED", err.Error())
		return
	}

	items, err := d.adminUserItems(users)
	if err != nil {
		Response{}.Fail(c, http.StatusInternalServerError, "QUERY_FAILED", err.Error())
		return
	}

	totalPages := 0
	if total > 0 {
		totalPages = int((total + int64(perPage) - 1) / int64(perPage))
	}

	Response{}.OkWithMeta(c, http.StatusOK, items, &Meta{
		Page:       page,
		PerPage:    perPage,
		Total:      int(total),
		TotalPages: totalPages,
	})
}

// GetAdminUserDetail 用户详情 GET /api/admin/users/:id
// stats 与 GET /api/me/stats 使用同一个 loadUserStats，避免两处统计口径漂移。
func (d *Data) GetAdminUserDetail(c *gin.Context) {
	id, ok := parseAdminUserID(c)
	if !ok {
		return
	}

	user, err := d.GetUserByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			Response{}.Fail(c, http.StatusNotFound, "USER_NOT_FOUND", "用户不存在")
			return
		}
		Response{}.Fail(c, http.StatusInternalServerError, "QUERY_FAILED", err.Error())
		return
	}

	items, err := d.adminUserItems([]User{user})
	if err != nil {
		Response{}.Fail(c, http.StatusInternalServerError, "QUERY_FAILED", err.Error())
		return
	}
	stats, err := d.loadUserStats(user.Name)
	if err != nil {
		Response{}.Fail(c, http.StatusInternalServerError, "QUERY_FAILED", err.Error())
		return
	}

	Response{}.Ok(c, http.StatusOK, AdminUserDetailResponse{
		User:  items[0],
		Stats: stats,
	})
}

// SetAdminUserStatus 封禁/解封 PATCH /api/admin/users/:id/status
// 封禁使用 RoleNone，AuthRequired 每次都会从数据库读角色，因此已登录令牌会立即失效。
func (d *Data) SetAdminUserStatus(c *gin.Context) {
	current, ok := CurrentUser(c)
	if !ok {
		Response{}.Fail(c, http.StatusUnauthorized, "UNAUTHORIZED", "请先登录")
		return
	}

	id, ok := parseAdminUserID(c)
	if !ok {
		return
	}

	var req adminUserStatusInput
	if err := c.ShouldBindJSON(&req); err != nil {
		Response{}.Fail(c, http.StatusBadRequest, "INVALID_BODY", err.Error())
		return
	}

	target := User{}
	if err := d.DB.Take(&target, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			Response{}.Fail(c, http.StatusNotFound, "USER_NOT_FOUND", "用户不存在")
			return
		}
		Response{}.Fail(c, http.StatusInternalServerError, "QUERY_FAILED", err.Error())
		return
	}

	if target.ID == current.ID || target.Role == RoleAdmin {
		Response{}.Fail(c, http.StatusBadRequest, "CANNOT_MODIFY_ADMIN", "不能封禁管理员")
		return
	}

	role := RoleUser
	if req.Disabled {
		role = RoleNone
	}
	if err := d.DB.Model(&target).Update("role", role).Error; err != nil {
		Response{}.Fail(c, http.StatusInternalServerError, "UPDATE_FAILED", err.Error())
		return
	}
	target.Role = role

	items, err := d.adminUserItems([]User{target})
	if err != nil {
		Response{}.Fail(c, http.StatusInternalServerError, "QUERY_FAILED", err.Error())
		return
	}
	Response{}.Ok(c, http.StatusOK, items[0])
}

// SetAdminUserPassword 管理员修改用户密码 PUT /api/admin/users/:id/password
func (d *Data) SetAdminUserPassword(c *gin.Context) {
	id, ok := parseAdminUserID(c)
	if !ok {
		return
	}

	var req adminUserPasswordInput
	if err := c.ShouldBindJSON(&req); err != nil {
		Response{}.Fail(c, http.StatusBadRequest, "INVALID_BODY", err.Error())
		return
	}
	if req.Password == "" {
		Response{}.Fail(c, http.StatusBadRequest, "PASSWORD_REQUIRED", "密码不能为空")
		return
	}
	// bcrypt 只接受最多 72 字节，提前返回可读错误而不是 500。
	if len([]byte(req.Password)) > 72 {
		Response{}.Fail(c, http.StatusBadRequest, "PASSWORD_TOO_LONG", "密码最多 72 字节")
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		Response{}.Fail(c, http.StatusInternalServerError, "HASH_FAILED", err.Error())
		return
	}

	result := d.DB.Model(&User{}).Where("id = ?", id).Update("password", string(hash))
	if result.Error != nil {
		Response{}.Fail(c, http.StatusInternalServerError, "UPDATE_FAILED", result.Error.Error())
		return
	}
	if result.RowsAffected == 0 {
		Response{}.Fail(c, http.StatusNotFound, "USER_NOT_FOUND", "用户不存在")
		return
	}

	Response{}.Ok(c, http.StatusOK, gin.H{"id": id})
}

// DeleteAdminUser 删除用户 DELETE /api/admin/users/:id
// 只软删除账号，保留其文章与评论，避免误删社区内容；账号删除后不能再登录。
func (d *Data) DeleteAdminUser(c *gin.Context) {
	current, ok := CurrentUser(c)
	if !ok {
		Response{}.Fail(c, http.StatusUnauthorized, "UNAUTHORIZED", "请先登录")
		return
	}

	id, ok := parseAdminUserID(c)
	if !ok {
		return
	}

	target := User{}
	if err := d.DB.Take(&target, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			Response{}.Fail(c, http.StatusNotFound, "USER_NOT_FOUND", "用户不存在")
			return
		}
		Response{}.Fail(c, http.StatusInternalServerError, "QUERY_FAILED", err.Error())
		return
	}

	if target.ID == current.ID || target.Role == RoleAdmin {
		Response{}.Fail(c, http.StatusBadRequest, "CANNOT_DELETE_ADMIN", "不能删除管理员")
		return
	}

	if err := d.DB.Delete(&target).Error; err != nil {
		Response{}.Fail(c, http.StatusInternalServerError, "DELETE_FAILED", err.Error())
		return
	}

	Response{}.Ok(c, http.StatusOK, gin.H{"id": id})
}

func parseAdminUserID(c *gin.Context) (uint, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil || id == 0 {
		Response{}.Fail(c, http.StatusBadRequest, "INVALID_ID", "用户 ID 无效")
		return 0, false
	}
	return uint(id), true
}

// adminUserItems 批量填充用户列表统计，避免每行单独查询。
func (d *Data) adminUserItems(users []User) ([]AdminUserListItem, error) {
	items := make([]AdminUserListItem, len(users))
	if len(users) == 0 {
		return items, nil
	}

	names := make([]string, 0, len(users))
	ids := make([]uint, 0, len(users))
	for _, user := range users {
		names = append(names, user.Name)
		ids = append(ids, user.ID)
	}

	type postAggregate struct {
		User          string
		PostCount     int64
		CommentCount  int64
		WordCount     int64
		LikeTotal     int64
		DislikeTotal  int64
		QuestionTotal int64
	}
	var postAggregates []postAggregate
	if err := d.DB.Model(&Post{}).
		Select(`user,
			COALESCE(SUM(CASE WHEN root_id IS NULL THEN 1 ELSE 0 END), 0) AS post_count,
			COALESCE(SUM(CASE WHEN root_id IS NOT NULL THEN 1 ELSE 0 END), 0) AS comment_count,
			COALESCE(SUM(CASE WHEN root_id IS NULL THEN char_count ELSE 0 END), 0) AS word_count,
			COALESCE(SUM(CASE WHEN root_id IS NULL THEN like_count ELSE 0 END), 0) AS like_total,
			COALESCE(SUM(CASE WHEN root_id IS NULL THEN dislike_count ELSE 0 END), 0) AS dislike_total,
			COALESCE(SUM(CASE WHEN root_id IS NULL THEN question_count ELSE 0 END), 0) AS question_total`).
		Where("user IN ?", names).
		Group("user").
		Scan(&postAggregates).Error; err != nil {
		return nil, err
	}

	type latestPost struct {
		User         string
		LastActivity string
	}
	var latestPosts []latestPost
	if err := d.DB.Model(&Post{}).
		Select("user, MAX(created_at) AS last_activity").
		Where("user IN ?", names).
		Group("user").
		Scan(&latestPosts).Error; err != nil {
		return nil, err
	}

	type latestReaction struct {
		UserID       uint
		LastActivity string
	}
	var latestReactions []latestReaction
	if err := d.DB.Model(&Reaction{}).
		Select("user_id, MAX(created_at) AS last_activity").
		Where("user_id IN ?", ids).
		Group("user_id").
		Scan(&latestReactions).Error; err != nil {
		return nil, err
	}

	postByUser := make(map[string]postAggregate, len(postAggregates))
	for _, aggregate := range postAggregates {
		postByUser[aggregate.User] = aggregate
	}
	postActivity := make(map[string]time.Time, len(latestPosts))
	for _, latest := range latestPosts {
		if value, ok := parseSQLiteTime(latest.LastActivity); ok {
			postActivity[latest.User] = value
		}
	}
	reactionActivity := make(map[uint]time.Time, len(latestReactions))
	for _, latest := range latestReactions {
		if value, ok := parseSQLiteTime(latest.LastActivity); ok {
			reactionActivity[latest.UserID] = value
		}
	}

	for i, user := range users {
		item := AdminUserListItem{
			ID:            user.ID,
			Name:          user.Name,
			Role:          user.Role,
			AvatarVersion: avatarVersion(user.Avatar),
			CreatedAt:     user.CreatedAt,
			LastLoginAt:   user.LastLoginAt,
			LoginCount:    user.LoginCount,
		}
		if aggregate, ok := postByUser[user.Name]; ok {
			item.PostCount = aggregate.PostCount
			item.CommentCount = aggregate.CommentCount
			item.WordCount = aggregate.WordCount
			item.Like = aggregate.LikeTotal
			item.Dislike = aggregate.DislikeTotal
			item.Question = aggregate.QuestionTotal
		}

		// 注册本身也算一次活动；没有任何后续操作时显示注册时间，而不是“从未”。
		lastActive := user.CreatedAt
		mergeActivity := func(value time.Time) {
			if value.After(lastActive) {
				lastActive = value
			}
		}
		if user.LastLoginAt != nil {
			mergeActivity(*user.LastLoginAt)
		}
		if value, ok := postActivity[user.Name]; ok {
			mergeActivity(value)
		}
		if value, ok := reactionActivity[user.ID]; ok {
			mergeActivity(value)
		}
		if !lastActive.IsZero() {
			value := lastActive
			item.LastActiveAt = &value
		}

		items[i] = item
	}

	return items, nil
}

// parseSQLiteTime 解析 MAX(created_at) 聚合后返回的字符串。
// SQLite 的聚合列会丢掉 time.Time 类型信息，需要兼容常见的时间格式。
func parseSQLiteTime(raw string) (time.Time, bool) {
	if raw == "" {
		return time.Time{}, false
	}

	layouts := []string{
		time.RFC3339Nano,
		"2006-01-02 15:04:05.999999999-07:00",
		"2006-01-02 15:04:05.999999999Z07:00",
		"2006-01-02 15:04:05-07:00",
		"2006-01-02 15:04:05Z07:00",
		"2006-01-02 15:04:05",
	}
	for _, layout := range layouts {
		if value, err := time.Parse(layout, raw); err == nil {
			return value, true
		}
	}
	return time.Time{}, false
}
