package main

import (
	"fmt"

	"gorm.io/gorm"
)

// ensureDatabaseIndexes 创建 AutoMigrate 不便声明的复合/中间表索引。
// 全部使用 IF NOT EXISTS，重复启动不会重复建索引。
func ensureDatabaseIndexes(db *gorm.DB) error {
	indexNames := []string{
		"idx_posts_feed",
		"idx_posts_created_at",
		"idx_post_tags_post_id",
		"idx_posts_user_root",
		"idx_posts_user_hash",
		"idx_posts_user_stats",
		"idx_posts_user_created",
		"idx_reactions_user_id",
	}

	created := false
	for _, name := range indexNames {
		var count int64
		if err := db.Raw("SELECT COUNT(*) FROM sqlite_master WHERE type = 'index' AND name = ?", name).Scan(&count).Error; err != nil {
			return fmt.Errorf("检查数据库索引[%s]失败：%w", name, err)
		}
		if count == 0 {
			created = true
		}
	}

	statements := []string{
		// 首页列表与总数：root_id + deleted_at 等值过滤后按 id 反向扫描。
		`CREATE INDEX IF NOT EXISTS idx_posts_feed ON posts(root_id, deleted_at, id)`,
		// 近 24 小时数量、统计曲线按时间过滤。
		`CREATE INDEX IF NOT EXISTS idx_posts_created_at ON posts(created_at)`,
		// 多对多中间表按文章反查标签。
		`CREATE INDEX IF NOT EXISTS idx_post_tags_post_id ON post_tags(post_id)`,
		// 用户统计/管理员聚合的覆盖索引，避免为读 char_count 等列而扫描大正文所在行。
		`CREATE INDEX IF NOT EXISTS idx_posts_user_stats ON posts(user, root_id, char_count, like_count, dislike_count, question_count)`,
		// 管理员列表取每个用户最近活跃时间。
		`CREATE INDEX IF NOT EXISTS idx_posts_user_created ON posts(user, created_at)`,
	}

	for _, statement := range statements {
		if err := db.Exec(statement).Error; err != nil {
			return fmt.Errorf("创建数据库索引失败：%w", err)
		}
	}

	// 新建索引后刷新统计信息，否则 SQLite 可能继续选择旧的单列索引。
	if created {
		if err := db.Exec("ANALYZE").Error; err != nil {
			return fmt.Errorf("更新数据库统计信息失败：%w", err)
		}
	}

	return nil
}
