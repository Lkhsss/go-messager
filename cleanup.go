package main

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// 可清理分类的 key。
const (
	cleanupKeyDeletedPosts    = "deleted_posts"
	cleanupKeyDeletedComments = "deleted_comments"
	cleanupKeyOrphanComments  = "orphan_comments"
	cleanupKeyOrphanReactions = "orphan_reactions"
	cleanupKeyOrphanPostTags  = "orphan_post_tags"
	cleanupKeyOrphanDaily     = "orphan_daily_progress"
)

// 每行固定开销（字节），只用于估算占用空间。
const (
	postRowOverheadBytes      = 64
	reactionRowOverheadBytes  = 32
	postTagRowOverheadBytes   = 16
	dailyTaskRowOverheadBytes = 48
)

// 子查询：可见文章 / 已删除文章。
const (
	cleanupVisibleArticlesSQL = `SELECT id FROM posts WHERE deleted_at IS NULL AND root_id IS NULL`
	cleanupDeletedArticlesSQL = `SELECT id FROM posts WHERE deleted_at IS NOT NULL AND root_id IS NULL`
)

// cleanupDeletedArticlesSubtreeSQL 已删除文章及其下评论与回复的 id。
const cleanupDeletedArticlesSubtreeSQL = cleanupDeletedArticlesSQL + ` UNION SELECT id FROM posts WHERE root_id IN (` + cleanupDeletedArticlesSQL + `)`

// cleanupOrphanCommentsCTE 孤立评论及其回复（递归包含全部后代）。
const cleanupOrphanCommentsCTE = `WITH RECURSIVE cleanup_orphans(id) AS (
  SELECT id FROM posts
  WHERE deleted_at IS NULL AND root_id IS NOT NULL
    AND root_id NOT IN (` + cleanupDeletedArticlesSQL + `)
    AND (
      root_id NOT IN (SELECT id FROM posts)
      OR (parent_id IS NOT NULL AND parent_id NOT IN (SELECT id FROM posts))
      OR (parent_id IS NOT NULL AND parent_id IN (SELECT id FROM posts WHERE deleted_at IS NOT NULL))
    )
  UNION
  SELECT p.id FROM posts p JOIN cleanup_orphans o ON p.parent_id = o.id WHERE p.deleted_at IS NULL
)`

// CleanupCategory 单个可清理分类。
type CleanupCategory struct {
	Key         string `json:"key"`
	Label       string `json:"label"`
	Description string `json:"description"`
	Count       int64  `json:"count"`
	Bytes       int64  `json:"bytes"`
}

// CleanupDatabaseInfo SQLite 文件占用信息，均为真实值。
type CleanupDatabaseInfo struct {
	PageSize      int64 `json:"page_size"`
	PageCount     int64 `json:"page_count"`
	FileSizeBytes int64 `json:"file_size_bytes"`
	FreePages     int64 `json:"freelist_count"`
	FreeBytes     int64 `json:"free_bytes"`
}

// CleanupStatsResponse 清理页统计数据。
type CleanupStatsResponse struct {
	Categories []CleanupCategory   `json:"categories"`
	TotalCount int64               `json:"total_count"`
	TotalBytes int64               `json:"total_bytes"`
	Database   CleanupDatabaseInfo `json:"database"`
}

// CleanupResult 单类清理结果。
type CleanupResult struct {
	Key        string `json:"key"`
	Deleted    int64  `json:"deleted"`
	FreedBytes int64  `json:"freed_bytes"`
}

type cleanupCategoryDef struct {
	Key         string
	Label       string
	Description string
}

