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
)

// Config 应用配置。
type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	JWT      JWTConfig
}

// ServerConfig HTTP 配置。
type ServerConfig struct {
	// 留空时 debug 监听 127.0.0.1，其他模式监听 0.0.0.0。
	Host string
	Port int
	// debug/release/test，留空用 GIN_MODE。
	Mode string
}

// DatabaseConfig 数据库配置。
type DatabaseConfig struct {
	// SQLite 文件路径。
	Path string
}

// JWTConfig JWT 配置。
type JWTConfig struct {
	Secret string
	TTL    time.Duration
}

// DefaultConfig 默认配置。
func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Port: defaultServerPort,
		},
		Database: DatabaseConfig{
			Path: filepath.Join("data", "data.db"),
		},
		JWT: JWTConfig{
			TTL: defaultJWTTTL,
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
	case "", gin.DebugMode, gin.ReleaseMode, gin.TestMode:
		cfg.Server.Mode = mode
	default:
		return nil, fmt.Errorf("配置项 [server] mode 无效[%s]：可选 debug、release、test", mode)
	}

	if dbPath := strings.TrimSpace(f.Section("database").Key("path").String()); dbPath != "" {
		cfg.Database.Path = filepath.Clean(dbPath)
	}

	cfg.JWT.Secret = strings.TrimSpace(f.Section("jwt").Key("secret").String())

	ttl, err := readDuration(f, "jwt", "ttl", cfg.JWT.TTL)
	if err != nil {
		return nil, err
	}
	cfg.JWT.TTL = ttl

	return cfg, nil
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
