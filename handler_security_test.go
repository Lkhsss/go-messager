package main

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestNewPostRejectsDuplicateBody(t *testing.T) {
	env := newTestEnvWithConfig(t, func(cfg *Config) {
		cfg.Limits.DuplicateWindow = 24 * time.Hour
		cfg.Limits.PostPerMinute = 100
		cfg.Limits.NewAccountPostPerDay = 100
	})

	alice := env.register(t, "alice", "alice123")
	bob := env.register(t, "bob", "bob12345")

	env.createPost(t, alice.Token, PostInput{Title: "first", Body: "same body"})

	code, resp := env.do(t, http.MethodPost, "/api/post/new", alice.Token,
		PostInput{Title: "second", Body: "same body"})
	if code != http.StatusConflict || resp.Error == nil || resp.Error.Code != "DUPLICATE_POST" {
		t.Fatalf("相同正文应 409 DUPLICATE_POST，实际 %d：%s", code, describe(resp))
	}

	// 归一化空白后相同，也应拦截。
	code, resp = env.do(t, http.MethodPost, "/api/post/new", alice.Token,
		PostInput{Title: "third", Body: " same \n body "})
	if code != http.StatusConflict || resp.Error == nil || resp.Error.Code != "DUPLICATE_POST" {
		t.Fatalf("空白变体应 409 DUPLICATE_POST，实际 %d：%s", code, describe(resp))
	}

	// 不同用户可以发布相同正文。
	env.createPost(t, bob.Token, PostInput{Title: "bob post", Body: "same body"})
}

func TestNewPostRateLimited(t *testing.T) {
	env := newTestEnvWithConfig(t, func(cfg *Config) {
		cfg.Limits.PostPerMinute = 2
		cfg.Limits.PostPerDay = 100
		cfg.Limits.NewAccountPostPerDay = 100
		cfg.Limits.DuplicateWindow = 0
	})

	user := env.register(t, "alice", "alice123")
	env.createPost(t, user.Token, PostInput{Title: "one", Body: "body one"})
	env.createPost(t, user.Token, PostInput{Title: "two", Body: "body two"})

	code, resp := env.do(t, http.MethodPost, "/api/post/new", user.Token,
		PostInput{Title: "three", Body: "body three"})
	if code != http.StatusTooManyRequests || resp.Error == nil || resp.Error.Code != "POST_RATE_LIMITED" {
		t.Fatalf("超过每分钟发文上限应 429 POST_RATE_LIMITED，实际 %d：%s", code, describe(resp))
	}
}

func TestUntrustedPostDailyQuota(t *testing.T) {
	env := newTestEnvWithConfig(t, func(cfg *Config) {
		cfg.Limits.PostPerMinute = 100
		cfg.Limits.NewAccountPostPerDay = 2
		cfg.Limits.PostPerDay = 100
		cfg.Limits.DuplicateWindow = 0
	})

	env.register(t, "admin", "admin123")
	bob := env.register(t, "bob", "bob12345")

	env.createPost(t, bob.Token, PostInput{Title: "one", Body: "body one"})
	env.createPost(t, bob.Token, PostInput{Title: "two", Body: "body two"})

	code, resp := env.do(t, http.MethodPost, "/api/post/new", bob.Token,
		PostInput{Title: "three", Body: "body three"})
	if code != http.StatusTooManyRequests || resp.Error == nil || resp.Error.Code != "POST_QUOTA_EXCEEDED" {
		t.Fatalf("新账号超过每日配额应 429 POST_QUOTA_EXCEEDED，实际 %d：%s", code, describe(resp))
	}

	// 管理员标记可信后解除新账号配额。
	if err := env.db.Model(&User{}).Where("name = ?", "bob").Update("trusted", true).Error; err != nil {
		t.Fatalf("标记可信失败：%v", err)
	}
	env.createPost(t, bob.Token, PostInput{Title: "three", Body: "body three"})
}

func TestRequestBodyTooLarge(t *testing.T) {
	env := newTestEnv(t)
	user := env.register(t, "alice", "alice123")

	code, resp := env.do(t, http.MethodPost, "/api/post/new", user.Token,
		PostInput{Title: "big", Body: strings.Repeat("a", int(defaultMaxRequestBodyBytes)+1024)})
	if code != http.StatusRequestEntityTooLarge || resp.Error == nil || resp.Error.Code != "BODY_TOO_LARGE" {
		t.Fatalf("超大请求体应 413 BODY_TOO_LARGE，实际 %d：%s", code, describe(resp))
	}
}

func TestRegisterRejectsWeakCredentials(t *testing.T) {
	env := newTestEnv(t)
	env.register(t, "admin", "admin123")

	code, resp := env.do(t, http.MethodPost, "/api/register", "",
		RegisterInput{Name: "1", Password: "password123"})
	if code != http.StatusBadRequest || resp.Error == nil || resp.Error.Code != "INVALID_USERNAME" {
		t.Fatalf("单字符用户名应 400 INVALID_USERNAME，实际 %d：%s", code, describe(resp))
	}

	code, resp = env.do(t, http.MethodPost, "/api/register", "",
		RegisterInput{Name: "bob", Password: "short"})
	if code != http.StatusBadRequest || resp.Error == nil || resp.Error.Code != "WEAK_PASSWORD" {
		t.Fatalf("短密码应 400 WEAK_PASSWORD，实际 %d：%s", code, describe(resp))
	}
}

