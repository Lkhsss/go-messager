package main

import (
	"net/http"
	"testing"
)

func findCleanupCategory(stats CleanupStatsResponse, key string) *CleanupCategory {
	for i := range stats.Categories {
		if stats.Categories[i].Key == key {
			return &stats.Categories[i]
		}
	}
	return nil
}

// fetchCleanupStats 以管理员身份读取清理统计。
func fetchCleanupStats(t *testing.T, env *testEnv, token string) CleanupStatsResponse {
	t.Helper()

	code, resp := env.do(t, http.MethodGet, "/api/admin/cleanup", token, nil)
	if code != http.StatusOK {
		t.Fatalf("清理统计应 200，实际 %d：%s", code, describe(resp))
	}

	return decode[CleanupStatsResponse](t, resp)
}

func TestAdminCleanupStatsAndDelete(t *testing.T) {
	env := newTestEnv(t)

	admin := env.register(t, "admin", "admin123")
	bob := env.register(t, "bob", "bob12345")

	tag := env.createTag(t, admin.Token, "Go")

	// 可见文章 + 评论（清理后必须保留）。
	visible := env.createPost(t, bob.Token, PostInput{Title: "visible", Body: "visible body", TagIDs: []uint{tag.ID}})
	code, resp := env.do(t, http.MethodPost, "/api/post/"+idString(visible.ID)+"/comments", bob.Token,
		CommentInput{Body: "keep me"})
	if code != http.StatusCreated {
		t.Fatalf("可见文章评论应 201，实际 %d：%s", code, describe(resp))
	}
	keepComment := decode[Post](t, resp)

	// 已删除文章：走真实接口软删除，其下评论保持原样。
	doomed := env.createPost(t, bob.Token, PostInput{Title: "doomed", Body: "doomed body", TagIDs: []uint{tag.ID}})
	code, resp = env.do(t, http.MethodPost, "/api/post/"+idString(doomed.ID)+"/comments", bob.Token,
		CommentInput{Body: "doomed comment"})
	if code != http.StatusCreated {
		t.Fatalf("已删除文章评论应 201，实际 %d：%s", code, describe(resp))
	}
	code, resp = env.do(t, http.MethodDelete, "/api/post/"+idString(doomed.ID), bob.Token, nil)
	if code != http.StatusOK {
		t.Fatalf("删除文章应 200，实际 %d：%s", code, describe(resp))
	}

	// 已删除评论：文章仍可见。
	removable := env.createPost(t, bob.Token, PostInput{Title: "removable", Body: "removable body"})
	code, resp = env.do(t, http.MethodPost, "/api/post/"+idString(removable.ID)+"/comments", bob.Token,
		CommentInput{Body: "delete me"})
	if code != http.StatusCreated {
		t.Fatalf("待删除评论应 201，实际 %d：%s", code, describe(resp))
	}
	deletedComment := decode[Post](t, resp)
	code, resp = env.do(t, http.MethodDelete, "/api/post/"+idString(deletedComment.ID), bob.Token, nil)
	if code != http.StatusOK {
		t.Fatalf("删除评论应 200，实际 %d：%s", code, describe(resp))
	}

	// 孤立评论：文章行被彻底删除后，评论成为孤儿。
	orphanRoot := env.createPost(t, bob.Token, PostInput{Title: "orphan root", Body: "orphan body"})
	code, resp = env.do(t, http.MethodPost, "/api/post/"+idString(orphanRoot.ID)+"/comments", bob.Token,
		CommentInput{Body: "orphan comment"})
	if code != http.StatusCreated {
		t.Fatalf("孤立评论应 201，实际 %d：%s", code, describe(resp))
	}
	if err := env.db.Unscoped().Where("id = ?", orphanRoot.ID).Delete(&Post{}).Error; err != nil {
		t.Fatalf("彻底删除文章失败：%v", err)
	}

	// 孤儿反应与孤立标签关联：文章带标签并收到反应后被彻底删除。
	orphanPost := env.createPost(t, bob.Token, PostInput{Title: "orphan post", Body: "orphan post body", TagIDs: []uint{tag.ID}})
	env.react(t, admin.Token, orphanPost.ID, ReactionLike)
	if err := env.db.Unscoped().Where("id = ?", orphanPost.ID).Delete(&Post{}).Error; err != nil {
		t.Fatalf("彻底删除带标签文章失败：%v", err)
	}

	// 孤立每日任务进度：用户已不存在。
	if err := env.db.Create(&DailyTaskProgress{UserID: 999999, Date: "2026-01-01"}).Error; err != nil {
		t.Fatalf("写入孤立每日任务失败：%v", err)
	}

	// 普通用户无权访问清理接口。
	if code, resp = env.do(t, http.MethodGet, "/api/admin/cleanup", bob.Token, nil); code != http.StatusForbidden {
		t.Fatalf("普通用户查看清理统计应 403，实际 %d：%s", code, describe(resp))
	}
	if code, resp = env.do(t, http.MethodDelete, "/api/admin/cleanup/"+cleanupKeyDeletedPosts, bob.Token, nil); code != http.StatusForbidden {
		t.Fatalf("普通用户清理数据应 403，实际 %d：%s", code, describe(resp))
	}
	if code, resp = env.do(t, http.MethodPost, "/api/admin/cleanup/vacuum", bob.Token, nil); code != http.StatusForbidden {
		t.Fatalf("普通用户压缩数据库应 403，实际 %d：%s", code, describe(resp))
	}

	stats := fetchCleanupStats(t, env, admin.Token)
	if len(stats.Categories) != len(cleanupCategoryDefs) {
		t.Fatalf("分类数量应为 %d，实际 %d", len(cleanupCategoryDefs), len(stats.Categories))
	}
	if stats.Database.PageSize <= 0 || stats.Database.FileSizeBytes <= 0 {
		t.Fatalf("数据库占用信息异常：%+v", stats.Database)
	}

	expect := map[string]int64{
		cleanupKeyDeletedPosts:    1,
		cleanupKeyDeletedComments: 1,
		cleanupKeyOrphanComments:  1,
		cleanupKeyOrphanReactions: 1,
		cleanupKeyOrphanPostTags:  1,
		cleanupKeyOrphanDaily:     1,
	}
	for key, want := range expect {
		category := findCleanupCategory(stats, key)
		if category == nil {
			t.Fatalf("统计结果缺少分类 %s", key)
		}
		if category.Count != want {
			t.Fatalf("分类 %s 数量应为 %d，实际 %d", key, want, category.Count)
		}
		if category.Bytes <= 0 {
			t.Fatalf("分类 %s 估算占用应大于 0", key)
		}
	}
	if stats.TotalCount != 6 {
		t.Fatalf("可清理条目合计应为 6，实际 %d", stats.TotalCount)
	}

	// 未知分类返回 400。
	if code, resp = env.do(t, http.MethodDelete, "/api/admin/cleanup/unknown", admin.Token, nil); code != http.StatusBadRequest {
		t.Fatalf("未知分类应 400，实际 %d：%s", code, describe(resp))
	}

	// 逐类清理，删除数量应与统计一致。
	for _, def := range cleanupCategoryDefs {
		code, resp = env.do(t, http.MethodDelete, "/api/admin/cleanup/"+def.Key, admin.Token, nil)
		if code != http.StatusOK {
			t.Fatalf("清理 %s 应 200，实际 %d：%s", def.Key, code, describe(resp))
		}
		result := decode[CleanupResult](t, resp)
		if result.Deleted != expect[def.Key] {
			t.Fatalf("清理 %s 删除数量应为 %d，实际 %d", def.Key, expect[def.Key], result.Deleted)
		}
	}

	// 清理后所有分类归零。
	stats = fetchCleanupStats(t, env, admin.Token)
	if stats.TotalCount != 0 || stats.TotalBytes != 0 {
		t.Fatalf("清理后统计应归零，实际 %+v", stats)
	}
	for _, category := range stats.Categories {
		if category.Count != 0 {
			t.Fatalf("清理后分类 %s 仍有 %d 条", category.Key, category.Count)
		}
	}

	// 可见文章与评论不受影响。
	if code, resp = env.do(t, http.MethodGet, "/api/post/"+idString(visible.ID), bob.Token, nil); code != http.StatusOK {
		t.Fatalf("可见文章应保留，实际 %d：%s", code, describe(resp))
	}
	code, resp = env.do(t, http.MethodGet, "/api/post/"+idString(visible.ID)+"/comments", bob.Token, nil)
	if code != http.StatusOK {
		t.Fatalf("可见文章评论应 200，实际 %d：%s", code, describe(resp))
	}
	if comments := decode[[]Post](t, resp); len(comments) != 1 || comments[0].ID != keepComment.ID {
		t.Fatalf("可见文章评论应保留 1 条，实际 %+v", comments)
	}
	if code, resp = env.do(t, http.MethodGet, "/api/post/"+idString(removable.ID), bob.Token, nil); code != http.StatusOK {
		t.Fatalf("被删除评论所属文章应保留，实际 %d：%s", code, describe(resp))
	}

	// 标签使用次数已重新校准：只剩 visible 一篇带标签。
	code, resp = env.do(t, http.MethodGet, "/api/tag", bob.Token, nil)
	if code != http.StatusOK {
		t.Fatalf("标签列表应 200，实际 %d：%s", code, describe(resp))
	}
	usage := int64(-1)
	for _, item := range decode[[]Tag](t, resp) {
		if item.ID == tag.ID {
			usage = item.UsageCount
		}
	}
	if usage != 1 {
		t.Fatalf("标签使用次数应为 1，实际 %d", usage)
	}

	// 再次一键清理：没有可清理的数据。
	code, resp = env.do(t, http.MethodDelete, "/api/admin/cleanup", admin.Token, nil)
	if code != http.StatusOK {
		t.Fatalf("一键清理应 200，实际 %d：%s", code, describe(resp))
	}
	for _, result := range decode[[]CleanupResult](t, resp) {
		if result.Deleted != 0 {
			t.Fatalf("重复清理 %s 不应删除数据，实际 %d", result.Key, result.Deleted)
		}
	}

	// 压缩数据库。
	code, resp = env.do(t, http.MethodPost, "/api/admin/cleanup/vacuum", admin.Token, nil)
	if code != http.StatusOK {
		t.Fatalf("压缩数据库应 200，实际 %d：%s", code, describe(resp))
	}
	vacuum := decode[struct {
		BeforeBytes int64 `json:"before_bytes"`
		AfterBytes  int64 `json:"after_bytes"`
		FreedBytes  int64 `json:"freed_bytes"`
		FreeBytes   int64 `json:"free_bytes"`
	}](t, resp)
	if vacuum.BeforeBytes <= 0 || vacuum.AfterBytes <= 0 {
		t.Fatalf("压缩前后大小异常：%+v", vacuum)
	}
}

