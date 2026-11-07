package main

import (
	"net/http"
	"testing"
	"time"
)

func TestRecentPostCount(t *testing.T) {
	env := newTestEnv(t)

	admin := env.register(t, "admin", "admin123")
	alice := env.register(t, "alice", "alice123")
	bob := env.register(t, "bob", "bob12345")

	alicePublic := env.createPost(t, alice.Token, PostInput{Title: "alice public", Body: "body"})
	alicePrivate := env.createPost(t, alice.Token, PostInput{Title: "alice private", Body: "body"})
	env.createPost(t, bob.Token, PostInput{Title: "bob public", Body: "body"})

	// 把一篇文章设为私密；普通用户不可见，作者和管理员可见。
	private := true
	code, resp := env.do(t, http.MethodPatch, "/api/post/"+idString(alicePrivate.ID)+"/private", admin.Token, postPrivateInput{Private: &private})
	if code != http.StatusOK {
		t.Fatalf("设置私密文章应 200，实际 %d：%s", code, describe(resp))
	}

	// 25 小时前的文章不计入 24 小时统计。
	oldPost := env.createPost(t, alice.Token, PostInput{Title: "old", Body: "body"})
	if err := env.db.Model(&Post{}).Where("id = ?", oldPost.ID).Update("created_at", time.Now().Add(-25*time.Hour)).Error; err != nil {
		t.Fatalf("修改旧文章时间失败：%v", err)
	}

	// 评论不是“新发文章”，不应计入。
	code, resp = env.do(t, http.MethodPost, "/api/post/"+idString(alicePublic.ID)+"/comments", bob.Token, CommentInput{Body: "comment"})
	if code != http.StatusCreated {
		t.Fatalf("发表评论应 201，实际 %d：%s", code, describe(resp))
	}

	check := func(token string, want int64) {
		t.Helper()
		code, resp := env.do(t, http.MethodGet, "/api/recent-post-count", token, nil)
		if code != http.StatusOK {
			t.Fatalf("24h 文章数应 200，实际 %d：%s", code, describe(resp))
		}
		got := decode[struct {
			Count int64 `json:"count"`
		}](t, resp)
		if got.Count != want {
			t.Fatalf("24h 文章数应为 %d，实际 %d", want, got.Count)
		}
	}

	// bob 看不到 alice 的私密文章；alice 能看到自己的；管理员能看到全部。
	check(bob.Token, 2)
	check(alice.Token, 3)
	check(admin.Token, 3)

	code, _ = env.do(t, http.MethodGet, "/api/recent-post-count", "", nil)
	if code != http.StatusUnauthorized {
		t.Fatalf("未登录访问 24h 文章数应 401，实际 %d", code)
	}
}
