package main

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func makePNGDataURL(t *testing.T, width, height int) string {
	t.Helper()

	img := image.NewRGBA(image.Rect(0, 0, width, height))
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("生成测试 PNG 失败：%v", err)
	}

	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes())
}

func (e *testEnv) raw(t *testing.T, method, path string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(method, path, nil)
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	w := httptest.NewRecorder()
	e.router.ServeHTTP(w, req)
	return w
}

func TestAvatarUploadFetchAndDelete(t *testing.T) {
	env := newTestEnv(t)

	alice := env.register(t, "alice", "alice123")
	first := makePNGDataURL(t, 2, 2)

	code, resp := env.do(t, http.MethodPut, "/api/me/avatar", alice.Token, AvatarInput{Avatar: first})
	if code != http.StatusOK {
		t.Fatalf("上传头像应 200，实际 %d：%s", code, describe(resp))
	}
	uploaded := decode[UserView](t, resp)
	if uploaded.AvatarVersion == "" {
		t.Fatalf("上传后应返回头像版本")
	}

	// /me 也要带上版本
	me := env.fetchMe(t, alice.Token)
	if me.AvatarVersion != uploaded.AvatarVersion {
		t.Fatalf("/me 头像版本不一致：%q vs %q", me.AvatarVersion, uploaded.AvatarVersion)
	}

	// 公开取图（不带 token）
	w := env.raw(t, http.MethodGet, "/api/avatar?name=alice", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("获取头像应 200，实际 %d：%s", w.Code, w.Body.String())
	}
	if got := w.Header().Get("Content-Type"); got != "image/png" {
		t.Fatalf("Content-Type 应为 image/png，实际 %q", got)
	}
	if w.Body.Len() == 0 {
		t.Fatalf("头像响应体为空")
	}
	etag := w.Header().Get("ETag")
	if etag == "" {
		t.Fatalf("应返回 ETag")
	}
	firstBody := append([]byte(nil), w.Body.Bytes()...)

	// ETag 命中返回 304
	w = env.raw(t, http.MethodGet, "/api/avatar?name=alice", map[string]string{"If-None-Match": etag})
	if w.Code != http.StatusNotModified {
		t.Fatalf("ETag 命中应 304，实际 %d", w.Code)
	}

	// 换一张图，版本与内容都应变化
	second := makePNGDataURL(t, 3, 3)
	code, resp = env.do(t, http.MethodPut, "/api/me/avatar", alice.Token, AvatarInput{Avatar: second})
	if code != http.StatusOK {
		t.Fatalf("替换头像应 200，实际 %d：%s", code, describe(resp))
	}
	replaced := decode[UserView](t, resp)
	if replaced.AvatarVersion == "" || replaced.AvatarVersion == uploaded.AvatarVersion {
		t.Fatalf("替换后头像版本应变化：%q -> %q", uploaded.AvatarVersion, replaced.AvatarVersion)
	}

	w = env.raw(t, http.MethodGet, "/api/avatar?name=alice", nil)
	if w.Code != http.StatusOK || bytes.Equal(w.Body.Bytes(), firstBody) {
		t.Fatalf("替换后应返回新的头像内容")
	}

	// 删除头像
	code, resp = env.do(t, http.MethodDelete, "/api/me/avatar", alice.Token, nil)
	if code != http.StatusOK {
		t.Fatalf("删除头像应 200，实际 %d：%s", code, describe(resp))
	}
	deleted := decode[UserView](t, resp)
	if deleted.AvatarVersion != "" {
		t.Fatalf("删除后版本应为空，实际 %q", deleted.AvatarVersion)
	}

	w = env.raw(t, http.MethodGet, "/api/avatar?name=alice", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("删除后取图应 404，实际 %d", w.Code)
	}
}

