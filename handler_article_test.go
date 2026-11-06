package main

import (
	"net/http"
	"strings"
	"testing"
)

func TestGetMyPosts(t *testing.T) {
	env := newTestEnv(t)

	admin := env.register(t, "admin", "admin123")
	user := env.register(t, "bob", "bob12345")

	tag := env.createTag(t, admin.Token, "Go")
	first := env.createPost(t, user.Token, PostInput{Title: "first", Body: "one", TagIDs: []uint{tag.ID}})
	second := env.createPost(t, user.Token, PostInput{Title: "second", Body: "two"})
	adminPost := env.createPost(t, admin.Token, PostInput{Title: "admin post", Body: "admin"})

	// 未登录不能访问
	code, _ := env.do(t, http.MethodGet, "/api/me/posts", "", nil)
	if code != http.StatusUnauthorized {
		t.Fatalf("未登录访问我的文章应 401，实际 %d", code)
	}

	// 普通用户只看到自己的文章
	code, resp := env.do(t, http.MethodGet, "/api/me/posts", user.Token, nil)
	if code != http.StatusOK {
		t.Fatalf("我的文章应 200，实际 %d：%s", code, describe(resp))
	}
	posts := decode[[]Post](t, resp)
	if len(posts) != 2 {
		t.Fatalf("我的文章应为 2 篇，实际 %d：%+v", len(posts), posts)
	}
	seen := map[uint]bool{}
	for _, post := range posts {
		seen[post.ID] = true
	}
	if !seen[first.ID] || !seen[second.ID] || seen[adminPost.ID] {
		t.Fatalf("我的文章过滤异常：%+v", posts)
	}
	if resp.Meta == nil || resp.Meta.Total != 2 || resp.Meta.TotalPages != 1 {
		t.Fatalf("分页信息异常：%+v", resp.Meta)
	}
	if len(posts[0].Tags) != 0 && len(posts[1].Tags) != 0 {
		t.Fatalf("文章标签预加载异常：%+v", posts)
	}

	// 分页参数生效
	code, resp = env.do(t, http.MethodGet, "/api/me/posts?page=1&per_page=1", user.Token, nil)
	if code != http.StatusOK {
		t.Fatalf("分页我的文章应 200，实际 %d：%s", code, describe(resp))
	}
	pageOne := decode[[]Post](t, resp)
	if len(pageOne) != 1 || resp.Meta == nil || resp.Meta.TotalPages != 2 {
		t.Fatalf("分页结果异常：len=%d meta=%+v", len(pageOne), resp.Meta)
	}

	// 管理员也只看到自己的文章
	code, resp = env.do(t, http.MethodGet, "/api/me/posts", admin.Token, nil)
	if code != http.StatusOK {
		t.Fatalf("管理员我的文章应 200，实际 %d：%s", code, describe(resp))
	}
	adminPosts := decode[[]Post](t, resp)
	if len(adminPosts) != 1 || adminPosts[0].ID != adminPost.ID {
		t.Fatalf("管理员我的文章异常：%+v", adminPosts)
	}
}

