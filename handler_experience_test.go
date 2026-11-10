package main

import (
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestDailyExperienceTasks(t *testing.T) {
	env := newTestEnv(t)

	// 注册即登录，发放每日登录经验。
	user := env.register(t, "alice", "alice100")
	if user.XP == nil || user.XP.Gained != DailyLoginXP || user.XP.Task != DailyTaskLogin {
		t.Fatalf("注册应发放每日登录经验，实际 %+v", user.XP)
	}
	if user.User.Exp != DailyLoginXP || user.User.Level != 1 {
		t.Fatalf("注册后经验/等级异常：exp=%d level=%d", user.User.Exp, user.User.Level)
	}

	// 当天再次登录不重复发放。
	code, resp := env.do(t, http.MethodPost, "/api/authorize", "", Credentials{Name: "alice", Password: "alice100"})
	if code != http.StatusOK {
		t.Fatalf("登录应 200，实际 %d：%s", code, describe(resp))
	}
	again := decode[AuthResult](t, resp)
	if again.XP != nil {
		t.Fatalf("同一天重复登录不应再发经验：%+v", again.XP)
	}
	if again.User.Exp != DailyLoginXP {
		t.Fatalf("重复登录后经验不应变化，实际 %d", again.User.Exp)
	}

	// 不足 200 字的文章不发经验。
	short := env.createPost(t, user.Token, PostInput{Title: "短文章", Body: "内容"})
	if short.XP != nil {
		t.Fatalf("短文章不应发经验：%+v", short.XP)
	}
	if short.UserLevel != 1 {
		t.Fatalf("新用户文章等级应为 1，实际 %d", short.UserLevel)
	}

	// 200 字文章 +5，同一天第二篇不再发放。
	longBody := strings.Repeat("文", int(DailyArticleMinChars))
	long := env.createPost(t, user.Token, PostInput{Title: "长文", Body: longBody})
	if long.XP == nil || long.XP.Gained != DailyArticleXP || long.XP.Task != DailyTaskArticle {
		t.Fatalf("200 字文章应发 %d 经验，实际 %+v", DailyArticleXP, long.XP)
	}
	if second := env.createPost(t, user.Token, PostInput{Title: "长文 2", Body: longBody}); second.XP != nil {
		t.Fatalf("同一天第二篇文章不应再发经验：%+v", second.XP)
	}

	commentPath := "/api/post/" + idString(long.ID) + "/comments"
	// 不足 100 字的评论不发经验。
	code, resp = env.do(t, http.MethodPost, commentPath, user.Token, CommentInput{Body: "太短了"})
	if code != http.StatusCreated {
		t.Fatalf("评论应 201，实际 %d：%s", code, describe(resp))
	}
	if shortComment := decode[Post](t, resp); shortComment.XP != nil {
		t.Fatalf("短评论不应发经验：%+v", shortComment.XP)
	}

	// 100 字评论 +3，第二条不再发放。
	code, resp = env.do(t, http.MethodPost, commentPath, user.Token,
		CommentInput{Body: strings.Repeat("评", int(DailyCommentMinChars))})
	if code != http.StatusCreated {
		t.Fatalf("评论应 201，实际 %d：%s", code, describe(resp))
	}
	comment := decode[Post](t, resp)
	if comment.XP == nil || comment.XP.Gained != DailyCommentXP || comment.XP.Task != DailyTaskComment {
		t.Fatalf("100 字评论应发 %d 经验，实际 %+v", DailyCommentXP, comment.XP)
	}
	code, resp = env.do(t, http.MethodPost, commentPath, user.Token,
		CommentInput{Body: strings.Repeat("评", int(DailyCommentMinChars))})
	if code != http.StatusCreated {
		t.Fatalf("评论应 201，实际 %d：%s", code, describe(resp))
	}
	if second := decode[Post](t, resp); second.XP != nil {
		t.Fatalf("同一天第二条评论不应再发经验：%+v", second.XP)
	}

	// 互动每天最多 5 次，取消反应不算。
	reactionTypes := []ReactionType{
		ReactionLike, ReactionDislike, ReactionQuestion, ReactionLaugh, ReactionHooray, ReactionHeart,
	}
	for i, reactionType := range reactionTypes {
		result := env.react(t, user.Token, long.ID, reactionType)
		if i < int(DailyInteractionLimit) {
			if result.XP == nil || result.XP.Gained != DailyInteractionXP {
				t.Fatalf("第 %d 次互动应发经验，实际 %+v", i+1, result.XP)
			}
			continue
		}
		if result.XP != nil {
			t.Fatalf("超过每日互动上限后不应发经验：%+v", result.XP)
		}
	}

	// 取消反应不计数，再新增也仍在上限内。
	if cancelled := env.react(t, user.Token, long.ID, ReactionLike); cancelled.XP != nil {
		t.Fatalf("取消反应不应发经验：%+v", cancelled.XP)
	}

	expectedExp := DailyLoginXP + DailyArticleXP + DailyCommentXP + DailyInteractionLimit*DailyInteractionXP
	me := env.fetchMe(t, user.Token)
	if me.Exp != expectedExp {
		t.Fatalf("总经验应为 %d，实际 %d", expectedExp, me.Exp)
	}

	// 任务面板状态。
	code, resp = env.do(t, http.MethodGet, "/api/me/experience", user.Token, nil)
	if code != http.StatusOK {
		t.Fatalf("经验面板应 200，实际 %d：%s", code, describe(resp))
	}
	status := decode[ExperienceStatus](t, resp)
	if status.Exp != expectedExp || status.Level != 1 {
		t.Fatalf("面板经验/等级异常：%+v", status)
	}
	if status.LevelStartExp != 0 || status.NextLevelExp != 100 ||
		status.LevelProgressExp != expectedExp || status.LevelNeedExp != 100 {
		t.Fatalf("面板等级进度异常：%+v", status)
	}
	if len(status.Tasks) != 4 {
		t.Fatalf("每日任务应为 4 条，实际 %d：%+v", len(status.Tasks), status.Tasks)
	}
	for _, task := range status.Tasks {
		if !task.Completed || task.Progress != task.Target {
			t.Fatalf("任务[%s]应已完成，实际 %+v", task.Key, task)
		}
	}

	// 文章列表带作者等级。
	code, resp = env.do(t, http.MethodGet, "/api/post", user.Token, nil)
	if code != http.StatusOK {
		t.Fatalf("文章列表应 200，实际 %d：%s", code, describe(resp))
	}

	for _, post := range decode[[]Post](t, resp) {
		if post.UserLevel != 1 {
			t.Fatalf("文章作者等级应为 1，实际 %d（%s）", post.UserLevel, post.Title)
		}
	}
}

func TestDailyLoginLevelUp(t *testing.T) {
	env := newTestEnv(t)

	auth := env.register(t, "bob", "bob12345")
	if auth.XP == nil {
		t.Fatalf("注册应发放每日登录经验")
	}

	var user User
	if err := env.db.Where("name = ?", "bob").Take(&user).Error; err != nil {
		t.Fatalf("查询用户失败：%v", err)
	}
	// 差 1 点经验升到 2 级。
	if err := env.db.Model(&User{}).Where("id = ?", user.ID).Update("exp", 99).Error; err != nil {
		t.Fatalf("更新经验失败：%v", err)
	}

	data := &Data{DB: env.db}
	// 用明天的日期模拟次日登录。
	gain, err := data.awardDailyTask(user.ID, DailyTaskLogin, DailyLoginXP, time.Now().AddDate(0, 0, 1))
	if err != nil {
		t.Fatalf("发放经验失败：%v", err)
	}
	if gain == nil || gain.Exp != 100 || gain.Level != 2 || !gain.LeveledUp {
		t.Fatalf("登录后应升到 2 级，实际 %+v", gain)
	}

	me := env.fetchMe(t, auth.Token)
	if me.Exp != 100 || me.Level != 2 {
		t.Fatalf("/me 经验等级异常：exp=%d level=%d", me.Exp, me.Level)
	}
}

func TestExperienceRequiresLogin(t *testing.T) {
	env := newTestEnv(t)

	code, resp := env.do(t, http.MethodGet, "/api/me/experience", "", nil)
	if code != http.StatusUnauthorized {
		t.Fatalf("未登录访问经验面板应 401，实际 %d：%s", code, describe(resp))
	}
}
