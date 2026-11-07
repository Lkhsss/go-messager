package main

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"
)

// JWT 相关配置
const (
	jwtSecretSettingKey = "jwt_secret"
	jwtSecretBytes      = 32
	defaultJWTTTL       = 24 * time.Hour
)

type Data struct {
	DB        *gorm.DB
	JWTSecret []byte
	JWTTTL    time.Duration

	// mu 保护「第一个用户成为 admin」的计数与插入。
	// 当前是单实例部署，进程内互斥足够；多实例部署时应改成数据库约束。
	mu sync.Mutex
}

// NewData 初始化数据层：确定 JWT 密钥与有效期。
func NewData(db *gorm.DB, cfg JWTConfig) (*Data, error) {
	ttl, err := resolveJWTTTL(cfg.TTL)
	if err != nil {
		return nil, err
	}

	d := &Data{DB: db, JWTTTL: ttl}

	secret, err := ensureJWTSecret(db, cfg.Secret)
	if err != nil {
		return nil, err
	}
	d.JWTSecret = secret

	return d, nil
}

// resolveJWTTTL 按优先级确定 JWT 有效期：
//  1. JWT_TTL 环境变量（部署时推荐，优先级最高）
//  2. conf/app.ini 的 [jwt] ttl
//  3. 默认 24h
func resolveJWTTTL(configured time.Duration) (time.Duration, error) {
	if raw := strings.TrimSpace(os.Getenv("JWT_TTL")); raw != "" {
		ttl, err := time.ParseDuration(raw)
		if err != nil || ttl <= 0 {
			return 0, fmt.Errorf("JWT_TTL 配置无效[%s]：需要正的时间段，例如 24h", raw)
		}
		return ttl, nil
	}

	if configured > 0 {
		return configured, nil
	}

	return defaultJWTTTL, nil
}

// ensureJWTSecret 按优先级取密钥：
//  1. JWT_SECRET 环境变量（部署时推荐）
//  2. conf/app.ini 的 [jwt] secret
//  3. settings 表里的 jwt_secret（首次启动自动生成并持久化，重启后 token 仍有效）
func ensureJWTSecret(db *gorm.DB, configured string) ([]byte, error) {
	if secret := strings.TrimSpace(os.Getenv("JWT_SECRET")); secret != "" {
		return []byte(secret), nil
	}
	if secret := strings.TrimSpace(configured); secret != "" {
		return []byte(secret), nil
	}

	var setting Setting
	err := db.Where("key = ?", jwtSecretSettingKey).Take(&setting).Error
	switch {
	case err == nil:
		return []byte(setting.Value), nil
	case !errors.Is(err, gorm.ErrRecordNotFound):
		return nil, fmt.Errorf("读取 JWT 密钥失败：%w", err)
	}

	raw := make([]byte, jwtSecretBytes)
	if _, err := rand.Read(raw); err != nil {
		return nil, fmt.Errorf("生成 JWT 密钥失败：%w", err)
	}
	secret := base64.RawURLEncoding.EncodeToString(raw)

	if err := db.Create(&Setting{Key: jwtSecretSettingKey, Value: secret}).Error; err != nil {
		// 并发启动时可能已被另一个进程写入，重新读一次
		var existing Setting
		if readErr := db.Where("key = ?", jwtSecretSettingKey).Take(&existing).Error; readErr == nil {
			return []byte(existing.Value), nil
		}
		return nil, fmt.Errorf("保存 JWT 密钥失败：%w", err)
	}

	return []byte(secret), nil
}

func (d *Data) GetUserByName(name string) (User, error) {
	var user User
	// 用 Take 而不是 Find：查不到时会返回 gorm.ErrRecordNotFound，便于区分「用户不存在」和「数据库故障」
	if err := d.DB.Where("name = ?", name).Take(&user).Error; err != nil {
		return User{}, fmt.Errorf("查询用户[%s]失败：%w", name, err)
	}

	return user, nil
}

func (d *Data) GetUserByID(id uint) (User, error) {
	var user User
	if err := d.DB.Take(&user, id).Error; err != nil {
		return User{}, fmt.Errorf("查询用户[%d]失败：%w", id, err)
	}

	return user, nil
}

func (d *Data) IfUserExist(name string) (bool, error) {
	var count int64
	// 包含已软删除账号：删除账号后保留其文章，用户名也不能被新账号复用。
	err := d.DB.Unscoped().Model(&User{}).Where("name = ?", name).Count(&count).Error
	if err != nil {
		return false, fmt.Errorf("查询用户名[%w]错误", err)
	}

	exists := count > 0
	return exists, nil
}

func (d *Data) IfTagExist(name string) (bool, error) {
	var count int64
	if err := d.DB.Model(&Tag{}).Where("name = ?", name).Count(&count).Error; err != nil {
		return false, fmt.Errorf("查询标签[%s]失败：%w", name, err)
	}

	return count > 0, nil
}

// syncTagUsageCounts 按 post_tags 全量校准标签使用次数（排除软删除的文章）。
// 启动时执行一次，幂等；以后新增/删除文章接口若漏了维护计数，重启即可自愈。
func syncTagUsageCounts(db *gorm.DB) error {
	err := db.Exec(`
		UPDATE tags SET usage_count = (
			SELECT COUNT(*) FROM post_tags
			JOIN posts ON posts.id = post_tags.post_id AND posts.deleted_at IS NULL
			WHERE post_tags.tag_id = tags.id
		)`).Error
	if err != nil {
		return fmt.Errorf("校准标签使用次数失败：%w", err)
	}

	return nil
}

// CreateUser 创建用户并决定角色：库里还没有用户时，第一个用户是 admin。
// 计数与插入放在同一个事务里，并用 mu 串行化，避免两个并发注册都成为 admin。
func (d *Data) CreateUser(name string, hash string) (User, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	var user User
	err := d.DB.Transaction(func(tx *gorm.DB) error {
		var count int64
		if err := tx.Model(&User{}).Count(&count).Error; err != nil {
			return err
		}

		user = User{Name: name, Password: hash, Role: RoleUser}
		if count == 0 {
			user.Role = RoleAdmin
		}

		return tx.Create(&user).Error
	})
	if err != nil {
		return User{}, fmt.Errorf("创建用户[%s]出错：%w", name, err)
	}

	return user, nil
}

// UpdateUserAvatar 更新头像 data URL（空字符串表示清除）。
func (d *Data) UpdateUserAvatar(id uint, avatar string) error {
	if err := d.DB.Model(&User{}).Where("id = ?", id).Update("avatar", avatar).Error; err != nil {
		return fmt.Errorf("更新用户[%d]头像失败：%w", id, err)
	}

	return nil
}
