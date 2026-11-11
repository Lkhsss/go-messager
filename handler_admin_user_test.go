package main

import (
	"net/http"
	"testing"
)

func findAdminUser(items []AdminUserListItem, name string) *AdminUserListItem {
	for i := range items {
		if items[i].Name == name {
			return &items[i]
		}
	}
	return nil
}

func TestAdminUserManagement(t *testing.T) {
	env := newTestEnv(t)

	admin := env.register(t, "admin", "admin123")
	alice := env.register(t, "alice", "alice123")
	bob := env.register(t, "bob", "bob12345")

	var adminRecord User
	if err := env.db.Where("name = ?", "admin").Take(&adminRecord).Error; err != nil {
		t.Fatalf("查询管理员失败：%v", err)
	}

	// alice 发一篇文章；bob 评论；alice 再评论 bob 的文章。
	alicePost := env.createPost(t, alice.Token, PostInput{Title: "article", Body: "body"})
	bobPost := env.createPost(t, bob.Token, PostInput{Title: "bob article", Body: "body"})
	code, resp := env.do(t, http.MethodPost, "/api/post/"+idString(alicePost.ID)+"/comments", bob.Token, CommentInput{Body: "bob comment"})
	if code != http.StatusCreated {
		t.Fatalf("bob 评论应 201，实际 %d：%s", code, describe(resp))
	}
	code, resp = env.do(t, http.MethodPost, "/api/post/"+idString(bobPost.ID)+"/comments", alice.Token, CommentInput{Body: "alice comment"})
	if code != http.StatusCreated {
		t.Fatalf("alice 评论应 201，实际 %d：%s", code, describe(resp))
	}
	env.react(t, bob.Token, alicePost.ID, ReactionLike)

	// 列表分页
	code, resp = env.do(t, http.MethodGet, "/api/admin/users?page=1&per_page=2", admin.Token, nil)
	if code != http.StatusOK {
		t.Fatalf("管理员用户列表应 200，实际 %d：%s", code, describe(resp))
	}
	if resp.Meta == nil || resp.Meta.Total != 3 || resp.Meta.TotalPages != 2 {
		t.Fatalf("用户列表分页信息异常：%+v", resp.Meta)
	}
	firstPage := decode[[]AdminUserListItem](t, resp)
	if len(firstPage) != 2 {
		t.Fatalf("第一页应有 2 位用户，实际 %d", len(firstPage))
	}

	code, resp = env.do(t, http.MethodGet, "/api/admin/users?page=2&per_page=2", admin.Token, nil)
	if code != http.StatusOK {
		t.Fatalf("第二页应 200，实际 %d：%s", code, describe(resp))
	}
	secondPage := decode[[]AdminUserListItem](t, resp)
	if len(secondPage) != 1 {
		t.Fatalf("第二页应有 1 位用户，实际 %d", len(secondPage))
	}

	// 搜索
	code, resp = env.do(t, http.MethodGet, "/api/admin/users?q=ali", admin.Token, nil)
	if code != http.StatusOK {
		t.Fatalf("搜索用户应 200，实际 %d：%s", code, describe(resp))
	}
	searchResult := decode[[]AdminUserListItem](t, resp)
	if len(searchResult) != 1 || searchResult[0].Name != "alice" {
		t.Fatalf("搜索 alice 结果异常：%+v", searchResult)
	}

	// 列表中的关键数据
	aliceItem := findAdminUser(searchResult, "alice")
	if aliceItem == nil {
		t.Fatal("搜索结果中没有 alice")
	}
	if aliceItem.PostCount != 1 || aliceItem.CommentCount != 1 || aliceItem.Like != 1 {
		t.Fatalf("alice 列表统计异常：%+v", aliceItem)
	}
	if aliceItem.LastActiveAt == nil {
		t.Fatal("alice 的上次活跃时间不应为空")
	}

	// 详情统计必须与 /api/me/stats 完全一致
	code, resp = env.do(t, http.MethodGet, "/api/admin/users/"+idString(aliceItem.ID), admin.Token, nil)
	if code != http.StatusOK {
		t.Fatalf("用户详情应 200，实际 %d：%s", code, describe(resp))
	}
	detail := decode[AdminUserDetailResponse](t, resp)
	code, resp = env.do(t, http.MethodGet, "/api/me/stats", alice.Token, nil)
	if code != http.StatusOK {
		t.Fatalf("个人统计应 200，实际 %d：%s", code, describe(resp))
	}
	myStats := decode[UserStatsResponse](t, resp)
	if detail.Stats != myStats {
		t.Fatalf("管理员详情统计与个人统计不一致：%+v vs %+v", detail.Stats, myStats)
	}
	if detail.Stats.PostCount != 1 || detail.Stats.CommentCount != 1 || detail.Stats.ReceivedCommentCount != 1 {
		t.Fatalf("详情统计异常：%+v", detail.Stats)
	}
	if detail.User.LastActiveAt == nil {
		t.Fatal("详情中的上次活跃时间不应为空")
	}

	// 普通用户无权访问用户管理
	code, _ = env.do(t, http.MethodGet, "/api/admin/users", alice.Token, nil)
	if code != http.StatusForbidden {
		t.Fatalf("普通用户访问用户列表应 403，实际 %d", code)
	}
	code, _ = env.do(t, http.MethodGet, "/api/admin/users", "", nil)
	if code != http.StatusUnauthorized {
		t.Fatalf("未登录访问用户列表应 401，实际 %d", code)
	}

	// 封禁后立即失效
	code, resp = env.do(t, http.MethodPatch, "/api/admin/users/"+idString(aliceItem.ID)+"/status", admin.Token, adminUserStatusInput{Disabled: true})
	if code != http.StatusOK {
		t.Fatalf("封禁用户应 200，实际 %d：%s", code, describe(resp))
	}
	code, resp = env.do(t, http.MethodGet, "/api/me", alice.Token, nil)
	if code != http.StatusForbidden || resp.Error == nil || resp.Error.Code != "ACCOUNT_DISABLED" {
		t.Fatalf("封禁后应 403 ACCOUNT_DISABLED，实际 %d：%s", code, describe(resp))
	}
	code, resp = env.do(t, http.MethodPost, "/api/authorize", "", Credentials{Name: "alice", Password: "alice123"})
	if code != http.StatusForbidden || resp.Error == nil || resp.Error.Code != "ACCOUNT_DISABLED" {
		t.Fatalf("封禁后登录应 403 ACCOUNT_DISABLED，实际 %d：%s", code, describe(resp))
	}

	// 解封
	code, resp = env.do(t, http.MethodPatch, "/api/admin/users/"+idString(aliceItem.ID)+"/status", admin.Token, adminUserStatusInput{Disabled: false})
	if code != http.StatusOK {
		t.Fatalf("解封用户应 200，实际 %d：%s", code, describe(resp))
	}
	code, resp = env.do(t, http.MethodGet, "/api/me", alice.Token, nil)
	if code != http.StatusOK {
		t.Fatalf("解封后应恢复访问，实际 %d：%s", code, describe(resp))
	}

	// 管理员不能被封禁/删除
	code, resp = env.do(t, http.MethodPatch, "/api/admin/users/"+idString(adminRecord.ID)+"/status", admin.Token, adminUserStatusInput{Disabled: true})
	if code != http.StatusBadRequest {
		t.Fatalf("封禁管理员应 400，实际 %d：%s", code, describe(resp))
	}

	// 修改密码
	code, resp = env.do(t, http.MethodPut, "/api/admin/users/"+idString(aliceItem.ID)+"/password", admin.Token, adminUserPasswordInput{Password: "newpass123"})
	if code != http.StatusOK {
		t.Fatalf("修改密码应 200，实际 %d：%s", code, describe(resp))
	}
	code, resp = env.do(t, http.MethodPost, "/api/authorize", "", Credentials{Name: "alice", Password: "alice123"})
	if code != http.StatusUnauthorized {
		t.Fatalf("旧密码登录应 401，实际 %d：%s", code, describe(resp))
	}
	code, resp = env.do(t, http.MethodPost, "/api/authorize", "", Credentials{Name: "alice", Password: "newpass123"})
	if code != http.StatusOK {
		t.Fatalf("新密码登录应 200，实际 %d：%s", code, describe(resp))
	}

	// 删除账号：文章/评论保留，账号无法再登录，名字不能被复用
	code, resp = env.do(t, http.MethodDelete, "/api/admin/users/"+idString(aliceItem.ID), admin.Token, nil)
	if code != http.StatusOK {
		t.Fatalf("删除用户应 200，实际 %d：%s", code, describe(resp))
	}
	code, _ = env.do(t, http.MethodGet, "/api/me", alice.Token, nil)
	if code != http.StatusUnauthorized {
		t.Fatalf("删除账号后旧令牌应 401，实际 %d", code)
	}
	code, _ = env.do(t, http.MethodDelete, "/api/admin/users/"+idString(aliceItem.ID), admin.Token, nil)
	if code != http.StatusNotFound {
		t.Fatalf("重复删除应 404，实际 %d", code)
	}

	var alicePostCount int64
	if err := env.db.Model(&Post{}).Where("user = ?", "alice").Count(&alicePostCount).Error; err != nil {
		t.Fatalf("查询保留内容失败：%v", err)
	}
	if alicePostCount != 2 {
		t.Fatalf("删除账号后应保留 alice 的 1 篇文章和 1 条评论，实际 %d", alicePostCount)
	}
	code, resp = env.do(t, http.MethodPost, "/api/register", "", Credentials{Name: "alice", Password: "another123"})
	if code != http.StatusConflict || resp.Error == nil || resp.Error.Code != "USER_EXIST" {
		t.Fatalf("删除账号后用户名不应可复用，实际 %d：%s", code, describe(resp))
	}

	// 管理员不能删除自己
	code, resp = env.do(t, http.MethodDelete, "/api/admin/users/"+idString(adminRecord.ID), admin.Token, nil)
	if code != http.StatusBadRequest {
		t.Fatalf("删除管理员应 400，实际 %d：%s", code, describe(resp))
	}
}