// cleanupCategoryDefs 按清理顺序排列：先处理整棵文章子树，再处理评论与孤立数据。
var cleanupCategoryDefs = []cleanupCategoryDef{
	{
		Key:         cleanupKeyDeletedPosts,
		Label:       "已删除文章",
		Description: "软删除的文章，连同其下评论与回复、反应和标签关联一起彻底删除",
	},
	{
		Key:         cleanupKeyDeletedComments,
		Label:       "已删除评论",
		Description: "仍在可见文章下被删除的评论与回复，连同其反应一起彻底删除",
	},
	{
		Key:         cleanupKeyOrphanComments,
		Label:       "孤立评论",
		Description: "所属文章或父评论已不存在（被彻底删除）的评论与回复",
	},
	{
		Key:         cleanupKeyOrphanReactions,
		Label:       "孤立反应",
		Description: "文章或用户已不存在，仍留在 reactions 表中的点赞等反应",
	},
	{
		Key:         cleanupKeyOrphanPostTags,
		Label:       "孤立标签关联",
		Description: "文章或标签已不存在，仍留在 post_tags 表中的关联记录",
	},
	{
		Key:         cleanupKeyOrphanDaily,
		Label:       "孤立每日任务",
		Description: "用户已不存在，仍留在 daily_task_progresses 表中的每日任务进度",
	},
}

// cleanupCategoryByKey 按 key 查分类定义。
func cleanupCategoryByKey(key string) (cleanupCategoryDef, bool) {
	for _, def := range cleanupCategoryDefs {
		if def.Key == key {
			return def, true
		}
	}
	return cleanupCategoryDef{}, false
}

// cleanupStats 汇总所有分类与数据库占用。
func (d *Data) cleanupStats() (CleanupStatsResponse, error) {
	stats := CleanupStatsResponse{Categories: make([]CleanupCategory, 0, len(cleanupCategoryDefs))}
	for _, def := range cleanupCategoryDefs {
		category, err := d.cleanupCategoryStats(def.Key)
		if err != nil {
			return CleanupStatsResponse{}, err
		}
		stats.Categories = append(stats.Categories, category)
		stats.TotalCount += category.Count
		stats.TotalBytes += category.Bytes
	}

	info, err := d.databaseInfo()
	if err != nil {
		return CleanupStatsResponse{}, err
	}
	stats.Database = info

	return stats, nil
}

// databaseInfo 读取 SQLite 页信息并换算成字节。
func (d *Data) databaseInfo() (CleanupDatabaseInfo, error) {
	var info CleanupDatabaseInfo
	if err := d.DB.Raw("PRAGMA page_size").Scan(&info.PageSize).Error; err != nil {
		return CleanupDatabaseInfo{}, fmt.Errorf("读取 page_size 失败：%w", err)
	}
	if err := d.DB.Raw("PRAGMA page_count").Scan(&info.PageCount).Error; err != nil {
		return CleanupDatabaseInfo{}, fmt.Errorf("读取 page_count 失败：%w", err)
	}
	if err := d.DB.Raw("PRAGMA freelist_count").Scan(&info.FreePages).Error; err != nil {
		return CleanupDatabaseInfo{}, fmt.Errorf("读取 freelist_count 失败：%w", err)
	}

	info.FileSizeBytes = info.PageSize * info.PageCount
	info.FreeBytes = info.PageSize * info.FreePages

	return info, nil
}

