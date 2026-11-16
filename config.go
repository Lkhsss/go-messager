package main

import (
	"errors"
	"fmt"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gopkg.in/ini.v1"
)

const (
	// 默认配置路径。
	defaultConfigPath = "conf/app.ini"
	// 默认端口。
	defaultServerPort = 8080
	// 默认运行模式。
	defaultServerMode = gin.ReleaseMode

	// 站点访问模式：所有页面和操作必须登录。
	AccessModePrivate = "private"
	// 站点访问模式：游客可浏览文章，写操作需要登录。
	AccessModePublic = "public"
	// 默认访问模式。
	defaultAccessMode = AccessModePrivate
)

// Config 应用配置。
type Config struct {
	Server   ServerConfig
	Access   AccessConfig
	Database DatabaseConfig
	JWT      JWTConfig
	Captcha  CaptchaConfig
	Register RegisterConfig
	Limits   LimitsConfig
}

// ServerConfig HTTP 配置。
type ServerConfig struct {
	// 留空时 debug 监听 127.0.0.1，其他模式监听 0.0.0.0。
	Host string
	Port int
	// debug/release/test，留空默认 release；GIN_MODE 优先。
	Mode string
	// 反向代理地址或网段，逗号分隔；留空表示不信任 X-Forwarded-For。
	TrustedProxies []string
}

// AccessConfig 站点访问控制。
type AccessConfig struct {
	// private：所有页面和操作必须登录（默认）；
	// public：游客可浏览文章，写文章/互动/评论需要登录。
	Mode string
}

// DatabaseConfig 数据库配置。
type DatabaseConfig struct {
	// SQLite 文件路径。
	Path string
	// 写锁等待时间，默认 5s。
	BusyTimeout time.Duration
	// 连接池上限，SQLite 建议 1，串行写可彻底避免 SQLITE_BUSY。
	MaxOpenConns int
}

// JWTConfig JWT 配置。
type JWTConfig struct {
	Secret string
	TTL    time.Duration
}

// CaptchaConfig 注册人机验证配置。
type CaptchaConfig struct {
	// 关闭后注册接口不再要求验证码 ticket。
	Enabled bool
}

// RegisterConfig 注册开关配置。
type RegisterConfig struct {
	// 关闭后除首个管理员初始化外不再允许注册。
	Enabled bool
}

// LimitsConfig 防刷与内容治理参数。
type LimitsConfig struct {
	// 所有用户的写入频率。
	PostPerMinute     int
	CommentPerMinute  int
	ReactionPerMinute int

	// 新账号判定窗口：注册不足该时长且未可信时使用更严格的每日配额。
	NewAccountHours         time.Duration
	NewAccountPostPerDay    int
	NewAccountCommentPerDay int

	// 普通老用户每日配额。
	PostPerDay    int
	CommentPerDay int

	// 每 IP 成功注册次数上限。
	RegisterPerIPPerHour int
	RegisterPerIPPerDay  int

	// 注册与修改密码时的用户名/密码约束。
	MinPasswordBytes int
	MinUsernameRunes int
	MaxUsernameRunes int

	// 同一用户相同正文的去重窗口。
	DuplicateWindow time.Duration
}

// DefaultConfig 默认配置。
func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Port: defaultServerPort,
			Mode: defaultServerMode,
		},
		Access: AccessConfig{
			Mode: defaultAccessMode,
		},
		Database: DatabaseConfig{
			Path:         filepath.Join("data", "data.db"),
			BusyTimeout:  defaultDatabaseBusyTimeout,
			MaxOpenConns: defaultDatabaseMaxOpenConns,
		},
		JWT: JWTConfig{
			TTL: defaultJWTTTL,
		},
		Captcha: CaptchaConfig{
			Enabled: true,
		},
		Register: RegisterConfig{
			Enabled: true,
		},
		Limits: LimitsConfig{
			PostPerMinute:           5,
			CommentPerMinute:        10,
			ReactionPerMinute:       30,
			NewAccountHours:         24 * time.Hour,
			NewAccountPostPerDay:    5,
			NewAccountCommentPerDay: 20,
			PostPerDay:              200,
			CommentPerDay:           500,
			RegisterPerIPPerHour:    10,
			RegisterPerIPPerDay:     20,
			MinPasswordBytes:        8,
			MinUsernameRunes:        2,
			MaxUsernameRunes:        32,
			DuplicateWindow:         24 * time.Hour,
		},
	}
}

