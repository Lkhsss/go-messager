package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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