func TestAdminCleanupBulkDelete(t *testing.T) {
	env := newTestEnv(t)

	admin := env.register(t, "admin", "admin123")
	bob := env.register(t, "bob", "bob12345")

	doomed := env.createPost(t, bob.Token, PostInput{Title: "doomed", Body: "doomed body"})
	if code, resp := env.do(t, http.MethodDelete, "/api/post/"+idString(doomed.ID), bob.Token, nil); code != http.StatusOK {
		t.Fatalf("删除文章应 200，实际 %d：%s", code, describe(resp))
	}
	if err := env.db.Create(&DailyTaskProgress{UserID: 888888, Date: "2026-02-02"}).Error; err != nil {
		t.Fatalf("写入孤立每日任务失败：%v", err)
	}

	code, resp := env.do(t, http.MethodDelete, "/api/admin/cleanup", admin.Token, nil)
	if code != http.StatusOK {
		t.Fatalf("一键清理应 200，实际 %d：%s", code, describe(resp))
	}
	deletedByKey := map[string]int64{}
	for _, result := range decode[[]CleanupResult](t, resp) {
		deletedByKey[result.Key] = result.Deleted
	}
	if deletedByKey[cleanupKeyDeletedPosts] != 1 {
		t.Fatalf("一键清理应删除 1 篇已删除文章，实际 %d", deletedByKey[cleanupKeyDeletedPosts])
	}
	if deletedByKey[cleanupKeyOrphanDaily] != 1 {
		t.Fatalf("一键清理应删除 1 条孤立每日任务，实际 %d", deletedByKey[cleanupKeyOrphanDaily])
	}
	if stats := fetchCleanupStats(t, env, admin.Token); stats.TotalCount != 0 {
		t.Fatalf("一键清理后统计应归零，实际 %+v", stats)
	}
}