// LoadConfig 读取 INI；文件不存在时用默认值。
func LoadConfig(path string) (*Config, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		path = defaultConfigPath
	}

	cfg := DefaultConfig()

	f, err := ini.LoadSources(ini.LoadOptions{Insensitive: true}, path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return cfg, nil
		}
		return nil, fmt.Errorf("读取配置文件[%s]失败：%w", path, err)
	}

	cfg.Server.Host = strings.TrimSpace(f.Section("server").Key("host").String())

	port, err := readInt(f, "server", "port", cfg.Server.Port)
	if err != nil {
		return nil, err
	}
	if port < 1 || port > 65535 {
		return nil, fmt.Errorf("配置项 [server] port 超出范围[%d]：需要在 1-65535 之间", port)
	}
	cfg.Server.Port = port

	mode := strings.ToLower(strings.TrimSpace(f.Section("server").Key("mode").String()))
	switch mode {
	case "":
		// 留空时保持默认 release。
	case gin.DebugMode, gin.ReleaseMode, gin.TestMode:
		cfg.Server.Mode = mode
	default:
		return nil, fmt.Errorf("配置项 [server] mode 无效[%s]：可选 debug、release、test", mode)
	}

	cfg.Server.TrustedProxies = parseProxyList(f.Section("server").Key("trusted_proxies").String())

	accessMode := strings.ToLower(strings.TrimSpace(f.Section("access").Key("mode").String()))
	switch accessMode {
	case "":
		// 留空时保持默认 private。
	case AccessModePrivate, AccessModePublic:
		cfg.Access.Mode = accessMode
	default:
		return nil, fmt.Errorf("配置项 [access] mode 无效[%s]：可选 private、public", accessMode)
	}

	if dbPath := strings.TrimSpace(f.Section("database").Key("path").String()); dbPath != "" {
		cfg.Database.Path = filepath.Clean(dbPath)
	}

	busyTimeout, err := readDuration(f, "database", "busy_timeout", cfg.Database.BusyTimeout)
	if err != nil {
		return nil, err
	}
	cfg.Database.BusyTimeout = busyTimeout

	maxOpenConns, err := readInt(f, "database", "max_open_conns", cfg.Database.MaxOpenConns)
	if err != nil {
		return nil, err
	}
	if maxOpenConns < 1 || maxOpenConns > maxDatabaseOpenConns {
		return nil, fmt.Errorf("配置项 [database] max_open_conns 超出范围[%d]：需要在 1-%d 之间", maxOpenConns, maxDatabaseOpenConns)
	}
	cfg.Database.MaxOpenConns = maxOpenConns

	cfg.JWT.Secret = strings.TrimSpace(f.Section("jwt").Key("secret").String())

	ttl, err := readDuration(f, "jwt", "ttl", cfg.JWT.TTL)
	if err != nil {
		return nil, err
	}
	cfg.JWT.TTL = ttl

	// 人机验证默认开启，只有显式配置为 false 才关闭。
	if raw := strings.TrimSpace(f.Section("captcha").Key("enabled").String()); raw != "" {
		enabled, err := strconv.ParseBool(raw)
		if err != nil {
			return nil, fmt.Errorf("配置项 [captcha] enabled 无效[%s]：可选 true、false", raw)
		}
		cfg.Captcha.Enabled = enabled
	}

	// 注册默认开启，只有显式配置为 false 才关闭。
	if raw := strings.TrimSpace(f.Section("register").Key("enabled").String()); raw != "" {
		enabled, err := strconv.ParseBool(raw)
		if err != nil {
			return nil, fmt.Errorf("配置项 [register] enabled 无效[%s]：可选 true、false", raw)
		}
		cfg.Register.Enabled = enabled
	}

	if err := loadLimits(f, &cfg.Limits); err != nil {
		return nil, err
	}

	return cfg, nil
}