func TestRegisterRateLimitedByIP(t *testing.T) {
	env := newTestEnvWithConfig(t, func(cfg *Config) {
		cfg.Limits.RegisterPerIPPerHour = 2
		cfg.Limits.RegisterPerIPPerDay = 100
	})

	env.register(t, "admin", "admin123")
	env.register(t, "bob", "bob12345")
	env.register(t, "carol", "carol123")

	ticket := env.solveCaptcha(t)
	code, resp := env.do(t, http.MethodPost, "/api/register", "",
		RegisterInput{Name: "dave", Password: "dave12345", CaptchaTicket: ticket})
	if code != http.StatusTooManyRequests || resp.Error == nil || resp.Error.Code != "REGISTER_RATE_LIMITED" {
		t.Fatalf("同 IP 注册超限应 429 REGISTER_RATE_LIMITED，实际 %d：%s", code, describe(resp))
	}
}

func TestCaptchaTicketBoundToIP(t *testing.T) {
	store := NewCaptchaStore()
	now := time.Now()

	challenge, err := store.create(now, "192.0.2.1")
	if err != nil {
		t.Fatalf("生成验证码失败：%v", err)
	}
	answer, ok := store.peekAnswer(challenge.ID)
	if !ok {
		t.Fatalf("验证码[%s]不存在", challenge.ID)
	}
	ticket, err := store.verify(now, "192.0.2.1", challenge.ID, answer)
	if err != nil {
		t.Fatalf("校验验证码失败：%v", err)
	}
	if store.consumeTicket(now, "192.0.2.2", ticket.Ticket) {
		t.Fatal("跨 IP 使用 ticket 应失败")
	}

	// 新 ticket 在同一 IP 仍然可用。
	challenge, err = store.create(now, "192.0.2.1")
	if err != nil {
		t.Fatalf("生成验证码失败：%v", err)
	}
	answer, _ = store.peekAnswer(challenge.ID)
	ticket, err = store.verify(now, "192.0.2.1", challenge.ID, answer)
	if err != nil {
		t.Fatalf("校验验证码失败：%v", err)
	}
	if !store.consumeTicket(now, "192.0.2.1", ticket.Ticket) {
		t.Fatal("同 IP 使用 ticket 应成功")
	}
}

func TestRateLimiterIsBounded(t *testing.T) {
	limiter := newRateLimiter(time.Hour)
	now := time.Now()
	for i := 0; i < rateLimiterMaxEntries; i++ {
		limiter.entries[fmt.Sprintf("key-%d", i)] = rateWindowState{start: now, count: 1}
	}
	limiter.lastSweep = now

	limiter.allowAt(now, "new-key", 1)
	if len(limiter.entries) > rateLimiterMaxEntries {
		t.Fatalf("限流器条目不应超过 %d，实际 %d", rateLimiterMaxEntries, len(limiter.entries))
	}
}

func TestSyncReactionCountsOnlyTouchesAffectedRows(t *testing.T) {
	env := newTestEnv(t)
	admin := env.register(t, "admin", "admin123")

	reacted := env.createPost(t, admin.Token, PostInput{Title: "reacted", Body: "reacted body"})
	env.react(t, admin.Token, reacted.ID, ReactionLike)

	stale := env.createPost(t, admin.Token, PostInput{Title: "stale", Body: "stale body"})
	untouched := env.createPost(t, admin.Token, PostInput{Title: "untouched", Body: "untouched body"})

	old := time.Date(2020, 1, 2, 3, 4, 5, 0, time.Local)
	if err := env.db.Model(&Post{}).Where("id = ?", stale.ID).Updates(map[string]any{
		"like_count": 9,
		"updated_at": old,
	}).Error; err != nil {
		t.Fatalf("写入脏计数失败：%v", err)
	}
	if err := env.db.Model(&Post{}).Where("id = ?", untouched.ID).Update("updated_at", old).Error; err != nil {
		t.Fatalf("设置更新时间失败：%v", err)
	}

	if err := syncReactionCounts(env.db); err != nil {
		t.Fatalf("校准反应统计失败：%v", err)
	}

	var reactedRow, staleRow, untouchedRow Post
	if err := env.db.Take(&reactedRow, reacted.ID).Error; err != nil {
		t.Fatalf("读取文章失败：%v", err)
	}
	if reactedRow.LikeCount != 1 {
		t.Fatalf("有反应的文章 like_count 应为 1，实际 %d", reactedRow.LikeCount)
	}
	if err := env.db.Take(&staleRow, stale.ID).Error; err != nil {
		t.Fatalf("读取文章失败：%v", err)
	}
	if staleRow.LikeCount != 0 {
		t.Fatalf("脏计数应被清零，实际 %d", staleRow.LikeCount)
	}
	if err := env.db.Take(&untouchedRow, untouched.ID).Error; err != nil {
		t.Fatalf("读取文章失败：%v", err)
	}
	if !untouchedRow.UpdatedAt.Equal(old) {
		t.Fatalf("无反应且计数为零的文章不应被重写，updated_at 变为 %v", untouchedRow.UpdatedAt)
	}
}