// cleanupCategoryStats 统计单个分类的数量与估算占用。
func (d *Data) cleanupCategoryStats(key string) (CleanupCategory, error) {
	def, ok := cleanupCategoryByKey(key)
	if !ok {
		return CleanupCategory{}, fmt.Errorf("未知的清理分类[%s]", key)
	}

	category := CleanupCategory{Key: def.Key, Label: def.Label, Description: def.Description}
	var err error

	switch key {
	case cleanupKeyDeletedPosts:
		if category.Count, err = countRows(d.DB, "posts", "deleted_at IS NOT NULL AND root_id IS NULL"); err != nil {
			return CleanupCategory{}, err
		}
		articleBytes, err := sumPostsBytes(d.DB, "deleted_at IS NOT NULL AND root_id IS NULL")
		if err != nil {
			return CleanupCategory{}, err
		}
		subtreeBytes, err := sumPostsBytes(d.DB, "root_id IN ("+cleanupDeletedArticlesSQL+")")
		if err != nil {
			return CleanupCategory{}, err
		}
		reactionBytes, err := sumReactionBytes(d.DB, "post_id IN ("+cleanupDeletedArticlesSubtreeSQL+")")
		if err != nil {
			return CleanupCategory{}, err
		}
		tags, err := countRows(d.DB, "post_tags", "post_id IN ("+cleanupDeletedArticlesSubtreeSQL+")")
		if err != nil {
			return CleanupCategory{}, err
		}
		category.Bytes = articleBytes + subtreeBytes + reactionBytes + tags*postTagRowOverheadBytes

	case cleanupKeyDeletedComments:
		where := "deleted_at IS NOT NULL AND root_id IS NOT NULL AND root_id IN (" + cleanupVisibleArticlesSQL + ")"
		if category.Count, err = countRows(d.DB, "posts", where); err != nil {
			return CleanupCategory{}, err
		}
		postBytes, err := sumPostsBytes(d.DB, where)
		if err != nil {
			return CleanupCategory{}, err
		}
		reactionBytes, err := sumReactionBytes(d.DB, "post_id IN (SELECT id FROM posts WHERE "+where+")")
		if err != nil {
			return CleanupCategory{}, err
		}
		tags, err := countRows(d.DB, "post_tags", "post_id IN (SELECT id FROM posts WHERE "+where+")")
		if err != nil {
			return CleanupCategory{}, err
		}
		category.Bytes = postBytes + reactionBytes + tags*postTagRowOverheadBytes

	case cleanupKeyOrphanComments:
		if category.Count, err = scalarInt64(d.DB, cleanupOrphanCommentsCTE+" SELECT COUNT(*) FROM cleanup_orphans"); err != nil {
			return CleanupCategory{}, err
		}
		postBytes, err := scalarInt64(d.DB, cleanupOrphanCommentsCTE+" SELECT COALESCE(SUM("+postContentBytesSQL()+"),0) FROM posts WHERE id IN (SELECT id FROM cleanup_orphans)")
		if err != nil {
			return CleanupCategory{}, err
		}
		reactionBytes, err := scalarInt64(d.DB, cleanupOrphanCommentsCTE+" SELECT COALESCE(SUM("+reactionContentBytesSQL()+"),0) FROM reactions WHERE post_id IN (SELECT id FROM cleanup_orphans)")
		if err != nil {
			return CleanupCategory{}, err
		}
		tags, err := scalarInt64(d.DB, cleanupOrphanCommentsCTE+" SELECT COUNT(*) FROM post_tags WHERE post_id IN (SELECT id FROM cleanup_orphans)")
		if err != nil {
			return CleanupCategory{}, err
		}
		category.Bytes = postBytes + reactionBytes + tags*postTagRowOverheadBytes

	case cleanupKeyOrphanReactions:
		where := "post_id NOT IN (SELECT id FROM posts) OR user_id NOT IN (SELECT id FROM users)"
		if category.Count, err = countRows(d.DB, "reactions", where); err != nil {
			return CleanupCategory{}, err
		}
		if category.Bytes, err = sumReactionBytes(d.DB, where); err != nil {
			return CleanupCategory{}, err
		}

	case cleanupKeyOrphanPostTags:
		if category.Count, err = countRows(d.DB, "post_tags", "post_id NOT IN (SELECT id FROM posts) OR tag_id NOT IN (SELECT id FROM tags)"); err != nil {
			return CleanupCategory{}, err
		}
		category.Bytes = category.Count * postTagRowOverheadBytes

	case cleanupKeyOrphanDaily:
		if category.Count, err = countRows(d.DB, "daily_task_progresses", "user_id NOT IN (SELECT id FROM users)"); err != nil {
			return CleanupCategory{}, err
		}
		category.Bytes = category.Count * dailyTaskRowOverheadBytes

	default:
		return CleanupCategory{}, fmt.Errorf("未知的清理分类[%s]", key)
	}

	return category, nil
}