// loadLimits 读取 [limits] 防刷配置并做基本范围校验。
func loadLimits(f *ini.File, limits *LimitsConfig) error {
	var err error

	if limits.PostPerMinute, err = readInt(f, "limits", "post_per_minute", limits.PostPerMinute); err != nil {
		return err
	}
	if limits.PostPerMinute < 1 || limits.PostPerMinute > 1000 {
		return fmt.Errorf("配置项 [limits] post_per_minute 超出范围[%d]：需要在 1-1000 之间", limits.PostPerMinute)
	}

	if limits.CommentPerMinute, err = readInt(f, "limits", "comment_per_minute", limits.CommentPerMinute); err != nil {
		return err
	}
	if limits.CommentPerMinute < 1 || limits.CommentPerMinute > 1000 {
		return fmt.Errorf("配置项 [limits] comment_per_minute 超出范围[%d]：需要在 1-1000 之间", limits.CommentPerMinute)
	}

	if limits.ReactionPerMinute, err = readInt(f, "limits", "reaction_per_minute", limits.ReactionPerMinute); err != nil {
		return err
	}
	if limits.ReactionPerMinute < 1 || limits.ReactionPerMinute > 10000 {
		return fmt.Errorf("配置项 [limits] reaction_per_minute 超出范围[%d]：需要在 1-10000 之间", limits.ReactionPerMinute)
	}

	if limits.NewAccountHours, err = readDuration(f, "limits", "new_account_hours", limits.NewAccountHours); err != nil {
		return err
	}

	if limits.NewAccountPostPerDay, err = readInt(f, "limits", "new_account_post_per_day", limits.NewAccountPostPerDay); err != nil {
		return err
	}
	if limits.NewAccountPostPerDay < 1 {
		return fmt.Errorf("配置项 [limits] new_account_post_per_day 需要大于 0")
	}

	if limits.NewAccountCommentPerDay, err = readInt(f, "limits", "new_account_comment_per_day", limits.NewAccountCommentPerDay); err != nil {
		return err
	}
	if limits.NewAccountCommentPerDay < 1 {
		return fmt.Errorf("配置项 [limits] new_account_comment_per_day 需要大于 0")
	}

	if limits.PostPerDay, err = readInt(f, "limits", "post_per_day", limits.PostPerDay); err != nil {
		return err
	}
	if limits.PostPerDay < limits.NewAccountPostPerDay {
		return fmt.Errorf("配置项 [limits] post_per_day[%d] 不能小于 new_account_post_per_day[%d]", limits.PostPerDay, limits.NewAccountPostPerDay)
	}

	if limits.CommentPerDay, err = readInt(f, "limits", "comment_per_day", limits.CommentPerDay); err != nil {
		return err
	}
	if limits.CommentPerDay < limits.NewAccountCommentPerDay {
		return fmt.Errorf("配置项 [limits] comment_per_day[%d] 不能小于 new_account_comment_per_day[%d]", limits.CommentPerDay, limits.NewAccountCommentPerDay)
	}

	if limits.RegisterPerIPPerHour, err = readInt(f, "limits", "register_per_ip_per_hour", limits.RegisterPerIPPerHour); err != nil {
		return err
	}
	if limits.RegisterPerIPPerHour < 1 {
		return fmt.Errorf("配置项 [limits] register_per_ip_per_hour 需要大于 0")
	}

	if limits.RegisterPerIPPerDay, err = readInt(f, "limits", "register_per_ip_per_day", limits.RegisterPerIPPerDay); err != nil {
		return err
	}
	if limits.RegisterPerIPPerDay < limits.RegisterPerIPPerHour {
		return fmt.Errorf("配置项 [limits] register_per_ip_per_day[%d] 不能小于 register_per_ip_per_hour[%d]", limits.RegisterPerIPPerDay, limits.RegisterPerIPPerHour)
	}

	if limits.MinPasswordBytes, err = readInt(f, "limits", "min_password_bytes", limits.MinPasswordBytes); err != nil {
		return err
	}
	if limits.MinPasswordBytes < 6 || limits.MinPasswordBytes > 72 {
		return fmt.Errorf("配置项 [limits] min_password_bytes 超出范围[%d]：需要在 6-72 之间", limits.MinPasswordBytes)
	}

	if limits.MinUsernameRunes, err = readInt(f, "limits", "min_username_runes", limits.MinUsernameRunes); err != nil {
		return err
	}
	if limits.MinUsernameRunes < 1 || limits.MinUsernameRunes > 32 {
		return fmt.Errorf("配置项 [limits] min_username_runes 超出范围[%d]：需要在 1-32 之间", limits.MinUsernameRunes)
	}

	if limits.MaxUsernameRunes, err = readInt(f, "limits", "max_username_runes", limits.MaxUsernameRunes); err != nil {
		return err
	}
	if limits.MaxUsernameRunes < limits.MinUsernameRunes || limits.MaxUsernameRunes > 64 {
		return fmt.Errorf("配置项 [limits] max_username_runes[%d] 需要在 %d-64 之间", limits.MaxUsernameRunes, limits.MinUsernameRunes)
	}

	if limits.DuplicateWindow, err = readDuration(f, "limits", "duplicate_window", limits.DuplicateWindow); err != nil {
		return err
	}

	return nil
}