func TestUpdatePost(t *testing.T) {
	env := newTestEnv(t)

	admin := env.register(t, "admin", "admin123")
	alice := env.register(t, "alice", "alice123")
	bob := env.register(t, "bob", "bob12345")

	oldTag := env.createTag(t, admin.Token, "Old")
	newTag := env.createTag(t, admin.Token, "New")
	post := env.createPost(t, alice.Token, PostInput{Title: "old", Subtitle: "old sub", Body: "old body", TagIDs: []uint{oldTag.ID}})
	path := "/api/post/" + idString(post.ID)

	// 未登录
	code, _ := env.do(t, http.MethodPut, path, "", PostInput{Title: "x", Body: "x"})
	if code != http.StatusUnauthorized {
		t.Fatalf("未登录编辑应 401，实际 %d", code)
	}

	// 其他用户 / 管理员都不能编辑
	for _, token := range []string{bob.Token, admin.Token} {
		code, resp := env.do(t, http.MethodPut, path, token, PostInput{Title: "hacked", Body: "hacked"})
		if code != http.StatusForbidden || resp.Error == nil || resp.Error.Code != "FORBIDDEN" {
			t.Fatalf("非作者编辑应 403 FORBIDDEN，实际 %d：%s", code, describe(resp))
		}
	}

	// ID 非法 / 不存在
	code, resp := env.do(t, http.MethodPut, "/api/post/abc", alice.Token, PostInput{Title: "x", Body: "x"})
	if code != http.StatusBadRequest || resp.Error == nil || resp.Error.Code != "INVALID_ID" {
		t.Fatalf("非法 ID 应 400 INVALID_ID，实际 %d：%s", code, describe(resp))
	}
	code, resp = env.do(t, http.MethodPut, "/api/post/99999", alice.Token, PostInput{Title: "x", Body: "x"})
	if code != http.StatusNotFound || resp.Error == nil || resp.Error.Code != "POST_NOT_FOUND" {
		t.Fatalf("不存在的文章应 404 POST_NOT_FOUND，实际 %d：%s", code, describe(resp))
	}

	// 标签校验
	code, resp = env.do(t, http.MethodPut, path, alice.Token, PostInput{Title: "x", Body: "x", TagIDs: []uint{99999}})
	if code != http.StatusBadRequest || resp.Error == nil || resp.Error.Code != "INVALID_TAGS" {
		t.Fatalf("不存在的标签应 400 INVALID_TAGS，实际 %d：%s", code, describe(resp))
	}
	tagIDs := make([]uint, 0, maxTagsPerPost+1)
	for i := 0; i < maxTagsPerPost+1; i++ {
		tagIDs = append(tagIDs, env.createTag(t, admin.Token, "Limit"+strings.Repeat("x", i)).ID)
	}
	code, resp = env.do(t, http.MethodPut, path, alice.Token, PostInput{Title: "x", Body: "x", TagIDs: tagIDs})
	if code != http.StatusBadRequest || resp.Error == nil || resp.Error.Code != "TOO_MANY_TAGS" {
		t.Fatalf("超过 5 个标签应 400 TOO_MANY_TAGS，实际 %d：%s", code, describe(resp))
	}

	// 作者正常编辑：字段和标签都更新
	code, resp = env.do(t, http.MethodPut, path, alice.Token, PostInput{
		Title: "new", Subtitle: "new sub", Body: "new body", TagIDs: []uint{newTag.ID},
	})
	if code != http.StatusOK {
		t.Fatalf("编辑文章应 200，实际 %d：%s", code, describe(resp))
	}
	updated := decode[Post](t, resp)
	if updated.Title != "new" || updated.Subtitle != "new sub" || updated.Body != "new body" || updated.User != "alice" {
		t.Fatalf("编辑后的文章字段异常：%+v", updated)
	}
	if len(updated.Tags) != 1 || updated.Tags[0].ID != newTag.ID {
		t.Fatalf("编辑后的标签异常：%+v", updated.Tags)
	}

	var oldStored, newStored Tag
	if err := env.db.First(&oldStored, oldTag.ID).Error; err != nil {
		t.Fatalf("查询旧标签失败：%v", err)
	}
	if err := env.db.First(&newStored, newTag.ID).Error; err != nil {
		t.Fatalf("查询新标签失败：%v", err)
	}
	if oldStored.UsageCount != 0 || newStored.UsageCount != 1 {
		t.Fatalf("标签使用次数校准异常：old=%d new=%d", oldStored.UsageCount, newStored.UsageCount)
	}

	// 清空标签
	code, resp = env.do(t, http.MethodPut, path, alice.Token, PostInput{Title: "new", Body: "new body"})
	if code != http.StatusOK {
		t.Fatalf("清空标签应 200，实际 %d：%s", code, describe(resp))
	}
	if err := env.db.First(&newStored, newTag.ID).Error; err != nil {
		t.Fatalf("重新查询新标签失败：%v", err)
	}
	if newStored.UsageCount != 0 {
		t.Fatalf("清空标签后使用次数应为 0，实际 %d", newStored.UsageCount)
	}
}

