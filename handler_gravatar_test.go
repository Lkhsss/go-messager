package main

import (
	"net/http"
	"strings"
	"testing"
)

func (e *testEnv) registerWithEmail(t *testing.T, name, password, email string) AuthResult {
	t.Helper()

	code, resp := e.do(t, http.MethodPost, "/api/register", "", RegisterInput{
		Name: name, Password: password, Email: email,
	})
	if code != http.StatusCreated {
		t.Fatalf("注册[%s]期望 201，实际 %d：%s", name, code, describe(resp))
	}

	return decode[AuthResult](t, resp)
}

func TestGravatarRegisterAndUpdate(t *testing.T) {
	env := newTestEnv(t)

	auth := env.registerWithEmail(t, "alice", "alice123", "  Foo@Bar.com ")
	wantHash := gravatarHash("foo@bar.com")
	if auth.User.Email != "foo@bar.com" {
		t.Fatalf("邮箱应规范化为小写，实际 %q", auth.User.Email)
	}
	if auth.User.GravatarHash != wantHash || len(wantHash) != 32 {
		t.Fatalf("gravatar 哈希异常：%q", auth.User.GravatarHash)
	}

	// /me 返回本人邮箱
	code, resp := env.do(t, http.MethodGet, "/api/me", auth.Token, nil)
	if code != http.StatusOK {
		t.Fatalf("/me 应 200，实际 %d：%s", code, describe(resp))
	}
	me := decode[UserView](t, resp)
	if me.Email != "foo@bar.com" || me.GravatarHash != wantHash {
		t.Fatalf("/me 用户信息异常：%+v", me)
	}

	// 修改邮箱后哈希随之变化
	code, resp = env.do(t, http.MethodPatch, "/api/me", auth.Token, UpdateMeInput{Email: "New@Example.com"})
	if code != http.StatusOK {
		t.Fatalf("修改邮箱应 200，实际 %d：%s", code, describe(resp))
	}
	updated := decode[UserView](t, resp)
	if updated.Email != "new@example.com" || updated.GravatarHash != gravatarHash("new@example.com") {
		t.Fatalf("修改后用户信息异常：%+v", updated)
	}

	// 非法邮箱
	for _, email := range []string{"abc", "a@", "Name <a@b.com>", strings.Repeat("a", 250) + "@example.com"} {
		code, resp = env.do(t, http.MethodPatch, "/api/me", auth.Token, UpdateMeInput{Email: email})
		if code != http.StatusBadRequest || resp.Error == nil || resp.Error.Code != "INVALID_EMAIL" {
			t.Fatalf("非法邮箱[%s]应 400 INVALID_EMAIL，实际 %d：%s", email, code, describe(resp))
		}
	}

	// 清空邮箱
	code, resp = env.do(t, http.MethodPatch, "/api/me", auth.Token, UpdateMeInput{})
	if code != http.StatusOK {
		t.Fatalf("清空邮箱应 200，实际 %d：%s", code, describe(resp))
	}
	cleared := decode[UserView](t, resp)
	if cleared.Email != "" || cleared.GravatarHash != "" {
		t.Fatalf("清空后应无邮箱/哈希：%+v", cleared)
	}
}

func TestPostAuthorGravatar(t *testing.T) {
	env := newTestEnv(t)

	alice := env.registerWithEmail(t, "alice", "alice123", "Alice@Example.com")
	bob := env.register(t, "bob", "bob12345") // 未设置邮箱

	alicePost := env.createPost(t, alice.Token, PostInput{Title: "alice post", Body: "x"})
	env.createPost(t, bob.Token, PostInput{Title: "bob post", Body: "x"})

	// 详情带作者头像哈希，且不暴露 email
	code, resp := env.do(t, http.MethodGet, "/api/post/"+idString(alicePost.ID), alice.Token, nil)
	if code != http.StatusOK {
		t.Fatalf("文章详情应 200，实际 %d：%s", code, describe(resp))
	}
	detail := decode[Post](t, resp)
	if detail.UserGravatar != gravatarHash("alice@example.com") {
		t.Fatalf("详情作者哈希异常：%q", detail.UserGravatar)
	}
	if strings.Contains(string(resp.Data), `"email"`) {
		t.Fatalf("文章详情不应暴露邮箱：%s", resp.Data)
	}

	// 列表里按作者区分：alice 有哈希，bob 为空
	code, resp = env.do(t, http.MethodGet, "/api/post", alice.Token, nil)
	if code != http.StatusOK {
		t.Fatalf("文章列表应 200，实际 %d：%s", code, describe(resp))
	}
	posts := decode[[]Post](t, resp)
	hashes := make(map[string]string, len(posts))
	for _, post := range posts {
		hashes[post.User] = post.UserGravatar
	}
	if hashes["alice"] != gravatarHash("alice@example.com") {
		t.Fatalf("列表 alice 哈希异常：%q", hashes["alice"])
	}
	if hashes["bob"] != "" {
		t.Fatalf("bob 未设置邮箱，哈希应为空，实际 %q", hashes["bob"])
	}
	if strings.Contains(string(resp.Data), `"email"`) {
		t.Fatalf("文章列表不应暴露邮箱：%s", resp.Data)
	}
}
