package main

import (
	"net/http"
	"testing"
)

func (e *testEnv) react(t *testing.T, token string, postID uint, reactionType ReactionType) ReactionResult {
	t.Helper()

	code, resp := e.do(t, http.MethodPost, "/api/post/"+idString(postID)+"/reaction", token, ReactionInput{Type: reactionType})
	if code != http.StatusOK {
		t.Fatalf("反应[%s]期望 200，实际 %d：%s", reactionType, code, describe(resp))
	}

	return decode[ReactionResult](t, resp)
}

func (e *testEnv) fetchMe(t *testing.T, token string) UserView {
	t.Helper()

	code, resp := e.do(t, http.MethodGet, "/api/me", token, nil)
	if code != http.StatusOK {
		t.Fatalf("/me 期望 200，实际 %d：%s", code, describe(resp))
	}

	return decode[UserView](t, resp)
}

func TestReactionToggleAndAuthorTotals(t *testing.T) {
	env := newTestEnv(t)

	author := env.register(t, "alice", "alice123")
	viewer := env.register(t, "bob", "bob12345")
	post := env.createPost(t, author.Token, PostInput{Title: "p", Body: "x"})

	// 点赞
	result := env.react(t, viewer.Token, post.ID, ReactionLike)
	if result.Reactions.Like != 1 {
		t.Fatalf("点赞后 like 应为 1，实际 %d", result.Reactions.Like)
	}
	if len(result.MyReactions) != 1 || result.MyReactions[0] != ReactionLike {
		t.Fatalf("my_reactions 异常：%+v", result.MyReactions)
	}
	if totals := env.fetchMe(t, author.Token).ReactionTotals; totals.Like != 1 {
		t.Fatalf("作者总赞应为 1，实际 %d", totals.Like)
	}

	// 再次点击取消
	result = env.react(t, viewer.Token, post.ID, ReactionLike)
	if result.Reactions.Like != 0 || len(result.MyReactions) != 0 {
		t.Fatalf("取消点赞后应清零：%+v", result)
	}
	if totals := env.fetchMe(t, author.Token).ReactionTotals; totals.Like != 0 {
		t.Fatalf("取消后作者总赞应为 0，实际 %d", totals.Like)
	}

	// 三种主反应可以同时拥有
	env.react(t, viewer.Token, post.ID, ReactionLike)
	env.react(t, viewer.Token, post.ID, ReactionDislike)
	result = env.react(t, viewer.Token, post.ID, ReactionQuestion)
	if result.Reactions.Like != 1 || result.Reactions.Dislike != 1 || result.Reactions.Question != 1 {
		t.Fatalf("三种反应应各 1：%+v", result.Reactions)
	}
	if len(result.MyReactions) != 3 {
		t.Fatalf("my_reactions 应有 3 项，实际 %+v", result.MyReactions)
	}

	totals := env.fetchMe(t, author.Token).ReactionTotals
	if totals.Like != 1 || totals.Dislike != 1 || totals.Question != 1 {
		t.Fatalf("作者总计异常：%+v", totals)
	}

	// 非法类型
	code, resp := env.do(t, http.MethodPost, "/api/post/"+idString(post.ID)+"/reaction", viewer.Token, ReactionInput{Type: "nope"})
	if code != http.StatusBadRequest || resp.Error == nil || resp.Error.Code != "INVALID_REACTION" {
		t.Fatalf("非法反应应 400 INVALID_REACTION，实际 %d：%s", code, describe(resp))
	}

	// 不存在的文章
	code, _ = env.do(t, http.MethodPost, "/api/post/99999/reaction", viewer.Token, ReactionInput{Type: ReactionLike})
	if code != http.StatusNotFound {
		t.Fatalf("不存在的文章应 404，实际 %d", code)
	}

	// 未登录
	code, _ = env.do(t, http.MethodPost, "/api/post/"+idString(post.ID)+"/reaction", "", ReactionInput{Type: ReactionLike})
	if code != http.StatusUnauthorized {
		t.Fatalf("未登录应 401，实际 %d", code)
	}
}