// scalarInt64 执行只返回一列一行的统计查询。
func scalarInt64(db *gorm.DB, sql string, args ...any) (int64, error) {
	var value int64
	if err := db.Raw(sql, args...).Scan(&value).Error; err != nil {
		return 0, err
	}
	return value, nil
}

func countRows(db *gorm.DB, table, where string) (int64, error) {
	return scalarInt64(db, "SELECT COUNT(*) FROM "+table+" WHERE "+where)
}

func sumPostsBytes(db *gorm.DB, where string) (int64, error) {
	return scalarInt64(db, "SELECT COALESCE(SUM("+postContentBytesSQL()+"),0) FROM posts WHERE "+where)
}

func sumReactionBytes(db *gorm.DB, where string) (int64, error) {
	return scalarInt64(db, "SELECT COALESCE(SUM("+reactionContentBytesSQL()+"),0) FROM reactions WHERE "+where)
}

// postContentBytesSQL 估算 posts 行内容字节（文本字段 + 固定开销）。
func postContentBytesSQL() string {
	return "COALESCE(LENGTH(CAST(user AS BLOB)),0)" +
		"+COALESCE(LENGTH(CAST(title AS BLOB)),0)" +
		"+COALESCE(LENGTH(CAST(subtitle AS BLOB)),0)" +
		"+COALESCE(LENGTH(CAST(body AS BLOB)),0)" +
		"+COALESCE(LENGTH(CAST(excerpt AS BLOB)),0)" +
		"+COALESCE(LENGTH(CAST(emoji_counts AS BLOB)),0)" +
		fmt.Sprintf("+%d", postRowOverheadBytes)
}

// reactionContentBytesSQL 估算 reactions 行内容字节。
func reactionContentBytesSQL() string {
	return "COALESCE(LENGTH(CAST(type AS BLOB)),0)" + fmt.Sprintf("+%d", reactionRowOverheadBytes)
}

// GetAdminCleanup GET /api/admin/cleanup
func (d *Data) GetAdminCleanup(c *gin.Context) {
	stats, err := d.cleanupStats()
	if err != nil {
		Response{}.Fail(c, http.StatusInternalServerError, "QUERY_FAILED", err.Error())
		return
	}

	Response{}.Ok(c, http.StatusOK, stats)
}

// DeleteAdminCleanup 清理单个分类 DELETE /api/admin/cleanup/:key
func (d *Data) DeleteAdminCleanup(c *gin.Context) {
	key := strings.TrimSpace(c.Param("key"))
	if _, ok := cleanupCategoryByKey(key); !ok {
		Response{}.Fail(c, http.StatusBadRequest, "INVALID_CLEANUP_KEY", "未知的清理分类")
		return
	}

	category, err := d.cleanupCategoryStats(key)
	if err != nil {
		Response{}.Fail(c, http.StatusInternalServerError, "QUERY_FAILED", err.Error())
		return
	}

	deleted, err := d.runCleanup(key)
	if err != nil {
		Response{}.Fail(c, http.StatusInternalServerError, "DELETE_FAILED", err.Error())
		return
	}

	Response{}.Ok(c, http.StatusOK, CleanupResult{Key: key, Deleted: deleted, FreedBytes: category.Bytes})
}

// DeleteAllAdminCleanup 一键清理所有分类 DELETE /api/admin/cleanup
func (d *Data) DeleteAllAdminCleanup(c *gin.Context) {
	stats, err := d.cleanupStats()
	if err != nil {
		Response{}.Fail(c, http.StatusInternalServerError, "QUERY_FAILED", err.Error())
		return
	}

	bytesByKey := make(map[string]int64, len(stats.Categories))
	for _, category := range stats.Categories {
		bytesByKey[category.Key] = category.Bytes
	}

	results := make([]CleanupResult, 0, len(cleanupCategoryDefs))
	err = d.DB.Transaction(func(tx *gorm.DB) error {
		for _, def := range cleanupCategoryDefs {
			deleted, err := runCleanupCategory(tx, def.Key)
			if err != nil {
				return err
			}
			results = append(results, CleanupResult{
				Key:        def.Key,
				Deleted:    deleted,
				FreedBytes: bytesByKey[def.Key],
			})
		}

		return syncCleanupCounts(tx)
	})
	if err != nil {
		Response{}.Fail(c, http.StatusInternalServerError, "DELETE_FAILED", err.Error())
		return
	}

	Response{}.Ok(c, http.StatusOK, results)
}