func TestDeletePost(t *testing.T) {
	env := newTestEnv(t)

	admin := env.register(t, "admin", "admin123")
	alice := env.register(t, "alice", "alice123")
	bob := env.register(t, "bob", "bob12345")

	tag := env.createTag(t, admin.Token, "Go")
	post := env.createPost(t, alice.Token, PostInput{Title: "to delete", Body: "body", TagIDs: []uint{tag.ID}})
	path := "/api/post/" + idString(post.ID)

	// 未登录 / 其他用户不能删除
	code, _ := env.do(t, http.MethodDelete, path, "", nil)
	if code != http.StatusUnauthorized {
		t.Fatalf("未登录删除应 401，实际 %d", code)
	}
	code, resp := env.do(t, http.MethodDelete, path, bob.Token, nil)
	if code != http.StatusForbidden || resp.Error == nil || resp.Error.Code != "FORBIDDEN" {
		t.Fatalf("非作者删除应 403 FORBIDDEN，实际 %d：%s", code, describe(resp))
	}
	// ID 非法 / 不存在
	code, resp = env.do(t, http.MethodDelete, "/api/post/abc", alice.Token, nil)
	if code != http.StatusBadRequest || resp.Error == nil || resp.Error.Code != "INVALID_ID" {
		t.Fatalf("非法 ID 应 400 INVALID_ID，实际 %d：%s", code, describe(resp))
	}
	code, resp = env.do(t, http.MethodDelete, "/api/post/99999", alice.Token, nil)
	if code != http.StatusNotFound || resp.Error == nil || resp.Error.Code != "POST_NOT_FOUND" {
		t.Fatalf("不存在的文章应 404 POST_NOT_FOUND，实际 %d：%s", code, describe(resp))
	}

	// 作者删除
	code, resp = env.do(t, http.MethodDelete, path, alice.Token, nil)
	if code != http.StatusOK {
		t.Fatalf("删除文章应 200，实际 %d：%s", code, describe(resp))
	}

	// 详情、首页、我的文章都不再返回
	code, _ = env.do(t, http.MethodGet, path, alice.Token, nil)
	if code != http.StatusNotFound {
		t.Fatalf("删除后详情应 404，实际 %d", code)
	}
	code, resp = env.do(t, http.MethodGet, "/api/post", alice.Token, nil)
	if code != http.StatusOK {
		t.Fatalf("文章列表应 200，实际 %d：%s", code, describe(resp))
	}
	if posts := decode[[]Post](t, resp); len(posts) != 0 {
		t.Fatalf("删除后首页不应有文章：%+v", posts)
	}
	code, resp = env.do(t, http.MethodGet, "/api/me/posts", alice.Token, nil)
	if code != http.StatusOK {
		t.Fatalf("我的文章应 200，实际 %d：%s", code, describe(resp))
	}
	if posts := decode[[]Post](t, resp); len(posts) != 0 {
		t.Fatalf("删除后我的文章不应有文章：%+v", posts)
	}

	// 软删除 + 标签使用次数归零
	var stored Post
	if err := env.db.Unscoped().First(&stored, post.ID).Error; err != nil {
		t.Fatalf("查询已删除文章失败：%v", err)
	}
	if !stored.DeletedAt.Valid {
		t.Fatalf("文章应为软删除，DeletedAt=%+v", stored.DeletedAt)
	}
	var storedTag Tag
	if err := env.db.First(&storedTag, tag.ID).Error; err != nil {
		t.Fatalf("查询标签失败：%v", err)
	}
	if storedTag.UsageCount != 0 {
		t.Fatalf("删除文章后标签使用次数应为 0，实际 %d", storedTag.UsageCount)
	}
}

