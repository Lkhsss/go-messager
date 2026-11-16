package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestLoadConfigMissingFileUsesDefaults(t *testing.T) {
	cfg, err := LoadConfig(filepath.Join(t.TempDir(), "missing.ini"))
	if err != nil {
		t.Fatalf("缺失配置文件不应报错：%v", err)
	}

	if cfg.Server.Host != "" {
		t.Fatalf("host 默认值应为空，实际 %q", cfg.Server.Host)
	}
	if cfg.Server.Port != defaultServerPort {
		t.Fatalf("port 默认值应为 %d，实际 %d", defaultServerPort, cfg.Server.Port)
	}
	if cfg.Server.Mode != gin.ReleaseMode {
		t.Fatalf("mode 默认值应为 release，实际 %q", cfg.Server.Mode)
	}
	if cfg.Access.Mode != AccessModePrivate {
		t.Fatalf("access.mode 默认值应为 private，实际 %q", cfg.Access.Mode)
	}
	if cfg.Database.Path != filepath.Join("data", "data.db") {
		t.Fatalf("数据库默认路径异常：%s", cfg.Database.Path)
	}
	if cfg.Database.BusyTimeout != defaultDatabaseBusyTimeout {
		t.Fatalf("busy_timeout 默认值应为 %s，实际 %s", defaultDatabaseBusyTimeout, cfg.Database.BusyTimeout)
	}
	if cfg.Database.MaxOpenConns != defaultDatabaseMaxOpenConns {
		t.Fatalf("max_open_conns 默认值应为 %d，实际 %d", defaultDatabaseMaxOpenConns, cfg.Database.MaxOpenConns)
	}
	if len(cfg.Server.TrustedProxies) != 0 {
		t.Fatalf("trusted_proxies 默认应为空，实际 %+v", cfg.Server.TrustedProxies)
	}
	if cfg.JWT.Secret != "" {
		t.Fatalf("JWT secret 默认值应为空，实际 %q", cfg.JWT.Secret)
	}
	if cfg.JWT.TTL != defaultJWTTTL {
		t.Fatalf("JWT TTL 默认值应为 %s，实际 %s", defaultJWTTTL, cfg.JWT.TTL)
	}
	if !cfg.Captcha.Enabled {
		t.Fatal("人机验证默认应开启")
	}
	if !cfg.Register.Enabled {
		t.Fatal("注册默认应开启")
	}
	if cfg.Limits.PostPerMinute != 5 || cfg.Limits.CommentPerMinute != 10 || cfg.Limits.ReactionPerMinute != 30 {
		t.Fatalf("limits 频率默认值异常：%+v", cfg.Limits)
	}
	if cfg.Limits.NewAccountPostPerDay != 5 || cfg.Limits.PostPerDay != 200 {
		t.Fatalf("limits 发文配额默认值异常：%+v", cfg.Limits)
	}
	if cfg.Limits.RegisterPerIPPerHour != 10 || cfg.Limits.RegisterPerIPPerDay != 20 {
		t.Fatalf("limits 注册默认值异常：%+v", cfg.Limits)
	}
	if cfg.Limits.MinPasswordBytes != 8 || cfg.Limits.MinUsernameRunes != 2 || cfg.Limits.MaxUsernameRunes != 32 {
		t.Fatalf("limits 账号约束默认值异常：%+v", cfg.Limits)
	}
	if cfg.Limits.DuplicateWindow != 24*time.Hour {
		t.Fatalf("limits 去重窗口默认值异常：%s", cfg.Limits.DuplicateWindow)
	}
}

func TestLoadConfigFromFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "app.ini")
	content := `
[server]
host = 127.0.0.1
port = 9090
mode = release
trusted_proxies = 127.0.0.1, 10.0.0.0/8

[access]
mode = public

[database]
path = ./tmp/app.db
busy_timeout = 3s
max_open_conns = 4

[jwt]
secret = test-secret
ttl = 2h

[captcha]
enabled = false

[register]
enabled = false

[limits]
post_per_minute = 3
comment_per_minute = 6
reaction_per_minute = 12
new_account_hours = 12h
new_account_post_per_day = 2
new_account_comment_per_day = 4
post_per_day = 20
comment_per_day = 40
register_per_ip_per_hour = 2
register_per_ip_per_day = 5
min_password_bytes = 10
min_username_runes = 3
max_username_runes = 20
duplicate_window = 6h
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("写入测试配置失败：%v", err)
	}

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("加载配置失败：%v", err)
	}

	if cfg.Server.Host != "127.0.0.1" || cfg.Server.Port != 9090 || cfg.Server.Mode != gin.ReleaseMode {
		t.Fatalf("server 配置解析异常：%+v", cfg.Server)
	}
	if cfg.Access.Mode != AccessModePublic {
		t.Fatalf("access.mode 解析异常：%q", cfg.Access.Mode)
	}
	if cfg.Database.Path != filepath.Clean("./tmp/app.db") {
		t.Fatalf("database.path 解析异常：%s", cfg.Database.Path)
	}
	if len(cfg.Server.TrustedProxies) != 2 || cfg.Server.TrustedProxies[0] != "127.0.0.1" || cfg.Server.TrustedProxies[1] != "10.0.0.0/8" {
		t.Fatalf("trusted_proxies 解析异常：%+v", cfg.Server.TrustedProxies)
	}
	if cfg.Database.BusyTimeout != 3*time.Second || cfg.Database.MaxOpenConns != 4 {
		t.Fatalf("database 连接配置解析异常：%+v", cfg.Database)
	}
	if cfg.JWT.Secret != "test-secret" || cfg.JWT.TTL != 2*time.Hour {
		t.Fatalf("jwt 配置解析异常：%+v", cfg.JWT)
	}
	if cfg.Captcha.Enabled {
		t.Fatal("captcha.enabled = false 时人机验证应关闭")
	}
	if cfg.Register.Enabled {
		t.Fatal("register.enabled = false 时注册应关闭")
	}
	if cfg.Limits.PostPerMinute != 3 || cfg.Limits.CommentPerMinute != 6 || cfg.Limits.ReactionPerMinute != 12 {
		t.Fatalf("limits 频率解析异常：%+v", cfg.Limits)
	}
	if cfg.Limits.NewAccountHours != 12*time.Hour || cfg.Limits.NewAccountPostPerDay != 2 {
		t.Fatalf("limits 新账号配置解析异常：%+v", cfg.Limits)
	}
	if cfg.Limits.PostPerDay != 20 || cfg.Limits.CommentPerDay != 40 {
		t.Fatalf("limits 每日配额解析异常：%+v", cfg.Limits)
	}
	if cfg.Limits.RegisterPerIPPerHour != 2 || cfg.Limits.RegisterPerIPPerDay != 5 {
		t.Fatalf("limits 注册限制解析异常：%+v", cfg.Limits)
	}
	if cfg.Limits.MinPasswordBytes != 10 || cfg.Limits.MinUsernameRunes != 3 || cfg.Limits.MaxUsernameRunes != 20 {
		t.Fatalf("limits 账号约束解析异常：%+v", cfg.Limits)
	}
	if cfg.Limits.DuplicateWindow != 6*time.Hour {
		t.Fatalf("limits 去重窗口解析异常：%s", cfg.Limits.DuplicateWindow)
	}
}

func TestLoadConfigEmptyValuesUseDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "app.ini")
	content := `
[server]
host =
port =
mode =

[access]
mode =

[database]
path =

[jwt]
secret =
ttl =
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("写入测试配置失败：%v", err)
	}

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("加载配置失败：%v", err)
	}

	if cfg.Server.Host != "" || cfg.Server.Port != defaultServerPort || cfg.Server.Mode != gin.ReleaseMode {
		t.Fatalf("空 server 配置应回退默认值：%+v", cfg.Server)
	}
	if cfg.Access.Mode != AccessModePrivate {
		t.Fatalf("空 access.mode 应回退 private，实际 %q", cfg.Access.Mode)
	}
	if cfg.Database.Path != filepath.Join("data", "data.db") {
		t.Fatalf("空 database.path 应回退默认值：%s", cfg.Database.Path)
	}
	if cfg.JWT.Secret != "" || cfg.JWT.TTL != defaultJWTTTL {
		t.Fatalf("空 jwt 配置应回退默认值：%+v", cfg.JWT)
	}
}

func TestLoadConfigInvalidValues(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{name: "port 非整数", body: "[server]\nport = abc\n", want: "[server] port"},
		{name: "port 超出范围", body: "[server]\nport = 0\n", want: "1-65535"},
		{name: "ttl 非法", body: "[jwt]\nttl = 2d\n", want: "[jwt] ttl"},
		{name: "mode 非法", body: "[server]\nmode = prod\n", want: "[server] mode"},
		{name: "access mode 非法", body: "[access]\nmode = secret\n", want: "[access] mode"},
		{name: "captcha enabled 非法", body: "[captcha]\nenabled = maybe\n", want: "[captcha] enabled"},
		{name: "register enabled 非法", body: "[register]\nenabled = maybe\n", want: "[register] enabled"},
		{name: "busy_timeout 非法", body: "[database]\nbusy_timeout = abc\n", want: "[database] busy_timeout"},
		{name: "max_open_conns 非整数", body: "[database]\nmax_open_conns = abc\n", want: "[database] max_open_conns"},
		{name: "max_open_conns 越界", body: "[database]\nmax_open_conns = 99\n", want: "[database] max_open_conns"},
		{name: "post_per_minute 越界", body: "[limits]\npost_per_minute = 0\n", want: "[limits] post_per_minute"},
		{name: "min_password_bytes 越界", body: "[limits]\nmin_password_bytes = 1\n", want: "[limits] min_password_bytes"},
		{name: "注册日上限小于小时上限", body: "[limits]\nregister_per_ip_per_day = 1\n", want: "register_per_ip_per_day"},
		{name: "每日文章配额小于新账号配额", body: "[limits]\npost_per_day = 1\n", want: "post_per_day"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "app.ini")
			if err := os.WriteFile(path, []byte(tt.body), 0o600); err != nil {
				t.Fatalf("写入测试配置失败：%v", err)
			}

			_, err := LoadConfig(path)
			if err == nil {
				t.Fatal("期望配置错误，实际为 nil")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("错误信息应包含 %q，实际：%v", tt.want, err)
			}
		})
	}
}

