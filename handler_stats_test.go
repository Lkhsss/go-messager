package main

import (
	"net/http"
	"testing"
)

func TestMyStats(t *testing.T) {
	env := newTestEnv(t)

	admin := env.register(t, "admin", "admin123")
	alice := env.register(t, "alice", "alice123")
	bob := env.register(t, "bob", "bob12345")
	_ = admin

	first := env.createPost(t, alice.Token, PostInput{Title: "one", Body: "abc"})
	second := env.createPost(t, alice.Token, PostInput{Title: "two", Body: "defgh"})

	// alice 自己发一条评论，bob 在 alice 文章下发一条评论
	code, resp := env.do(t, http.MethodPost, "/api/post/"+idString(first.ID)+"/comments", alice.Token, CommentInput{Body: "my comment"})
	if code != http.StatusCreated {
		t.Fatalf("alice 发表评论应 201，实际 %d：%s", code, describe(resp))
	}
	code, resp = env.do(t, http.MethodPost, "/api/post/"+idString(second.ID)+"/comments", bob.Token, CommentInput{Body: "bob comment"})
	if code != http.StatusCreated {
		t.Fatalf("bob 发表评论应 201，实际 %d：%s", code, describe(resp))
	}

	env.react(t, bob.Token, first.ID, ReactionLike)
	env.react(t, bob.Token, first.ID, ReactionDislike)

	code, resp = env.do(t, http.MethodGet, "/api/me/stats", alice.Token, nil)
	if code != http.StatusOK {
		t.Fatalf("个人统计应 200，实际 %d：%s", code, describe(resp))
	}
	stats := decode[UserStatsResponse](t, resp)
	if stats.PostCount != 2 {
		t.Fatalf("文章数应为 2，实际 %d", stats.PostCount)
	}
	// one(3) + abc(3) + two(3) + defgh(5) = 14
	if stats.WordCount != 14 {
		t.Fatalf("字数应为 14，实际 %d", stats.WordCount)
	}
	if stats.Like != 1 || stats.Dislike != 1 || stats.Question != 0 {
		t.Fatalf("反应统计异常：%+v", stats)
	}
	if stats.CommentCount != 1 {
		t.Fatalf("发表评论数应为 1，实际 %d", stats.CommentCount)
	}
	if stats.ReceivedCommentCount != 2 {
		t.Fatalf("文章收到评论数应为 2，实际 %d", stats.ReceivedCommentCount)
	}
}

func TestAdminStats(t *testing.T) {
	env := newTestEnv(t)

	admin := env.register(t, "admin", "admin123")
	alice := env.register(t, "alice", "alice123")
	bob := env.register(t, "bob", "bob12345")

	// 成功登录一次，登录统计应从 0 变为 1。
	code, resp := env.do(t, http.MethodPost, "/api/authorize", "", Credentials{Name: "admin", Password: "admin123"})
	if code != http.StatusOK {
		t.Fatalf("登录应 200，实际 %d：%s", code, describe(resp))
	}

	tag := env.createTag(t, admin.Token, "Go")
	post := env.createPost(t, alice.Token, PostInput{Title: "article", Body: "body", TagIDs: []uint{tag.ID}})
	code, resp = env.do(t, http.MethodPost, "/api/post/"+idString(post.ID)+"/comments", bob.Token, CommentInput{Body: "comment"})
	if code != http.StatusCreated {
		t.Fatalf("评论应 201，实际 %d：%s", code, describe(resp))
	}
	env.react(t, bob.Token, post.ID, ReactionLike)

	code, resp = env.do(t, http.MethodGet, "/api/admin/stats?unit=day", admin.Token, nil)
	if code != http.StatusOK {
		t.Fatalf("管理员统计应 200，实际 %d：%s", code, describe(resp))
	}
	stats := decode[AdminStatsResponse](t, resp)
	if stats.PostCount != 1 || stats.UserCount != 3 || stats.CommentCount != 1 {
		t.Fatalf("基础统计异常：%+v", stats)
	}
	if stats.LoginCount != 1 {
		t.Fatalf("登录次数应为 1，实际 %d", stats.LoginCount)
	}
	if stats.RegistrationTotal != 3 || stats.RegistrationRecent != 3 {
		t.Fatalf("注册统计异常：%+v", stats)
	}
	if stats.TagCount != 1 || len(stats.TagUsage) != 1 || stats.TagUsage[0].Name != "Go" || stats.TagUsage[0].Count != 1 {
		t.Fatalf("标签统计异常：%+v", stats.TagUsage)
	}
	if stats.Reactions.Like != 1 || stats.Reactions.Total != 1 {
		t.Fatalf("反应统计异常：%+v", stats.Reactions)
	}
	if len(stats.ArticleSeries) != 30 {
		t.Fatalf("day 折线长度应为 30，实际 %d", len(stats.ArticleSeries))
	}
	var articleTotal int64
	for _, point := range stats.ArticleSeries {
		articleTotal += point.Count
	}
	if articleTotal != 1 {
		t.Fatalf("day 折线文章总数应为 1，实际 %d", articleTotal)
	}
	if len(stats.Heatmap) != 365 {
		t.Fatalf("热力图长度应为 365，实际 %d", len(stats.Heatmap))
	}
	var interactionTotal int64
	for _, point := range stats.Heatmap {
		interactionTotal += point.Count
	}
	if interactionTotal != 3 {
		t.Fatalf("互动总数应为文章+评论+反应=3，实际 %d", interactionTotal)
	}

	// 其他时间单位长度
	for unit, length := range map[string]int{"hour": 24, "week": 12, "month": 12} {
		code, resp = env.do(t, http.MethodGet, "/api/admin/stats?unit="+unit, admin.Token, nil)
		if code != http.StatusOK {
			t.Fatalf("unit=%s 应 200，实际 %d：%s", unit, code, describe(resp))
		}
		got := decode[AdminStatsResponse](t, resp)
		if len(got.ArticleSeries) != length {
			t.Fatalf("unit=%s 折线长度应为 %d，实际 %d", unit, length, len(got.ArticleSeries))
		}
	}

	// 普通用户不能访问管理员统计
	code, _ = env.do(t, http.MethodGet, "/api/admin/stats", alice.Token, nil)
	if code != http.StatusForbidden {
		t.Fatalf("普通用户访问管理员统计应 403，实际 %d", code)
	}
	code, _ = env.do(t, http.MethodGet, "/api/admin/stats", "", nil)
	if code != http.StatusUnauthorized {
		t.Fatalf("未登录访问管理员统计应 401，实际 %d", code)
	}
}