func TestAdminDeletePost(t *testing.T) {
	env := newTestEnv(t)

	admin := env.register(t, "admin", "admin123")
	alice := env.register(t, "alice", "alice123")

	tag := env.createTag(t, admin.Token, "Go")
	post := env.createPost(t, alice.Token, PostInput{Title: "to delete", Body: "body", TagIDs: []uint{tag.ID}})
	path := "/api/post/" + idString(post.ID)

	code, resp := env.do(t, http.MethodDelete, path, admin.Token, nil)
	if code != http.StatusOK {
		t.Fatalf("管理员删除文章应 200，实际 %d：%s", code, describe(resp))
	}

	code, _ = env.do(t, http.MethodGet, path, alice.Token, nil)
	if code != http.StatusNotFound {
		t.Fatalf("删除后详情应 404，实际 %d", code)
	}
	code, resp = env.do(t, http.MethodGet, "/api/post", alice.Token, nil)
	if code != http.StatusOK {
		t.Fatalf("文章列表应 200，实际 %d：%s", code, describe(resp))
	}
	if posts := decode[[]Post](t, resp); len(posts) != 0 {
		t.Fatalf("管理员删除后首页不应有文章：%+v", posts)
	}

	var stored Post
	if err := env.db.Unscoped().First(&stored, post.ID).Error; err != nil {
		t.Fatalf("查询已删除文章失败：%v", err)
	}
	if !stored.DeletedAt.Valid {
		t.Fatalf("管理员删除也应为软删除：%+v", stored.DeletedAt)
	}
	var storedTag Tag
	if err := env.db.First(&storedTag, tag.ID).Error; err != nil {
		t.Fatalf("查询标签失败：%v", err)
	}
	if storedTag.UsageCount != 0 {
		t.Fatalf("删除后标签使用次数应为 0，实际 %d", storedTag.UsageCount)
	}
}