func TestAdminClearUserPosts(t *testing.T) {
	env := newTestEnv(t)

	admin := env.register(t, "admin", "admin123")
	bob := env.register(t, "bob", "bob12345")
	alice := env.register(t, "alice", "alice123")

	bobPost := env.createPost(t, bob.Token, PostInput{Title: "违规文章", Body: "body"})
	bobPost2 := env.createPost(t, bob.Token, PostInput{Title: "违规文章 2", Body: "body"})
	alicePost := env.createPost(t, alice.Token, PostInput{Title: "正常文章", Body: "body"})

	// bob 文章下的评论与回复。
	code, resp := env.do(t, http.MethodPost, "/api/post/"+idString(bobPost.ID)+"/comments", alice.Token,
		CommentInput{Body: "评论"})
	if code != http.StatusCreated {
		t.Fatalf("评论应 201，实际 %d：%s", code, describe(resp))
	}
	comment := decode[Post](t, resp)

	code, resp = env.do(t, http.MethodPost, "/api/post/"+idString(bobPost.ID)+"/comments", bob.Token,
		CommentInput{Body: "回复", ParentID: &comment.ID})
	if code != http.StatusCreated {
		t.Fatalf("回复应 201，实际 %d：%s", code, describe(resp))
	}

	// bob 在 alice 文章下的评论应保留。
	code, resp = env.do(t, http.MethodPost, "/api/post/"+idString(alicePost.ID)+"/comments", bob.Token,
		CommentInput{Body: "他人文章下的评论"})
	if code != http.StatusCreated {
		t.Fatalf("alice 文章评论应 201，实际 %d：%s", code, describe(resp))
	}

	var bobRecord User
	if err := env.db.Where("name = ?", "bob").Take(&bobRecord).Error; err != nil {
		t.Fatalf("查询 bob 失败：%v", err)
	}

	// 普通用户无权清空他人文章。
	code, resp = env.do(t, http.MethodDelete, "/api/admin/users/"+idString(bobRecord.ID)+"/posts", alice.Token, nil)
	if code != http.StatusForbidden {
		t.Fatalf("普通用户清空文章应 403，实际 %d：%s", code, describe(resp))
	}

	// 管理员清空 bob 的文章。
	code, resp = env.do(t, http.MethodDelete, "/api/admin/users/"+idString(bobRecord.ID)+"/posts", admin.Token, nil)
	if code != http.StatusOK {
		t.Fatalf("清空文章应 200，实际 %d：%s", code, describe(resp))
	}
	result := decode[struct {
		Deleted  int   `json:"deleted"`
		Comments int64 `json:"comments"`
	}](t, resp)
	if result.Deleted != 2 || result.Comments != 2 {
		t.Fatalf("清空结果异常：%+v", result)
	}

	// bob 的文章及文章下的评论都不再可见。
	for _, path := range []string{
		"/api/post/" + idString(bobPost.ID),
		"/api/post/" + idString(bobPost2.ID),
	} {
		if code, resp = env.do(t, http.MethodGet, path, bob.Token, nil); code != http.StatusNotFound {
			t.Fatalf("%s 应 404，实际 %d：%s", path, code, describe(resp))
		}
	}
	if code, resp = env.do(t, http.MethodGet, "/api/post/"+idString(bobPost.ID)+"/comments", bob.Token, nil); code != http.StatusNotFound {
		t.Fatalf("bob 文章评论应 404，实际 %d：%s", code, describe(resp))
	}

	// alice 的文章保留，bob 在其中的评论也保留。
	if code, resp = env.do(t, http.MethodGet, "/api/post/"+idString(alicePost.ID), alice.Token, nil); code != http.StatusOK {
		t.Fatalf("alice 文章应保留，实际 %d：%s", code, describe(resp))
	}
	code, resp = env.do(t, http.MethodGet, "/api/post/"+idString(alicePost.ID)+"/comments", alice.Token, nil)
	if code != http.StatusOK {
		t.Fatalf("alice 文章评论应 200，实际 %d：%s", code, describe(resp))
	}
	if kept := decode[[]Post](t, resp); len(kept) != 1 {
		t.Fatalf("alice 文章应保留 1 条评论，实际 %d：%+v", len(kept), kept)
	}

	// 用户列表统计同步更新：文章归零，评论只剩他人文章下的 1 条。
	code, resp = env.do(t, http.MethodGet, "/api/admin/users?q=bob", admin.Token, nil)
	if code != http.StatusOK {
		t.Fatalf("用户列表应 200，实际 %d：%s", code, describe(resp))
	}
	bobItem := findAdminUser(decode[[]AdminUserListItem](t, resp), "bob")
	if bobItem == nil || bobItem.PostCount != 0 || bobItem.CommentCount != 1 {
		t.Fatalf("bob 统计异常：%+v", bobItem)
	}

	// 管理员账号的文章不能被清空。
	var adminRecord User
	if err := env.db.Where("name = ?", "admin").Take(&adminRecord).Error; err != nil {
		t.Fatalf("查询管理员失败：%v", err)
	}
	code, resp = env.do(t, http.MethodDelete, "/api/admin/users/"+idString(adminRecord.ID)+"/posts", admin.Token, nil)
	if code != http.StatusBadRequest {
		t.Fatalf("清空管理员文章应 400，实际 %d：%s", code, describe(resp))
	}
}
