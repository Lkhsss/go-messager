package main

import (
	"net/http"
	"testing"
)

type statusPayload struct {
	Initialized bool   `json:"initialized"`
	Mode        string `json:"mode"`
}

func TestStatusIncludesAccessMode(t *testing.T) {
	tests := []struct {
		name string
		mode string
	}{
		{name: "隐私模式", mode: AccessModePrivate},
		{name: "公开模式", mode: AccessModePublic},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := newTestEnvWithAccessMode(t, tt.mode)

			code, resp := env.do(t, http.MethodGet, "/api/status", "", nil)
			if code != http.StatusOK {
				t.Fatalf("系统状态应 200，实际 %d：%s", code, describe(resp))
			}

			payload := decode[statusPayload](t, resp)
			if payload.Mode != tt.mode {
				t.Fatalf("access mode 应为 %s，实际 %s", tt.mode, payload.Mode)
			}
		})
	}
}

func TestPrivateModeRequiresLoginForReads(t *testing.T) {
	env := newTestEnv(t)

	paths := []string{
		"/api/post",
		"/api/post/1",
		"/api/post/1/comments",
		"/api/recent-post-count",
		"/api/tag",
		"/api/leaderboard",
	}

	for _, path := range paths {
		code, resp := env.do(t, http.MethodGet, path, "", nil)
		if code != http.StatusUnauthorized {
			t.Fatalf("隐私模式未登录访问 %s 应 401，实际 %d：%s", path, code, describe(resp))
		}
	}
}

func TestPublicModeAnonymousRead(t *testing.T) {
	env := newTestEnvWithAccessMode(t, AccessModePublic)

	admin := env.register(t, "admin", "admin123")
	user := env.register(t, "bob", "bob12345")

	env.createTag(t, admin.Token, "Go")
	publicPost := env.createPost(t, user.Token, PostInput{Title: "public", Body: "public body"})
	hiddenPost := env.createPost(t, user.Token, PostInput{Title: "hidden", Body: "hidden body"})

	code, resp := env.do(
		t, http.MethodPatch, "/api/post/"+idString(hiddenPost.ID)+"/private",
		admin.Token, map[string]bool{"private": true},
	)
	if code != http.StatusOK {
		t.Fatalf("隐藏文章应 200，实际 %d：%s", code, describe(resp))
	}

	// 游客可以看到公开文章，看不到被隐藏的文章。
	code, resp = env.do(t, http.MethodGet, "/api/post", "", nil)
	if code != http.StatusOK {
		t.Fatalf("公开模式游客查看文章列表应 200，实际 %d：%s", code, describe(resp))
	}
	posts := decode[[]Post](t, resp)
	if len(posts) != 1 || posts[0].ID != publicPost.ID {
		t.Fatalf("游客应只看到公开文章：%+v", posts)
	}
	if len(posts[0].MyReactions) != 0 {
		t.Fatalf("游客不应有我的反应：%+v", posts[0].MyReactions)
	}

	code, resp = env.do(t, http.MethodGet, "/api/post/"+idString(publicPost.ID), "", nil)
	if code != http.StatusOK {
		t.Fatalf("游客查看公开文章详情应 200，实际 %d：%s", code, describe(resp))
	}

	code, _ = env.do(t, http.MethodGet, "/api/post/"+idString(hiddenPost.ID), "", nil)
	if code != http.StatusNotFound {
		t.Fatalf("游客查看隐藏文章详情应 404，实际 %d", code)
	}

	// 评论列表公开，但隐藏文章的评论不可见。
	code, resp = env.do(
		t, http.MethodPost, "/api/post/"+idString(publicPost.ID)+"/comments",
		user.Token, CommentInput{Body: "first comment"},
	)
	if code != http.StatusCreated {
		t.Fatalf("发表评论应 201，实际 %d：%s", code, describe(resp))
	}

	code, resp = env.do(t, http.MethodGet, "/api/post/"+idString(publicPost.ID)+"/comments", "", nil)
	if code != http.StatusOK {
		t.Fatalf("游客查看评论应 200，实际 %d：%s", code, describe(resp))
	}
	if comments := decode[[]Post](t, resp); len(comments) != 1 {
		t.Fatalf("评论数量异常：%+v", comments)
	}

	code, _ = env.do(t, http.MethodGet, "/api/post/"+idString(hiddenPost.ID)+"/comments", "", nil)
	if code != http.StatusNotFound {
		t.Fatalf("游客查看隐藏文章评论应 404，实际 %d", code)
	}

	// 标签、排行榜、24 小时计数同样公开。
	for _, path := range []string{"/api/tag", "/api/leaderboard", "/api/recent-post-count"} {
		code, resp = env.do(t, http.MethodGet, path, "", nil)
		if code != http.StatusOK {
			t.Fatalf("公开模式游客访问 %s 应 200，实际 %d：%s", path, code, describe(resp))
		}
	}

	// 写操作仍然必须登录。
	writeCases := []struct {
		method string
		path   string
		body   any
	}{
		{method: http.MethodPost, path: "/api/post/new", body: PostInput{Title: "x", Body: "x"}},
		{method: http.MethodPost, path: "/api/post/" + idString(publicPost.ID) + "/reaction", body: ReactionInput{Type: ReactionLike}},
		{method: http.MethodPost, path: "/api/post/" + idString(publicPost.ID) + "/comments", body: CommentInput{Body: "x"}},
		{method: http.MethodGet, path: "/api/me", body: nil},
	}
	for _, tc := range writeCases {
		code, resp = env.do(t, tc.method, tc.path, "", tc.body)
		if code != http.StatusUnauthorized {
			t.Fatalf("公开模式游客 %s %s 应 401，实际 %d：%s", tc.method, tc.path, code, describe(resp))
		}
	}

	// 已登录用户读取时仍带自己的反应。
	code, resp = env.do(
		t, http.MethodPost, "/api/post/"+idString(publicPost.ID)+"/reaction",
		user.Token, ReactionInput{Type: ReactionLike},
	)
	if code != http.StatusOK {
		t.Fatalf("登录用户反应应 200，实际 %d：%s", code, describe(resp))
	}

	code, resp = env.do(t, http.MethodGet, "/api/post", user.Token, nil)
	if code != http.StatusOK {
		t.Fatalf("登录用户查看文章列表应 200，实际 %d：%s", code, describe(resp))
	}
	posts = decode[[]Post](t, resp)
	var found *Post
	for i := range posts {
		if posts[i].ID == publicPost.ID {
			found = &posts[i]
			break
		}
	}
	if found == nil || len(found.MyReactions) != 1 || found.MyReactions[0] != ReactionLike {
		t.Fatalf("登录用户的我的反应异常：%+v", posts)
	}
}
