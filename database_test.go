package main

import (
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCheckDatabaseWritableCreatesFileAndCleansProbe(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "data.db")

	if err := checkDatabaseWritable(dbPath); err != nil {
		t.Fatalf("可写目录不应报错：%v", err)
	}

	info, err := os.Stat(dbPath)
	if err != nil {
		t.Fatalf("应创建数据库文件：%v", err)
	}
	if info.IsDir() {
		t.Fatalf("数据库路径不应是目录：%s", dbPath)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("读取目录失败：%v", err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".go-messager-write-") {
			t.Fatalf("写权限探测用的临时文件未清理：%s", entry.Name())
		}
	}
}

func TestCheckDatabaseWritableMissingDir(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "missing", "data.db")

	err := checkDatabaseWritable(dbPath)
	if err == nil {
		t.Fatal("父目录不存在时应返回错误")
	}
	if !strings.Contains(err.Error(), "不可写") {
		t.Fatalf("错误信息应说明不可写，实际：%v", err)
	}
	if !strings.Contains(err.Error(), dbPath) {
		t.Fatalf("错误信息应包含数据库路径 %s，实际：%v", dbPath, err)
	}
}

func TestOpenDatabaseSuccess(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "data.db")

	db, err := openDatabase(dbPath)
	if err != nil {
		t.Fatalf("打开数据库失败：%v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("获取底层连接失败：%v", err)
	}
	t.Cleanup(func() {
		_ = sqlDB.Close()
	})

	if err := db.AutoMigrate(&User{}); err != nil {
		t.Fatalf("建表失败：%v", err)
	}
}

func TestOpenDatabaseErrorMentionsPath(t *testing.T) {
	dir := t.TempDir() // 用目录当数据库路径，必然打开失败

	_, err := openDatabase(dir)
	if err == nil {
		t.Fatal("数据库路径是目录时应返回错误")
	}
	if !strings.Contains(err.Error(), "打开数据库") {
		t.Fatalf("错误信息应说明打开数据库失败，实际：%v", err)
	}
	if !strings.Contains(err.Error(), dir) {
		t.Fatalf("错误信息应包含数据库路径 %s，实际：%v", dir, err)
	}
}

func TestDatabaseDSNUsesPragmas(t *testing.T) {
	dsn := databaseDSN("data/data.db", 5*time.Second)
	path, rawQuery, ok := strings.Cut(dsn, "?")
	if !ok || path != "data/data.db" {
		t.Fatalf("DSN 路径异常：%s", dsn)
	}

	query, err := url.ParseQuery(rawQuery)
	if err != nil {
		t.Fatalf("解析 DSN 失败：%v", err)
	}

	pragmas := query["_pragma"]
	for _, want := range []string{"journal_mode(WAL)", "busy_timeout(5000)", "synchronous(NORMAL)"} {
		found := false
		for _, value := range pragmas {
			if value == want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("DSN 缺少 %s：%v", want, pragmas)
		}
	}
	if query.Get("_txlock") != "immediate" {
		t.Fatalf("_txlock 应为 immediate，实际 %q", query.Get("_txlock"))
	}
}

func TestOpenDatabaseAppliesPragmasAndPool(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "data.db")
	db, err := openDatabaseWithOptions(dbPath, dbPoolOptions{
		busyTimeout:  3 * time.Second,
		maxOpenConns: 1,
	})
	if err != nil {
		t.Fatalf("打开数据库失败：%v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("获取底层连接失败：%v", err)
	}
	t.Cleanup(func() {
		_ = sqlDB.Close()
	})

	if got := sqlDB.Stats().MaxOpenConnections; got != 1 {
		t.Fatalf("连接池上限应为 1，实际 %d", got)
	}

	var journalMode string
	if err := db.Raw("PRAGMA journal_mode").Scan(&journalMode).Error; err != nil {
		t.Fatalf("读取 journal_mode 失败：%v", err)
	}
	if strings.ToLower(journalMode) != "wal" {
		t.Fatalf("journal_mode 应为 wal，实际 %s", journalMode)
	}

	var busyTimeout int64
	if err := db.Raw("PRAGMA busy_timeout").Scan(&busyTimeout).Error; err != nil {
		t.Fatalf("读取 busy_timeout 失败：%v", err)
	}
	if busyTimeout != 3000 {
		t.Fatalf("busy_timeout 应为 3000，实际 %d", busyTimeout)
	}

	var synchronous int64
	if err := db.Raw("PRAGMA synchronous").Scan(&synchronous).Error; err != nil {
		t.Fatalf("读取 synchronous 失败：%v", err)
	}
	if synchronous != 1 {
		t.Fatalf("synchronous 应为 1(NORMAL)，实际 %d", synchronous)
	}
}
