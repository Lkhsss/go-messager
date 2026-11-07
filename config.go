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
	// defaultConfigPath 是默认配置文件路径，相对进程工作目录。
	defaultConfigPath = "conf/app.ini"
	// defaultServerPort 是默认 HTTP 端口。
	defaultServerPort = 8080
)

// Config 是应用启动所需的全部配置。
type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	JWT      JWTConfig
}

// ServerConfig 是 HTTP 服务相关配置。
type ServerConfig struct {
	// Host 留空时：debug 模式监听 127.0.0.1，其他模式监听 0.0.0.0。
	Host string
	Port int
	// Mode 为 gin 的运行模式：debug / release / test；留空时沿用 GIN_MODE。
	Mode string
}

// DatabaseConfig 是 SQLite 数据库配置。
type DatabaseConfig struct {
	// Path 是数据库文件路径，相对进程工作目录。
	Path string
}

// JWTConfig 是 JWT 签发相关配置。
type JWTConfig struct {
	Secret string
	TTL    time.Duration
}

// DefaultConfig 返回默认配置。
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

// LoadConfig 从 path 读取 INI 配置。
// 文件不存在时返回默认配置；文件存在但内容非法时返回错误。
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

// readInt 读取整数配置项；缺失或空值时返回默认值。
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

// readDuration 读取 Go duration 配置项；缺失或空值时返回默认值。
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

// ListenAddr 返回 HTTP 监听地址。
// Host 留空时保持原有行为：debug 模式监听本地回环，其他模式监听所有网卡。
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

// ensureDatabaseDir 创建 SQLite 数据库文件的父目录。
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