func TestAvatarValidation(t *testing.T) {
	env := newTestEnv(t)

	alice := env.register(t, "alice", "alice123")

	// 未登录
	code, _ := env.do(t, http.MethodPut, "/api/me/avatar", "", AvatarInput{Avatar: makePNGDataURL(t, 2, 2)})
	if code != http.StatusUnauthorized {
		t.Fatalf("未登录上传应 401，实际 %d", code)
	}

	tooLarge := "data:image/png;base64," + base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{0x89}, 320*1024))

	cases := []struct {
		name   string
		avatar string
		code   string
	}{
		{"空内容", "", "INVALID_AVATAR"},
		{"非 dataURL", "not-a-data-url", "INVALID_AVATAR_TYPE"},
		{"SVG", "data:image/svg+xml;base64,PHN2Zy8+", "INVALID_AVATAR_TYPE"},
		{"坏 base64", "data:image/png;base64,not-base64!!", "INVALID_AVATAR"},
		{"超过字节上限", tooLarge, "AVATAR_TOO_LARGE"},
		{"超过尺寸上限", makePNGDataURL(t, 600, 600), "AVATAR_TOO_LARGE"},
	}

	for _, tc := range cases {
		code, resp := env.do(t, http.MethodPut, "/api/me/avatar", alice.Token, AvatarInput{Avatar: tc.avatar})
		if code != http.StatusBadRequest || resp.Error == nil || resp.Error.Code != tc.code {
			t.Fatalf("%s 应 400 %s，实际 %d：%s", tc.name, tc.code, code, describe(resp))
		}
	}

	// 失败不应写入头像
	if me := env.fetchMe(t, alice.Token); me.AvatarVersion != "" {
		t.Fatalf("非法上传后不应有头像版本，实际 %q", me.AvatarVersion)
	}
}

func TestPostAvatarVersion(t *testing.T) {
	env := newTestEnv(t)

	alice := env.register(t, "alice", "alice123")
	bob := env.register(t, "bob", "bob12345")

	code, resp := env.do(t, http.MethodPut, "/api/me/avatar", alice.Token, AvatarInput{Avatar: makePNGDataURL(t, 4, 4)})
	if code != http.StatusOK {
		t.Fatalf("上传头像应 200，实际 %d：%s", code, describe(resp))
	}
	version := decode[UserView](t, resp).AvatarVersion

	alicePost := env.createPost(t, alice.Token, PostInput{Title: "alice", Body: "x"})
	env.createPost(t, bob.Token, PostInput{Title: "bob", Body: "x"})

	code, resp = env.do(t, http.MethodGet, "/api/post", bob.Token, nil)
	if code != http.StatusOK {
		t.Fatalf("文章列表应 200，实际 %d：%s", code, describe(resp))
	}
	posts := decode[[]Post](t, resp)
	versions := make(map[string]string, len(posts))
	for _, post := range posts {
		versions[post.User] = post.UserAvatarVersion
	}
	if versions["alice"] != version {
		t.Fatalf("alice 文章头像版本异常：%q", versions["alice"])
	}
	if versions["bob"] != "" {
		t.Fatalf("bob 没有头像，版本应为空：%q", versions["bob"])
	}
	if strings.Contains(string(resp.Data), "data:image") || strings.Contains(string(resp.Data), "base64") {
		t.Fatalf("文章列表不应包含头像 base64：%s", resp.Data)
	}

	code, resp = env.do(t, http.MethodGet, "/api/post/"+idString(alicePost.ID), bob.Token, nil)
	if code != http.StatusOK {
		t.Fatalf("文章详情应 200，实际 %d：%s", code, describe(resp))
	}
	if detail := decode[Post](t, resp); detail.UserAvatarVersion != version {
		t.Fatalf("详情头像版本异常：%q", detail.UserAvatarVersion)
	}

	// 中文用户名也能通过 query 正常取图
	chinese := env.register(t, "头像用户", "china123")
	code, resp = env.do(t, http.MethodPut, "/api/me/avatar", chinese.Token, AvatarInput{Avatar: makePNGDataURL(t, 2, 2)})
	if code != http.StatusOK {
		t.Fatalf("中文用户名上传头像应 200，实际 %d：%s", code, describe(resp))
	}
	w := env.raw(t, http.MethodGet, "/api/avatar?name="+url.QueryEscape("头像用户"), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("中文用户名取图应 200，实际 %d", w.Code)
	}
}