func TestPostVisibility(t *testing.T) {
	env := newTestEnv(t)

	admin := env.register(t, "admin", "admin123")
	alice := env.register(t, "alice", "alice123")
	bob := env.register(t, "bob", "bob12345")

	post := env.createPost(t, alice.Token, PostInput{Title: "private", Body: "body"})
	path := "/api/post/" + idString(post.ID)

	// 非管理员不能隐藏
	code, resp := env.do(t, http.MethodPatch, path+"/private", bob.Token, map[string]bool{"private": true})
	if code != http.StatusForbidden || resp.Error == nil || resp.Error.Code != "FORBIDDEN" {
		t.Fatalf("非管理员隐藏应 403 FORBIDDEN，实际 %d：%s", code, describe(resp))
	}

	// 参数校验
	code, resp = env.do(t, http.MethodPatch, path+"/private", admin.Token, map[string]any{})
	if code != http.StatusBadRequest || resp.Error == nil || resp.Error.Code != "INVALID_BODY" {
		t.Fatalf("缺少 private 应 400 INVALID_BODY，实际 %d：%s", code, describe(resp))
	}
	code, resp = env.do(t, http.MethodPatch, "/api/post/abc/private", admin.Token, map[string]bool{"private": true})
	if code != http.StatusBadRequest || resp.Error == nil || resp.Error.Code != "INVALID_ID" {
		t.Fatalf("非法 ID 应 400 INVALID_ID，实际 %d：%s", code, describe(resp))
	}
	code, resp = env.do(t, http.MethodPatch, "/api/post/99999/private", admin.Token, map[string]bool{"private": true})
	if code != http.StatusNotFound || resp.Error == nil || resp.Error.Code != "POST_NOT_FOUND" {
		t.Fatalf("不存在的文章应 404 POST_NOT_FOUND，实际 %d：%s", code, describe(resp))
	}

	// 隐藏文章
	code, resp = env.do(t, http.MethodPatch, path+"/private", admin.Token, map[string]bool{"private": true})
	if code != http.StatusOK {
		t.Fatalf("隐藏文章应 200，实际 %d：%s", code, describe(resp))
	}
	hidden := decode[Post](t, resp)
	if !hidden.IsPrivate {
		t.Fatalf("隐藏后的 private 应为 true：%+v", hidden)
	}

	list := func(token string) []Post {
		code, resp := env.do(t, http.MethodGet, "/api/post", token, nil)
		if code != http.StatusOK {
			t.Fatalf("文章列表应 200，实际 %d：%s", code, describe(resp))
		}
		return decode[[]Post](t, resp)
	}

	if posts := list(alice.Token); len(posts) != 1 || posts[0].ID != post.ID {
		t.Fatalf("作者应能看到自己的 Private 文章：%+v", posts)
	}
	if posts := list(bob.Token); len(posts) != 0 {
		t.Fatalf("其他人不应看到 Private 文章：%+v", posts)
	}
	if posts := list(admin.Token); len(posts) != 1 || posts[0].ID != post.ID {
		t.Fatalf("管理员应能看到全部 Private 文章：%+v", posts)
	}

	// 详情同理
	code, _ = env.do(t, http.MethodGet, path, alice.Token, nil)
	if code != http.StatusOK {
		t.Fatalf("作者查看 Private 详情应 200，实际 %d", code)
	}
	code, _ = env.do(t, http.MethodGet, path, bob.Token, nil)
	if code != http.StatusNotFound {
		t.Fatalf("其他人查看 Private 详情应 404，实际 %d", code)
	}
	code, _ = env.do(t, http.MethodGet, path, admin.Token, nil)
	if code != http.StatusOK {
		t.Fatalf("管理员查看 Private 详情应 200，实际 %d", code)
	}

	// 个人列表可见
	code, resp = env.do(t, http.MethodGet, "/api/me/posts", alice.Token, nil)
	if code != http.StatusOK {
		t.Fatalf("我的文章应 200，实际 %d：%s", code, describe(resp))
	}
	if posts := decode[[]Post](t, resp); len(posts) != 1 || posts[0].ID != post.ID {
		t.Fatalf("作者个人列表应包含 Private 文章：%+v", posts)
	}

	// 其他人不能对隐藏文章做出反应
	code, resp = env.do(t, http.MethodPost, path+"/reaction", bob.Token, ReactionInput{Type: ReactionLike})
	if code != http.StatusNotFound {
		t.Fatalf("其他人对 Private 文章反应应 404，实际 %d：%s", code, describe(resp))
	}
	env.react(t, alice.Token, post.ID, ReactionLike)

	// 取消隐藏后其他人恢复可见
	code, resp = env.do(t, http.MethodPatch, path+"/private", admin.Token, map[string]bool{"private": false})
	if code != http.StatusOK {
		t.Fatalf("取消隐藏应 200，实际 %d：%s", code, describe(resp))
	}
	if posts := list(bob.Token); len(posts) != 1 || posts[0].ID != post.ID {
		t.Fatalf("取消隐藏后其他人应能看到文章：%+v", posts)
	}
}