func TestReactionEmojiAndMyReactions(t *testing.T) {
	env := newTestEnv(t)

	author := env.register(t, "alice", "alice123")
	bob := env.register(t, "bob", "bob12345")
	carol := env.register(t, "carol", "carol123")
	post := env.createPost(t, author.Token, PostInput{Title: "p", Body: "x"})

	// 表情计数
	result := env.react(t, bob.Token, post.ID, ReactionLaugh)
	if result.Reactions.Emoji["laugh"] != 1 {
		t.Fatalf("laugh 应为 1：%+v", result.Reactions.Emoji)
	}
	env.react(t, bob.Token, post.ID, ReactionHeart)
	result = env.react(t, carol.Token, post.ID, ReactionLaugh)
	if result.Reactions.Emoji["laugh"] != 2 {
		t.Fatalf("两个 laugh 后应为 2：%+v", result.Reactions.Emoji)
	}

	// bob 取消 laugh，只减这个表情
	result = env.react(t, bob.Token, post.ID, ReactionLaugh)
	if result.Reactions.Emoji["laugh"] != 1 || result.Reactions.Emoji["heart"] != 1 {
		t.Fatalf("取消一个表情后计数异常：%+v", result.Reactions.Emoji)
	}

	totals := env.fetchMe(t, author.Token).ReactionTotals
	if totals.Emoji["laugh"] != 1 || totals.Emoji["heart"] != 1 {
		t.Fatalf("作者表情总计异常：%+v", totals.Emoji)
	}

	// 列表按请求者返回各自的 my_reactions
	code, resp := env.do(t, http.MethodGet, "/api/post", bob.Token, nil)
	if code != http.StatusOK {
		t.Fatalf("文章列表应 200，实际 %d：%s", code, describe(resp))
	}
	posts := decode[[]Post](t, resp)
	if len(posts) != 1 {
		t.Fatalf("应只有 1 篇文章，实际 %d", len(posts))
	}
	if len(posts[0].MyReactions) != 1 || posts[0].MyReactions[0] != ReactionHeart {
		t.Fatalf("bob 的 my_reactions 异常：%+v", posts[0].MyReactions)
	}
	if posts[0].Reactions.Emoji["laugh"] != 1 {
		t.Fatalf("列表反应汇总异常：%+v", posts[0].Reactions)
	}

	code, resp = env.do(t, http.MethodGet, "/api/post", carol.Token, nil)
	if code != http.StatusOK {
		t.Fatalf("文章列表应 200，实际 %d：%s", code, describe(resp))
	}
	posts = decode[[]Post](t, resp)
	if len(posts[0].MyReactions) != 1 || posts[0].MyReactions[0] != ReactionLaugh {
		t.Fatalf("carol 的 my_reactions 异常：%+v", posts[0].MyReactions)
	}

	// 详情按请求者区分
	code, resp = env.do(t, http.MethodGet, "/api/post/"+idString(post.ID), bob.Token, nil)
	if code != http.StatusOK {
		t.Fatalf("文章详情应 200，实际 %d：%s", code, describe(resp))
	}
	detail := decode[Post](t, resp)
	if len(detail.MyReactions) != 1 || detail.MyReactions[0] != ReactionHeart {
		t.Fatalf("详情 my_reactions 异常：%+v", detail.MyReactions)
	}
}

func TestSyncReactionCounts(t *testing.T) {
	env := newTestEnv(t)

	author := env.register(t, "alice", "alice123")
	viewer := env.register(t, "bob", "bob12345")
	post := env.createPost(t, author.Token, PostInput{Title: "p", Body: "x"})

	env.react(t, viewer.Token, post.ID, ReactionLike)
	env.react(t, viewer.Token, post.ID, ReactionRocket)

	var stored Post
	if err := env.db.First(&stored, post.ID).Error; err != nil {
		t.Fatalf("查询文章失败：%v", err)
	}
	if stored.LikeCount != 1 || stored.EmojiCounts == "" {
		t.Fatalf("反应计数未写入：%+v", stored)
	}

	// 模拟明细丢失后校准：计数应回到 0
	if err := env.db.Exec("DELETE FROM reactions").Error; err != nil {
		t.Fatalf("删除反应明细失败：%v", err)
	}
	if err := syncReactionCounts(env.db); err != nil {
		t.Fatalf("校准反应计数失败：%v", err)
	}

	if err := env.db.First(&stored, post.ID).Error; err != nil {
		t.Fatalf("重新查询文章失败：%v", err)
	}
	if stored.LikeCount != 0 || stored.EmojiCounts != "" {
		t.Fatalf("校准后文章计数应清零：%+v", stored)
	}

	totals := env.fetchMe(t, author.Token).ReactionTotals
	if totals.Like != 0 || len(totals.Emoji) != 0 {
		t.Fatalf("校准后作者总计应清零：%+v", totals)
	}
}

