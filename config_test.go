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
	if cfg.Server.Mode != "" {
		t.Fatalf("mode 默认值应为空，实际 %q", cfg.Server.Mode)
	}
	if cfg.Database.Path != filepath.Join("data", "data.db") {
		t.Fatalf("数据库默认路径异常：%s", cfg.Database.Path)
	}
	if cfg.JWT.Secret != "" {
		t.Fatalf("JWT secret 默认值应为空，实际 %q", cfg.JWT.Secret)
	}
	if cfg.JWT.TTL != defaultJWTTTL {
		t.Fatalf("JWT TTL 默认值应为 %s，实际 %s", defaultJWTTTL, cfg.JWT.TTL)
	}
}

func TestLoadConfigFromFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "app.ini")
	content := `
[server]
host = 127.0.0.1
port = 9090
mode = release

[database]
path = ./tmp/app.db

[jwt]
secret = test-secret
ttl = 2h
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
	if cfg.Database.Path != filepath.Clean("./tmp/app.db") {
		t.Fatalf("database.path 解析异常：%s", cfg.Database.Path)
	}
	if cfg.JWT.Secret != "test-secret" || cfg.JWT.TTL != 2*time.Hour {
		t.Fatalf("jwt 配置解析异常：%+v", cfg.JWT)
	}
}

func TestLoadConfigEmptyValuesUseDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "app.ini")
	content := `
[server]
host =
port =
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

	if cfg.Server.Host != "" || cfg.Server.Port != defaultServerPort || cfg.Server.Mode != "" {
		t.Fatalf("空 server 配置应回退默认值：%+v", cfg.Server)
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
	for _, path := range []string{"conf/app.ini", "conf/app.example.ini"} {
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