func TestComments(t *testing.T) {
	env := newTestEnv(t)

	admin := env.register(t, "admin", "admin123")
	alice := env.register(t, "alice", "alice123")
	bob := env.register(t, "bob", "bob12345")

	root := env.createPost(t, alice.Token, PostInput{Title: "article", Body: "body"})
	commentsPath := "/api/post/" + idString(root.ID) + "/comments"

	// 顶层评论
	code, resp := env.do(t, http.MethodPost, commentsPath, bob.Token, CommentInput{Body: "top level"})
	if code != http.StatusCreated {
		t.Fatalf("发表评论应 201，实际 %d：%s", code, describe(resp))
	}
	top := decode[Post](t, resp)
	if top.RootID == nil || *top.RootID != root.ID || top.ParentID != nil {
		t.Fatalf("顶层评论关系异常：%+v", top)
	}

	// 嵌套回复
	parentID := top.ID
	code, resp = env.do(t, http.MethodPost, commentsPath, alice.Token, CommentInput{Body: "nested reply", ParentID: &parentID})
	if code != http.StatusCreated {
		t.Fatalf("回复评论应 201，实际 %d：%s", code, describe(resp))
	}
	nested := decode[Post](t, resp)
	if nested.RootID == nil || *nested.RootID != root.ID || nested.ParentID == nil || *nested.ParentID != top.ID {
		t.Fatalf("嵌套评论关系异常：%+v", nested)
	}

	// 评论不进入首页/我的文章
	code, resp = env.do(t, http.MethodGet, "/api/post", bob.Token, nil)
	if code != http.StatusOK {
		t.Fatalf("首页应 200，实际 %d：%s", code, describe(resp))
	}
	homePosts := decode[[]Post](t, resp)
	if len(homePosts) != 1 || homePosts[0].ID != root.ID {
		t.Fatalf("首页不应包含评论：%+v", homePosts)
	}
	code, resp = env.do(t, http.MethodGet, "/api/me/posts", alice.Token, nil)
	if code != http.StatusOK {
		t.Fatalf("我的文章应 200，实际 %d：%s", code, describe(resp))
	}
	myPosts := decode[[]Post](t, resp)
	if len(myPosts) != 1 || myPosts[0].ID != root.ID {
		t.Fatalf("我的文章不应包含评论：%+v", myPosts)
	}

	// 评论列表
	code, resp = env.do(t, http.MethodGet, commentsPath, bob.Token, nil)
	if code != http.StatusOK {
		t.Fatalf("评论列表应 200，实际 %d：%s", code, describe(resp))
	}
	list := decode[[]Post](t, resp)
	if len(list) != 2 || list[0].ID != top.ID || list[1].ID != nested.ID {
		t.Fatalf("评论列表异常：%+v", list)
	}

	// 评论不能作为文章详情打开
	code, _ = env.do(t, http.MethodGet, "/api/post/"+idString(top.ID), bob.Token, nil)
	if code != http.StatusNotFound {
		t.Fatalf("评论详情应 404，实际 %d", code)
	}

	// 评论复用反应机制
	env.react(t, bob.Token, top.ID, ReactionLike)

	// 空评论
	code, resp = env.do(t, http.MethodPost, commentsPath, bob.Token, CommentInput{Body: "   "})
	if code != http.StatusBadRequest || resp.Error == nil || resp.Error.Code != "INVALID_BODY" {
		t.Fatalf("空评论应 400 INVALID_BODY，实际 %d：%s", code, describe(resp))
	}

	// 跨原文回复
	anotherRoot := env.createPost(t, alice.Token, PostInput{Title: "another", Body: "body"})
	anotherParent := anotherRoot.ID
	code, resp = env.do(t, http.MethodPost, commentsPath, bob.Token, CommentInput{Body: "bad parent", ParentID: &anotherParent})
	if code != http.StatusBadRequest || resp.Error == nil || resp.Error.Code != "INVALID_PARENT" {
		t.Fatalf("跨原文父评论应 400 INVALID_PARENT，实际 %d：%s", code, describe(resp))
	}

	// Private 原文的评论区只对作者/管理员可见
	code, resp = env.do(t, http.MethodPatch, "/api/post/"+idString(root.ID)+"/private", admin.Token, map[string]bool{"private": true})
	if code != http.StatusOK {
		t.Fatalf("隐藏原文应 200，实际 %d：%s", code, describe(resp))
	}

	code, _ = env.do(t, http.MethodGet, commentsPath, bob.Token, nil)
	if code != http.StatusNotFound {
		t.Fatalf("其他人读取 Private 评论区应 404，实际 %d", code)
	}
	code, _ = env.do(t, http.MethodGet, commentsPath, alice.Token, nil)
	if code != http.StatusOK {
		t.Fatalf("作者读取 Private 评论区应 200，实际 %d", code)
	}
	code, _ = env.do(t, http.MethodGet, commentsPath, admin.Token, nil)
	if code != http.StatusOK {
		t.Fatalf("管理员读取 Private 评论区应 200，实际 %d", code)
	}

	code, _ = env.do(t, http.MethodPost, commentsPath, bob.Token, CommentInput{Body: "blocked"})
	if code != http.StatusNotFound {
		t.Fatalf("其他人向 Private 原文评论应 404，实际 %d", code)
	}
	code, _ = env.do(t, http.MethodPost, "/api/post/"+idString(top.ID)+"/reaction", bob.Token, ReactionInput{Type: ReactionLike})
	if code != http.StatusNotFound {
		t.Fatalf("其他人对 Private 原文评论反应应 404，实际 %d", code)
	}
	code, resp = env.do(t, http.MethodPost, commentsPath, alice.Token, CommentInput{Body: "private reply"})
	if code != http.StatusCreated {
		t.Fatalf("作者向 Private 原文评论应 201，实际 %d：%s", code, describe(resp))
	}
}