func TestListenAddr(t *testing.T) {
	oldMode := gin.Mode()
	t.Cleanup(func() {
		gin.SetMode(oldMode)
	})

	cfg := DefaultConfig()

	gin.SetMode(gin.DebugMode)
	if got := cfg.ListenAddr(); got != "127.0.0.1:8080" {
		t.Fatalf("debug 模式默认监听地址异常：%s", got)
	}

	gin.SetMode(gin.ReleaseMode)
	if got := cfg.ListenAddr(); got != "0.0.0.0:8080" {
		t.Fatalf("release 模式默认监听地址异常：%s", got)
	}

	cfg.Server.Host = "127.0.0.1"
	cfg.Server.Port = 9090
	if got := cfg.ListenAddr(); got != "127.0.0.1:9090" {
		t.Fatalf("显式 host/port 监听地址异常：%s", got)
	}
}

func TestEnsureDatabaseDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "a", "b")
	dbPath := filepath.Join(dir, "data.db")

	if err := ensureDatabaseDir(dbPath); err != nil {
		t.Fatalf("创建数据库目录失败：%v", err)
	}

	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("数据库目录不存在：%v", err)
	}
	if !info.IsDir() {
		t.Fatalf("数据库目录路径不是目录：%s", dir)
	}
}

func TestShippedConfigFiles(t *testing.T) {
	for _, path := range []string{"conf/app.min.ini", "conf/app.example.ini"} {
		if _, err := LoadConfig(path); err != nil {
			t.Errorf("加载仓库配置文件[%s]失败：%v", path, err)
		}
	}
}

func TestResolveJWTTTL(t *testing.T) {
	t.Setenv("JWT_TTL", "")
	if got, err := resolveJWTTTL(2 * time.Hour); err != nil || got != 2*time.Hour {
		t.Fatalf("应使用配置中的 TTL，实际 %s，err=%v", got, err)
	}

	t.Setenv("JWT_TTL", "30m")
	if got, err := resolveJWTTTL(2 * time.Hour); err != nil || got != 30*time.Minute {
		t.Fatalf("环境变量 JWT_TTL 应优先，实际 %s，err=%v", got, err)
	}

	t.Setenv("JWT_TTL", "bad")
	if _, err := resolveJWTTTL(2 * time.Hour); err == nil {
		t.Fatal("非法 JWT_TTL 应返回错误")
	}
}

func TestNewDataUsesJWTConfig(t *testing.T) {
	t.Setenv("JWT_SECRET", "")
	t.Setenv("JWT_TTL", "")

	db, err := gorm.Open(
		sqlite.Open(filepath.Join(t.TempDir(), "test.db")),
		&gorm.Config{Logger: logger.Default.LogMode(logger.Silent)},
	)
	if err != nil {
		t.Fatalf("打开测试数据库失败：%v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("获取底层连接失败：%v", err)
	}
	t.Cleanup(func() {
		_ = sqlDB.Close()
	})

	data, err := NewData(db, JWTConfig{Secret: "config-secret", TTL: 2 * time.Hour})
	if err != nil {
		t.Fatalf("初始化数据层失败：%v", err)
	}
	if string(data.JWTSecret) != "config-secret" {
		t.Fatalf("应使用配置文件中的 JWT secret，实际 %q", string(data.JWTSecret))
	}
	if data.JWTTTL != 2*time.Hour {
		t.Fatalf("应使用配置文件中的 JWT TTL，实际 %s", data.JWTTTL)
	}

	t.Setenv("JWT_SECRET", "env-secret")
	data, err = NewData(db, JWTConfig{Secret: "config-secret", TTL: 2 * time.Hour})
	if err != nil {
		t.Fatalf("初始化数据层失败：%v", err)
	}
	if string(data.JWTSecret) != "env-secret" {
		t.Fatalf("JWT_SECRET 环境变量应优先，实际 %q", string(data.JWTSecret))
	}
}