// VacuumAdminCleanup 压缩数据库 POST /api/admin/cleanup/vacuum
func (d *Data) VacuumAdminCleanup(c *gin.Context) {
	before, err := d.databaseInfo()
	if err != nil {
		Response{}.Fail(c, http.StatusInternalServerError, "QUERY_FAILED", err.Error())
		return
	}

	// VACUUM 不能在事务中执行，会重建整个数据库文件。
	if err := d.DB.Exec("VACUUM").Error; err != nil {
		Response{}.Fail(c, http.StatusInternalServerError, "VACUUM_FAILED", err.Error())
		return
	}

	after, err := d.databaseInfo()
	if err != nil {
		Response{}.Fail(c, http.StatusInternalServerError, "QUERY_FAILED", err.Error())
		return
	}

	Response{}.Ok(c, http.StatusOK, gin.H{
		"before_bytes": before.FileSizeBytes,
		"after_bytes":  after.FileSizeBytes,
		"freed_bytes":  before.FileSizeBytes - after.FileSizeBytes,
		"free_bytes":   after.FreeBytes,
	})
}

// runCleanup 在事务内执行单个分类清理并校准统计。
func (d *Data) runCleanup(key string) (int64, error) {
	var deleted int64
	err := d.DB.Transaction(func(tx *gorm.DB) error {
		count, err := runCleanupCategory(tx, key)
		if err != nil {
			return err
		}
		deleted = count

		return syncCleanupCounts(tx)
	})

	return deleted, err
}

// syncCleanupCounts 清理后校准标签使用次数与反应统计。
func syncCleanupCounts(tx *gorm.DB) error {
	if err := syncTagUsageCounts(tx); err != nil {
		return err
	}

	return syncReactionCounts(tx)
}

// runCleanupCategory 按分类执行硬删除。
func runCleanupCategory(tx *gorm.DB, key string) (int64, error) {
	switch key {
	case cleanupKeyDeletedPosts:
		return cleanupDeletedPosts(tx)
	case cleanupKeyDeletedComments:
		return cleanupDeletedComments(tx)
	case cleanupKeyOrphanComments:
		return cleanupOrphanComments(tx)
	case cleanupKeyOrphanReactions:
		return cleanupOrphanReactions(tx)
	case cleanupKeyOrphanPostTags:
		return cleanupOrphanPostTags(tx)
	case cleanupKeyOrphanDaily:
		return cleanupOrphanDailyTasks(tx)
	default:
		return 0, fmt.Errorf("未知的清理分类[%s]", key)
	}
}

// cleanupDeletedPosts 硬删除软删除的文章及其整棵评论子树。
func cleanupDeletedPosts(tx *gorm.DB) (int64, error) {
	articleIDs, err := pluckIDs(tx, "SELECT id FROM posts WHERE deleted_at IS NOT NULL AND root_id IS NULL")
	if err != nil {
		return 0, err
	}
	if len(articleIDs) == 0 {
		return 0, nil
	}

	var subtreeIDs []uint
	if err := tx.Unscoped().Model(&Post{}).Where("root_id IN ?", articleIDs).Pluck("id", &subtreeIDs).Error; err != nil {
		return 0, err
	}

	allIDs, err := collectDescendantsIncluding(tx, append(articleIDs, subtreeIDs...))
	if err != nil {
		return 0, err
	}
	if err := deletePostsWithRelations(tx, allIDs); err != nil {
		return 0, err
	}

	return int64(len(articleIDs)), nil
}