func TestPostBodyLimitAndExcerpt(t *testing.T) {
	env := newTestEnv(t)

	admin := env.register(t, "admin", "admin123")
	user := env.register(t, "bob", "bob12345")
	_ = admin

	// 正好达到上限
	exact := strings.Repeat("a", maxPostBodyRunes)
	limitPost := env.createPost(t, user.Token, PostInput{Title: "limit", Body: exact})
	if len([]rune(limitPost.Body)) != maxPostBodyRunes {
		t.Fatalf("正文长度异常：%d", len([]rune(limitPost.Body)))
	}

	// 超过上限
	code, resp := env.do(t, http.MethodPost, "/api/post/new", user.Token, PostInput{
		Title: "too long", Body: strings.Repeat("b", maxPostBodyRunes+1),
	})
	if code != http.StatusBadRequest || resp.Error == nil || resp.Error.Code != "BODY_TOO_LONG" {
		t.Fatalf("超长正文应 400 BODY_TOO_LONG，实际 %d：%s", code, describe(resp))
	}

	body := "# Heading\n\n**bold** [site](https://example.com) and `code`\n\n" + strings.Repeat("word ", 100)
	code, resp = env.do(t, http.MethodPost, "/api/post/new", user.Token, PostInput{Title: "markdown", Body: body})
	if code != http.StatusCreated {
		t.Fatalf("创建 Markdown 文章应 201，实际 %d：%s", code, describe(resp))
	}
	created := decode[Post](t, resp)
	if created.Excerpt == "" || strings.Contains(created.Excerpt, "#") || strings.Contains(created.Excerpt, "http") {
		t.Fatalf("摘要生成异常：%q", created.Excerpt)
	}
	if len([]rune(created.Excerpt)) > excerptLength+1 {
		t.Fatalf("摘要长度异常：%d", len([]rune(created.Excerpt)))
	}

	// 列表不返回正文，只返回摘要
	code, resp = env.do(t, http.MethodGet, "/api/post", user.Token, nil)
	if code != http.StatusOK {
		t.Fatalf("文章列表应 200，实际 %d：%s", code, describe(resp))
	}
	posts := decode[[]Post](t, resp)
	var listed Post
	found := false
	for _, post := range posts {
		if post.ID == created.ID {
			listed = post
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("列表中没有新文章：%+v", posts)
	}
	if listed.Body != "" {
		t.Fatalf("列表不应返回正文，实际 %d 字节", len(listed.Body))
	}
	if listed.Excerpt == "" {
		t.Fatalf("列表应返回摘要")
	}

	// 详情返回完整正文
	code, resp = env.do(t, http.MethodGet, "/api/post/"+idString(created.ID), user.Token, nil)
	if code != http.StatusOK {
		t.Fatalf("文章详情应 200，实际 %d：%s", code, describe(resp))
	}
	detail := decode[Post](t, resp)
	if detail.Body != body {
		t.Fatalf("详情正文不完整：got %d bytes, want %d", len(detail.Body), len(body))
	}

	// 更新后摘要同步刷新
	code, resp = env.do(t, http.MethodPut, "/api/post/"+idString(created.ID), user.Token, PostInput{
		Title: "updated", Body: "## New heading\n\nnew summary",
	})
	if code != http.StatusOK {
		t.Fatalf("更新文章应 200，实际 %d：%s", code, describe(resp))
	}
	updated := decode[Post](t, resp)
	if updated.Excerpt == "" || strings.Contains(updated.Excerpt, "#") {
		t.Fatalf("更新后的摘要异常：%q", updated.Excerpt)
	}
}

func TestDeleteComment(t *testing.T) {
	env := newTestEnv(t)

	admin := env.register(t, "admin", "admin123")
	alice := env.register(t, "alice", "alice123")
	bob := env.register(t, "bob", "bob12345")

	root := env.createPost(t, alice.Token, PostInput{Title: "article", Body: "body"})
	commentsPath := "/api/post/" + idString(root.ID) + "/comments"

	code, resp := env.do(t, http.MethodPost, commentsPath, bob.Token, CommentInput{Body: "top"})
	if code != http.StatusCreated {
		t.Fatalf("发表评论应 201，实际 %d：%s", code, describe(resp))
	}
	top := decode[Post](t, resp)

	parentID := top.ID
	code, resp = env.do(t, http.MethodPost, commentsPath, alice.Token, CommentInput{Body: "reply", ParentID: &parentID})
	if code != http.StatusCreated {
		t.Fatalf("发表回复应 201，实际 %d：%s", code, describe(resp))
	}
	nested := decode[Post](t, resp)

	// 非作者不能删除（alice 是原文作者，但不是这条评论作者）
	code, resp = env.do(t, http.MethodDelete, "/api/post/"+idString(top.ID), alice.Token, nil)
	if code != http.StatusForbidden || resp.Error == nil || resp.Error.Code != "FORBIDDEN" {
		t.Fatalf("非作者删除评论应 403 FORBIDDEN，实际 %d：%s", code, describe(resp))
	}

	// 作者删除父评论，子回复应级联软删除
	code, resp = env.do(t, http.MethodDelete, "/api/post/"+idString(top.ID), bob.Token, nil)
	if code != http.StatusOK {
		t.Fatalf("作者删除评论应 200，实际 %d：%s", code, describe(resp))
	}

	code, resp = env.do(t, http.MethodGet, commentsPath, bob.Token, nil)
	if code != http.StatusOK {
		t.Fatalf("评论列表应 200，实际 %d：%s", code, describe(resp))
	}
	if comments := decode[[]Post](t, resp); len(comments) != 0 {
		t.Fatalf("删除父评论后不应还有回复：%+v", comments)
	}

	var storedTop, storedNested Post
	if err := env.db.Unscoped().First(&storedTop, top.ID).Error; err != nil {
		t.Fatalf("查询顶层评论失败：%v", err)
	}
	if err := env.db.Unscoped().First(&storedNested, nested.ID).Error; err != nil {
		t.Fatalf("查询回复失败：%v", err)
	}
	if !storedTop.DeletedAt.Valid || !storedNested.DeletedAt.Valid {
		t.Fatalf("父评论和回复都应为软删除：top=%+v nested=%+v", storedTop.DeletedAt, storedNested.DeletedAt)
	}

	// 管理员可以删除其他人的评论
	code, resp = env.do(t, http.MethodPost, commentsPath, bob.Token, CommentInput{Body: "admin delete me"})
	if code != http.StatusCreated {
		t.Fatalf("发表第二条评论应 201，实际 %d：%s", code, describe(resp))
	}
	second := decode[Post](t, resp)
	code, resp = env.do(t, http.MethodDelete, "/api/post/"+idString(second.ID), admin.Token, nil)
	if code != http.StatusOK {
		t.Fatalf("管理员删除评论应 200，实际 %d：%s", code, describe(resp))
	}
	code, resp = env.do(t, http.MethodGet, commentsPath, bob.Token, nil)
	if code != http.StatusOK {
		t.Fatalf("评论列表应 200，实际 %d：%s", code, describe(resp))
	}
	if comments := decode[[]Post](t, resp); len(comments) != 0 {
		t.Fatalf("管理员删除后评论列表应为空：%+v", comments)
	}
}
