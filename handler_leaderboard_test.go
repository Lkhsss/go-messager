package main

import (
	"net/http"
	"testing"
	"time"
)

func leaderboardCount(entries []LeaderboardEntry, postID uint) int64 {
	for _, entry := range entries {
		if entry.Post.ID == postID {
			return entry.Count
		}
	}
	return 0
}

func TestLeaderboard(t *testing.T) {
	env := newTestEnv(t)

	alice := env.register(t, "alice", "alice123")
	bob := env.register(t, "bob", "bob12345")
	carol := env.register(t, "carol", "carol123")

	first := env.createPost(t, alice.Token, PostInput{Title: "first", Body: "body"})
	second := env.createPost(t, bob.Token, PostInput{Title: "second", Body: "body"})
	third := env.createPost(t, alice.Token, PostInput{Title: "third", Body: "body"})

	// 点赞：first 两个；first 的评论也有一个反应，但评论不应进入排行榜
	env.react(t, bob.Token, first.ID, ReactionLike)
	env.react(t, carol.Token, first.ID, ReactionLike)
	code, resp := env.do(t, http.MethodPost, "/api/post/"+idString(first.ID)+"/comments", bob.Token, CommentInput{Body: "comment"})
	if code != http.StatusCreated {
		t.Fatalf("评论应 201，实际 %d：%s", code, describe(resp))
	}
	comment := decode[Post](t, resp)
	env.react(t, carol.Token, comment.ID, ReactionLike)

	// 点踩：second 两个
	env.react(t, bob.Token, second.ID, ReactionDislike)
	env.react(t, carol.Token, second.ID, ReactionDislike)

	// 问号：third 一个
	env.react(t, carol.Token, third.ID, ReactionQuestion)

	// 加入一个 10 天前的点赞，用于验证 day/week 时间窗口
	var aliceUser User
	if err := env.db.Where("name = ?", "alice").First(&aliceUser).Error; err != nil {
		t.Fatalf("查询用户失败：%v", err)
	}
	if err := env.db.Create(&Reaction{
		PostID: second.ID, UserID: aliceUser.ID, Type: ReactionLike,
		CreatedAt: time.Now().Add(-3 * 24 * time.Hour),
	}).Error; err != nil {
		t.Fatalf("创建历史反应失败：%v", err)
	}

	check := func(period string) LeaderboardResponse {
		t.Helper()
		code, resp := env.do(t, http.MethodGet, "/api/leaderboard?period="+period, alice.Token, nil)
		if code != http.StatusOK {
			t.Fatalf("排行榜[%s]应 200，实际 %d：%s", period, code, describe(resp))
		}
		return decode[LeaderboardResponse](t, resp)
	}

	day := check("day")
	if day.Period != "day" {
		t.Fatalf("period 应为 day，实际 %s", day.Period)
	}
	if got := leaderboardCount(day.Like, first.ID); got != 2 {
		t.Fatalf("day 点赞 first 应为 2，实际 %d", got)
	}
	if got := leaderboardCount(day.Dislike, second.ID); got != 2 {
		t.Fatalf("day 点踩 second 应为 2，实际 %d", got)
	}
	if got := leaderboardCount(day.Question, third.ID); got != 1 {
		t.Fatalf("day 问号 third 应为 1，实际 %d", got)
	}
	if got := leaderboardCount(day.Like, second.ID); got != 0 {
		t.Fatalf("day 不应统计 3 天前的点赞，实际 %d", got)
	}

	week := check("week")
	if got := leaderboardCount(week.Like, second.ID); got != 1 {
		t.Fatalf("week 点赞 second 应包含历史记录为 1，实际 %d", got)
	}

	// 非法时间范围
	code, resp = env.do(t, http.MethodGet, "/api/leaderboard?period=all", alice.Token, nil)
	if code != http.StatusBadRequest || resp.Error == nil || resp.Error.Code != "INVALID_PERIOD" {
		t.Fatalf("非法 period 应 400 INVALID_PERIOD，实际 %d：%s", code, describe(resp))
	}

	// 未登录
	code, _ = env.do(t, http.MethodGet, "/api/leaderboard", "", nil)
	if code != http.StatusUnauthorized {
		t.Fatalf("未登录排行榜应 401，实际 %d", code)
	}
}