// cleanupDeletedComments 硬删除可见文章下已删除的评论与回复。
func cleanupDeletedComments(tx *gorm.DB) (int64, error) {
	ids, err := pluckIDs(tx, "SELECT id FROM posts WHERE deleted_at IS NOT NULL AND root_id IS NOT NULL AND root_id IN ("+cleanupVisibleArticlesSQL+")")
	if err != nil {
		return 0, err
	}
	if len(ids) == 0 {
		return 0, nil
	}

	allIDs, err := collectDescendantsIncluding(tx, ids)
	if err != nil {
		return 0, err
	}
	if err := deletePostsWithRelations(tx, allIDs); err != nil {
		return 0, err
	}

	return int64(len(allIDs)), nil
}

// cleanupOrphanComments 硬删除文章或父评论已不存在的评论与回复。
func cleanupOrphanComments(tx *gorm.DB) (int64, error) {
	ids, err := pluckIDs(tx, cleanupOrphanCommentsCTE+" SELECT id FROM cleanup_orphans")
	if err != nil {
		return 0, err
	}
	if len(ids) == 0 {
		return 0, nil
	}

	allIDs, err := collectDescendantsIncluding(tx, ids)
	if err != nil {
		return 0, err
	}
	if err := deletePostsWithRelations(tx, allIDs); err != nil {
		return 0, err
	}

	return int64(len(allIDs)), nil
}

func cleanupOrphanReactions(tx *gorm.DB) (int64, error) {
	result := tx.Exec("DELETE FROM reactions WHERE post_id NOT IN (SELECT id FROM posts) OR user_id NOT IN (SELECT id FROM users)")

	return result.RowsAffected, result.Error
}

func cleanupOrphanPostTags(tx *gorm.DB) (int64, error) {
	result := tx.Exec("DELETE FROM post_tags WHERE post_id NOT IN (SELECT id FROM posts) OR tag_id NOT IN (SELECT id FROM tags)")

	return result.RowsAffected, result.Error
}

func cleanupOrphanDailyTasks(tx *gorm.DB) (int64, error) {
	result := tx.Exec("DELETE FROM daily_task_progresses WHERE user_id NOT IN (SELECT id FROM users)")

	return result.RowsAffected, result.Error
}

// deletePostsWithRelations 硬删除帖子及其反应、标签关联。
func deletePostsWithRelations(tx *gorm.DB, ids []uint) error {
	if len(ids) == 0 {
		return nil
	}

	if err := tx.Unscoped().Where("post_id IN ?", ids).Delete(&Reaction{}).Error; err != nil {
		return err
	}
	if err := tx.Exec("DELETE FROM post_tags WHERE post_id IN ?", ids).Error; err != nil {
		return err
	}

	return tx.Unscoped().Where("id IN ?", ids).Delete(&Post{}).Error
}

// collectDescendantsIncluding 返回给定帖子及其全部后代（含软删除）。
func collectDescendantsIncluding(tx *gorm.DB, roots []uint) ([]uint, error) {
	all := make([]uint, 0, len(roots))
	seen := make(map[uint]struct{}, len(roots))
	frontier := make([]uint, 0, len(roots))
	for _, id := range roots {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		all = append(all, id)
		frontier = append(frontier, id)
	}

	for len(frontier) > 0 {
		var children []uint
		if err := tx.Unscoped().Model(&Post{}).Where("parent_id IN ?", frontier).Pluck("id", &children).Error; err != nil {
			return nil, err
		}

		next := make([]uint, 0, len(children))
		for _, id := range children {
			if _, ok := seen[id]; ok {
				continue
			}
			seen[id] = struct{}{}
			all = append(all, id)
			next = append(next, id)
		}
		frontier = next
	}

	return all, nil
}

// pluckIDs 执行只返回 id 列的查询。
func pluckIDs(tx *gorm.DB, sql string) ([]uint, error) {
	var rows []struct {
		ID uint
	}
	if err := tx.Raw(sql).Scan(&rows).Error; err != nil {
		return nil, err
	}

	ids := make([]uint, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ID)
	}

	return ids, nil
}
