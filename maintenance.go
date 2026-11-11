package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	// 维护模式状态存放在 settings 表的 key。
	maintenanceSettingKey = "maintenance"
	// 未填写标题时的默认文案。
	maintenanceDefaultTitle = "站点维护中"
	// 标题和正文的长度上限（按字符数）。
	maintenanceTitleMaxRune = 60
	maintenanceBodyMaxRune  = 2000
)

// MaintenanceState 维护模式状态，整体以 JSON 存进 settings 表。
type MaintenanceState struct {
	Enabled bool   `json:"enabled"`
	Title   string `json:"title"`
	Body    string `json:"body"`
}

// normalized 裁剪空白并补齐默认标题。
func (s MaintenanceState) normalized() MaintenanceState {
	s.Title = strings.TrimSpace(s.Title)
	s.Body = strings.TrimSpace(s.Body)
	if s.Title == "" {
		s.Title = maintenanceDefaultTitle
	}

	return s
}

// loadMaintenance 读取维护状态，记录不存在时回落为关闭状态。
func loadMaintenance(db *gorm.DB) (MaintenanceState, error) {
	fallback := MaintenanceState{Title: maintenanceDefaultTitle}

	var setting Setting
	err := db.Where("key = ?", maintenanceSettingKey).Take(&setting).Error
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		return fallback, nil
	case err != nil:
		return fallback, fmt.Errorf("读取维护模式配置失败：%w", err)
	}

	var state MaintenanceState
	if err := json.Unmarshal([]byte(setting.Value), &state); err != nil {
		return fallback, fmt.Errorf("解析维护模式配置失败：%w", err)
	}

	return state.normalized(), nil
}

// saveMaintenance 写入 settings 表并更新内存缓存。
func (d *Data) saveMaintenance(state MaintenanceState) error {
	state = state.normalized()

	raw, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("序列化维护模式配置失败：%w", err)
	}

	if err := d.DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "key"}},
		DoUpdates: clause.AssignmentColumns([]string{"value", "updated_at"}),
	}).Create(&Setting{Key: maintenanceSettingKey, Value: string(raw)}).Error; err != nil {
		return fmt.Errorf("保存维护模式配置失败：%w", err)
	}

	d.maintenanceMu.Lock()
	d.maintenance = state
	d.maintenanceMu.Unlock()

	return nil
}

// loadMaintenanceState 启动时把维护状态读进内存缓存。
func (d *Data) loadMaintenanceState() error {
	state, err := loadMaintenance(d.DB)

	d.maintenanceMu.Lock()
	d.maintenance = state
	d.maintenanceMu.Unlock()

	return err
}

// MaintenanceState 返回当前维护状态快照。
func (d *Data) MaintenanceState() MaintenanceState {
	d.maintenanceMu.RLock()
	defer d.maintenanceMu.RUnlock()

	return d.maintenance
}

// IsMaintenance 是否处于维护模式。
func (d *Data) IsMaintenance() bool {
	return d.MaintenanceState().Enabled
}

// maintenanceAllowlist 维护期间仍需放行的接口：
// 状态查询给维护页和前端守卫、运行时长给容器健康检查、登录给管理员恢复现场。
// 注册也放行到 handler 里判断：全新库没有用户时允许初始化首个管理员。
func maintenanceAllowlist(method, path string) bool {
	switch path {
	case "/api/status", "/api/uptime":
		return method == http.MethodGet
	case "/api/authorize", "/api/register":
		return method == http.MethodPost
	}

	return false
}

// MaintenanceGuard 维护模式下只放行白名单和管理员，其余请求一律 503。
func (d *Data) MaintenanceGuard() gin.HandlerFunc {
	return func(c *gin.Context) {
		state := d.MaintenanceState()
		if !state.Enabled {
			c.Next()
			return
		}

		if maintenanceAllowlist(c.Request.Method, c.Request.URL.Path) || d.isAdminRequest(c) {
			c.Next()
			return
		}

		Response{}.Fail(c, http.StatusServiceUnavailable, "MAINTENANCE", state.Title)
		c.Abort()
	}
}

// isAdminRequest 判断请求是否携带有效的管理员 token。
func (d *Data) isAdminRequest(c *gin.Context) bool {
	raw, ok := bearerToken(c.GetHeader("Authorization"))
	if !ok {
		return false
	}

	claims, err := d.parseToken(raw)
	if err != nil {
		return false
	}

	id, err := userIDFromClaims(claims)
	if err != nil {
		return false
	}

	user, err := d.GetUserByID(id)
	if err != nil {
		return false
	}

	return user.Role == RoleAdmin
}