// readInt 读取整数，缺失时返回默认值。
func readInt(f *ini.File, section, key string, def int) (int, error) {
	raw := strings.TrimSpace(f.Section(section).Key(key).String())
	if raw == "" {
		return def, nil
	}

	v, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("配置项 [%s] %s 必须是整数，当前为[%s]", section, key, raw)
	}
	return v, nil
}

// readDuration 读取 duration，缺失时返回默认值。
func readDuration(f *ini.File, section, key string, def time.Duration) (time.Duration, error) {
	raw := strings.TrimSpace(f.Section(section).Key(key).String())
	if raw == "" {
		return def, nil
	}

	d, err := time.ParseDuration(raw)
	if err != nil || d <= 0 {
		return 0, fmt.Errorf("配置项 [%s] %s 无效[%s]：需要正的时间段，例如 24h、30m", section, key, raw)
	}
	return d, nil
}

// parseProxyList 解析逗号分隔的代理地址，忽略空项。
func parseProxyList(raw string) []string {
	parts := strings.Split(raw, ",")
	list := make([]string, 0, len(parts))
	for _, part := range parts {
		if value := strings.TrimSpace(part); value != "" {
			list = append(list, value)
		}
	}
	if len(list) == 0 {
		return nil
	}

	return list
}

// ListenAddr 返回监听地址。
func (c Config) ListenAddr() string {
	host := strings.TrimSpace(c.Server.Host)
	if host == "" {
		if gin.Mode() == gin.DebugMode {
			host = "127.0.0.1"
		} else {
			host = "0.0.0.0"
		}
	}
	return net.JoinHostPort(host, strconv.Itoa(c.Server.Port))
}

// ensureDatabaseDir 创建数据库父目录。
func ensureDatabaseDir(dbPath string) error {
	dir := filepath.Dir(dbPath)
	if dir == "." || dir == "" {
		return nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("创建数据库目录[%s]失败：%w", dir, err)
	}
	return nil
}
