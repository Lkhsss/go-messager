package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// testEnv 用临时 SQLite 文件 + gin 路由做接口级测试，不占用真实端口。
type testEnv struct {
	router *gin.Engine
	db     *gorm.DB
}

type apiResponse struct {
	Success bool            `json:"success"`
	Data    json.RawMessage `json:"data"`
	Error   *ErrorInfo      `json:"error"`
	Meta    *Meta           `json:"meta"`
}

func newTestEnv(t *testing.T) *testEnv {
	t.Helper()

	gin.SetMode(gin.TestMode)

	db, err := gorm.Open(
		sqlite.Open(filepath.Join(t.TempDir(), "test.db")),
		&gorm.Config{Logger: logger.Default.LogMode(logger.Silent)},
	)
	if err != nil {
		t.Fatalf("打开测试数据库失败：%v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("获取底层连接失败：%v", err)
	}
	// 测试结束先关闭连接，否则 Windows 上临时目录删不掉
	t.Cleanup(func() {
		_ = sqlDB.Close()
	})

	if err := db.AutoMigrate(&User{}, &Post{}, &Tag{}, &Reaction{}, &Setting{}); err != nil {
		t.Fatalf("建表失败：%v", err)
	}

	r := gin.New()
	if err := api(r, db); err != nil {
		t.Fatalf("挂载路由失败：%v", err)
	}

	return &testEnv{router: r, db: db}
}

func (e *testEnv) do(t *testing.T, method, path, token string, body any) (int, apiResponse) {
	t.Helper()

	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("序列化请求体失败：%v", err)
		}
		reader = bytes.NewReader(raw)
	}

	req := httptest.NewRequest(method, path, reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	w := httptest.NewRecorder()
	e.router.ServeHTTP(w, req)

	var resp apiResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败[%d %s]：%v", w.Code, w.Body.String(), err)
	}

	return w.Code, resp
}

func (e *testEnv) register(t *testing.T, name, password string) AuthResult {
	t.Helper()

	code, resp := e.do(t, http.MethodPost, "/api/register", "", Credentials{Name: name, Password: password})
	if code != http.StatusCreated {
		t.Fatalf("注册[%s]期望 201，实际 %d：%s", name, code, describe(resp))
	}

	var result AuthResult
	if err := json.Unmarshal(resp.Data, &result); err != nil {
		t.Fatalf("解析注册响应失败：%v", err)
	}
	return result
}

func (e *testEnv) createTag(t *testing.T, token, name string) Tag {
	t.Helper()
	return e.createTagWithColor(t, token, name, "")
}

func (e *testEnv) createTagWithColor(t *testing.T, token, name, color string) Tag {
	t.Helper()

	code, resp := e.do(t, http.MethodPost, "/api/tag", token, TagInput{Name: name, Color: color})
	if code != http.StatusCreated {
		t.Fatalf("创建标签[%s]期望 201，实际 %d：%s", name, code, describe(resp))
	}

	var tag Tag
	if err := json.Unmarshal(resp.Data, &tag); err != nil {
		t.Fatalf("解析标签响应失败：%v", err)
	}
	return tag
}

func describe(resp apiResponse) string {
	if resp.Error == nil {
		return string(resp.Data)
	}
	return resp.Error.Code + ": " + resp.Error.Message
}

func decode[T any](t *testing.T, resp apiResponse) T {
	t.Helper()

	var out T
	if err := json.Unmarshal(resp.Data, &out); err != nil {
		t.Fatalf("解析 data 失败：%v", err)
	}
	return out
}

func idString(id uint) string {
	return strconv.FormatUint(uint64(id), 10)
}

func TestRolesAndTagPermissions(t *testing.T) {
	env := newTestEnv(t)

	admin := env.register(t, "admin", "admin123")
	if admin.User.Role != RoleAdmin {
		t.Fatalf("第一个用户应为 admin，实际 %s", admin.User.Role)
	}

	user := env.register(t, "bob", "bob12345")
	if user.User.Role != RoleUser {
		t.Fatalf("第二个用户应为 user，实际 %s", user.User.Role)
	}

	// 普通用户不能创建标签
	code, resp := env.do(t, http.MethodPost, "/api/tag", user.Token, TagInput{Name: "Go"})
	if code != http.StatusForbidden || resp.Error == nil || resp.Error.Code != "FORBIDDEN" {
		t.Fatalf("普通用户创建标签应 403 FORBIDDEN，实际 %d：%s", code, describe(resp))
	}

	// 管理员可以创建
	tag := env.createTag(t, admin.Token, "Go")
	if tag.Name != "Go" || tag.ID == 0 {
		t.Fatalf("标签返回异常：%+v", tag)
	}

	// 重名冲突（首尾空格会被去掉）
	code, resp = env.do(t, http.MethodPost, "/api/tag", admin.Token, TagInput{Name: " Go "})
	if code != http.StatusConflict || resp.Error == nil || resp.Error.Code != "TAG_EXIST" {
		t.Fatalf("重名标签应 409 TAG_EXIST，实际 %d：%s", code, describe(resp))
	}

	// 未登录不能读标签
	code, _ = env.do(t, http.MethodGet, "/api/tag", "", nil)
	if code != http.StatusUnauthorized {
		t.Fatalf("未登录读取标签应 401，实际 %d", code)
	}

	// 普通用户可以读标签
	code, resp = env.do(t, http.MethodGet, "/api/tag", user.Token, nil)
	if code != http.StatusOK {
		t.Fatalf("读取标签应 200，实际 %d：%s", code, describe(resp))
	}
	tags := decode[[]Tag](t, resp)
	if len(tags) != 1 || tags[0].Name != "Go" {
		t.Fatalf("标签列表异常：%+v", tags)
	}
}

func (e *testEnv) createPost(t *testing.T, token string, body any) Post {
	t.Helper()

	code, resp := e.do(t, http.MethodPost, "/api/post/new", token, body)
	if code != http.StatusCreated {
		t.Fatalf("创建文章期望 201，实际 %d：%s", code, describe(resp))
	}

	return decode[Post](t, resp)
}

func TestPostTagsLimit(t *testing.T) {
	env := newTestEnv(t)

	admin := env.register(t, "admin", "admin123")
	user := env.register(t, "bob", "bob12345")

	tagNames := []string{"Go", "Gin", "GORM", "SQLite", "Vue", "Tailwind"}
	tags := make([]Tag, 0, len(tagNames))
	for _, name := range tagNames {
		tags = append(tags, env.createTag(t, admin.Token, name))
	}

	ids := func(n int) []uint {
		out := make([]uint, 0, n)
		for i := 0; i < n; i++ {
			out = append(out, tags[i].ID)
		}
		return out
	}

	// 超过 5 个
	code, resp := env.do(t, http.MethodPost, "/api/post/new", user.Token, PostInput{
		Title: "too many", Body: "body", TagIDs: ids(6),
	})
	if code != http.StatusBadRequest || resp.Error == nil || resp.Error.Code != "TOO_MANY_TAGS" {
		t.Fatalf("6 个标签应 400 TOO_MANY_TAGS，实际 %d：%s", code, describe(resp))
	}

	// 不存在的标签
	code, resp = env.do(t, http.MethodPost, "/api/post/new", user.Token, PostInput{
		Title: "bad tag", Body: "body", TagIDs: []uint{99999},
	})
	if code != http.StatusBadRequest || resp.Error == nil || resp.Error.Code != "INVALID_TAGS" {
		t.Fatalf("不存在的标签应 400 INVALID_TAGS，实际 %d：%s", code, describe(resp))
	}

	// 刚好 5 个
	post := env.createPost(t, user.Token, PostInput{
		Title: "hello", Body: "content", TagIDs: ids(5),
	})
	if post.User != "bob" {
		t.Fatalf("作者应取自 token(bob)，实际 %s", post.User)
	}
	if len(post.Tags) != 5 {
		t.Fatalf("文章应有 5 个标签，实际 %d", len(post.Tags))
	}

	// 重复 ID 去重后不算超限
	deduped := env.createPost(t, user.Token, PostInput{
		Title: "dedupe", Body: "content",
		TagIDs: []uint{tags[0].ID, tags[0].ID, tags[0].ID, tags[0].ID, tags[0].ID, tags[0].ID},
	})
	if len(deduped.Tags) != 1 {
		t.Fatalf("重复标签应去重为 1 个，实际 %d", len(deduped.Tags))
	}

	// 使用次数：tags[0] 被两篇文章使用，其余各 1
	counts := make(map[uint]int64)
	var stored []Tag
	if err := env.db.Find(&stored).Error; err != nil {
		t.Fatalf("查询标签失败：%v", err)
	}
	for _, tag := range stored {
		counts[tag.ID] = tag.UsageCount
	}
	if counts[tags[0].ID] != 2 {
		t.Fatalf("tags[0] 使用次数应为 2，实际 %d", counts[tags[0].ID])
	}
	if counts[tags[1].ID] != 1 {
		t.Fatalf("tags[1] 使用次数应为 1，实际 %d", counts[tags[1].ID])
	}
}

func TestPostTagQueries(t *testing.T) {
	env := newTestEnv(t)

	admin := env.register(t, "admin", "admin123")
	user := env.register(t, "bob", "bob12345")

	firstTag := env.createTag(t, admin.Token, "Go")
	secondTag := env.createTag(t, admin.Token, "Gin")

	// 请求体里伪造 user，服务端应忽略
	first := env.createPost(t, user.Token, map[string]any{
		"user":    "hacker",
		"title":   "first",
		"body":    "content",
		"tag_ids": []uint{firstTag.ID, secondTag.ID},
	})
	env.createPost(t, user.Token, PostInput{
		Title: "second", Body: "content", TagIDs: []uint{firstTag.ID},
	})

	if first.User != "bob" {
		t.Fatalf("作者应取自 token(bob)，实际 %s", first.User)
	}

	// post → tags：列表带标签
	code, resp := env.do(t, http.MethodGet, "/api/post", user.Token, nil)
	if code != http.StatusOK {
		t.Fatalf("文章列表应 200，实际 %d：%s", code, describe(resp))
	}
	posts := decode[[]Post](t, resp)
	if len(posts) != 2 {
		t.Fatalf("文章列表应有 2 篇，实际 %d", len(posts))
	}
	for _, post := range posts {
		if len(post.Tags) == 0 {
			t.Fatalf("文章列表缺少标签：%+v", post)
		}
	}

	// post → tags：详情带标签
	code, resp = env.do(t, http.MethodGet, "/api/post/"+idString(first.ID), user.Token, nil)
	if code != http.StatusOK {
		t.Fatalf("文章详情应 200，实际 %d：%s", code, describe(resp))
	}
	detail := decode[Post](t, resp)
	if detail.ID != first.ID || len(detail.Tags) != 2 {
		t.Fatalf("文章详情异常：%+v", detail)
	}

	// 按 include_tag_ids 过滤（单个标签）
	code, resp = env.do(t, http.MethodGet, "/api/post?include_tag_ids="+idString(secondTag.ID), user.Token, nil)
	if code != http.StatusOK {
		t.Fatalf("按标签过滤应 200，实际 %d：%s", code, describe(resp))
	}
	filtered := decode[[]Post](t, resp)
	if len(filtered) != 1 || filtered[0].ID != first.ID {
		t.Fatalf("按标签过滤结果异常：%+v", filtered)
	}

	// 不存在的文章
	code, resp = env.do(t, http.MethodGet, "/api/post/99999", user.Token, nil)
	if code != http.StatusNotFound || resp.Error == nil || resp.Error.Code != "POST_NOT_FOUND" {
		t.Fatalf("不存在的文章应 404 POST_NOT_FOUND，实际 %d：%s", code, describe(resp))
	}
}

func postIDSet(posts []Post) []uint {
	ids := make([]uint, 0, len(posts))
	for _, post := range posts {
		ids = append(ids, post.ID)
	}
	return ids
}

func sameIDSet(got, want []uint) bool {
	if len(got) != len(want) {
		return false
	}
	counts := make(map[uint]int, len(got))
	for _, id := range got {
		counts[id]++
	}
	for _, id := range want {
		if counts[id] == 0 {
			return false
		}
		counts[id]--
	}
	return true
}

func TestPostTagFilters(t *testing.T) {
	env := newTestEnv(t)

	admin := env.register(t, "admin", "admin123")
	user := env.register(t, "bob", "bob12345")

	a := env.createTag(t, admin.Token, "A")
	b := env.createTag(t, admin.Token, "B")
	c := env.createTag(t, admin.Token, "C")

	p1 := env.createPost(t, user.Token, PostInput{Title: "p1", Body: "x", TagIDs: []uint{a.ID, b.ID}})
	p2 := env.createPost(t, user.Token, PostInput{Title: "p2", Body: "x", TagIDs: []uint{a.ID}})
	p3 := env.createPost(t, user.Token, PostInput{Title: "p3", Body: "x", TagIDs: []uint{b.ID, c.ID}})
	p4 := env.createPost(t, user.Token, PostInput{Title: "p4", Body: "x", TagIDs: []uint{c.ID}})

	query := func(rawQuery string) []Post {
		t.Helper()
		code, resp := env.do(t, http.MethodGet, "/api/post?"+rawQuery, user.Token, nil)
		if code != http.StatusOK {
			t.Fatalf("筛选[%s]应 200，实际 %d：%s", rawQuery, code, describe(resp))
		}
		return decode[[]Post](t, resp)
	}

	cases := []struct {
		name  string
		query string
		want  []uint
	}{
		{"单标签包含", "include_tag_ids=" + idString(a.ID), []uint{p1.ID, p2.ID}},
		{"多标签 AND", "include_tag_ids=" + idString(a.ID) + "," + idString(b.ID), []uint{p1.ID}},
		{"排除", "exclude_tag_ids=" + idString(c.ID), []uint{p1.ID, p2.ID}},
		{"包含+排除", "include_tag_ids=" + idString(a.ID) + "&exclude_tag_ids=" + idString(b.ID), []uint{p2.ID}},
		{"冲突时排除优先", "include_tag_ids=" + idString(a.ID) + "&exclude_tag_ids=" + idString(a.ID), []uint{p3.ID, p4.ID}},
		{"尾随逗号", "include_tag_ids=" + idString(a.ID) + ",", []uint{p1.ID, p2.ID}},
	}

	for _, tc := range cases {
		got := postIDSet(query(tc.query))
		if !sameIDSet(got, tc.want) {
			t.Fatalf("%s：期望 %v，实际 %v", tc.name, tc.want, got)
		}
	}

	// 非法格式
	for _, rawQuery := range []string{"include_tag_ids=abc", "exclude_tag_ids=0", "include_tag_ids=1,,x"} {
		code, resp := env.do(t, http.MethodGet, "/api/post?"+rawQuery, user.Token, nil)
		if code != http.StatusBadRequest || resp.Error == nil || resp.Error.Code != "INVALID_TAG_FILTER" {
			t.Fatalf("[%s] 应 400 INVALID_TAG_FILTER，实际 %d：%s", rawQuery, code, describe(resp))
		}
	}

	// 超过上限
	many := make([]string, 0, 21)
	for i := 1; i <= 21; i++ {
		many = append(many, strconv.Itoa(i))
	}
	code, resp := env.do(t, http.MethodGet, "/api/post?include_tag_ids="+strings.Join(many, ","), user.Token, nil)
	if code != http.StatusBadRequest || resp.Error == nil || resp.Error.Code != "TOO_MANY_FILTERS" {
		t.Fatalf("21 个标签应 400 TOO_MANY_FILTERS，实际 %d：%s", code, describe(resp))
	}
}

func TestTagColorValidation(t *testing.T) {
	env := newTestEnv(t)

	admin := env.register(t, "admin", "admin123")

	short := env.createTagWithColor(t, admin.Token, "Short", "#ABC")
	if short.Color != "#aabbcc" {
		t.Fatalf("#ABC 应归一化为 #aabbcc，实际 %s", short.Color)
	}

	noHash := env.createTagWithColor(t, admin.Token, "NoHash", "1F2937")
	if noHash.Color != "#1f2937" {
		t.Fatalf("1F2937 应归一化为 #1f2937，实际 %s", noHash.Color)
	}

	fallback := env.createTag(t, admin.Token, "Default")
	if fallback.Color != "" {
		t.Fatalf("不传颜色应为空字符串，实际 %s", fallback.Color)
	}

	for _, color := range []string{"red", "#12345", "#1234567", "#gggggg"} {
		code, resp := env.do(t, http.MethodPost, "/api/tag", admin.Token, TagInput{Name: "bad-" + color, Color: color})
		if code != http.StatusBadRequest || resp.Error == nil || resp.Error.Code != "INVALID_COLOR" {
			t.Fatalf("颜色[%s]应 400 INVALID_COLOR，实际 %d：%s", color, code, describe(resp))
		}
	}
}

func TestTagSearchAndHotSort(t *testing.T) {
	env := newTestEnv(t)

	admin := env.register(t, "admin", "admin123")
	user := env.register(t, "bob", "bob12345")

	goTag := env.createTag(t, admin.Token, "Go")
	ginTag := env.createTag(t, admin.Token, "Gin")
	vueTag := env.createTag(t, admin.Token, "Vue")

	// Go 使用 2 次，Gin 1 次，Vue 0 次
	env.createPost(t, user.Token, PostInput{Title: "p1", Body: "x", TagIDs: []uint{goTag.ID, ginTag.ID}})
	env.createPost(t, user.Token, PostInput{Title: "p2", Body: "x", TagIDs: []uint{goTag.ID}})

	getTags := func(rawQuery string) []Tag {
		t.Helper()
		code, resp := env.do(t, http.MethodGet, "/api/tag"+rawQuery, user.Token, nil)
		if code != http.StatusOK {
			t.Fatalf("标签列表[%s]应 200，实际 %d：%s", rawQuery, code, describe(resp))
		}
		return decode[[]Tag](t, resp)
	}

	hot := getTags("?sort=hot")
	if len(hot) != 3 || hot[0].ID != goTag.ID || hot[1].ID != ginTag.ID || hot[2].ID != vueTag.ID {
		t.Fatalf("热门排序异常：%+v", hot)
	}
	if hot[0].UsageCount != 2 || hot[1].UsageCount != 1 || hot[2].UsageCount != 0 {
		t.Fatalf("使用次数异常：%+v", hot)
	}

	limited := getTags("?sort=hot&limit=2")
	if len(limited) != 2 || limited[0].ID != goTag.ID || limited[1].ID != ginTag.ID {
		t.Fatalf("limit=2 结果异常：%+v", limited)
	}

	// 搜索大小写不敏感
	found := getTags("?q=go")
	if len(found) != 1 || found[0].ID != goTag.ID {
		t.Fatalf("搜索 go 结果异常：%+v", found)
	}

	// 通配符按字面量处理（q=%25 => "%"）
	if none := getTags("?q=%25"); len(none) != 0 {
		t.Fatalf("搜索 %% 应无结果，实际 %+v", none)
	}
}

func TestSyncTagUsageCounts(t *testing.T) {
	env := newTestEnv(t)

	admin := env.register(t, "admin", "admin123")
	user := env.register(t, "bob", "bob12345")

	tag := env.createTag(t, admin.Token, "Go")
	post := env.createPost(t, user.Token, PostInput{Title: "p", Body: "x", TagIDs: []uint{tag.ID}})

	var stored Tag
	if err := env.db.First(&stored, tag.ID).Error; err != nil {
		t.Fatalf("查询标签失败：%v", err)
	}
	if stored.UsageCount != 1 {
		t.Fatalf("发帖后使用次数应为 1，实际 %d", stored.UsageCount)
	}

	// 软删除文章后校准，计数应回到 0
	if err := env.db.Delete(&Post{}, post.ID).Error; err != nil {
		t.Fatalf("软删除文章失败：%v", err)
	}
	if err := syncTagUsageCounts(env.db); err != nil {
		t.Fatalf("校准使用次数失败：%v", err)
	}
	if err := env.db.First(&stored, tag.ID).Error; err != nil {
		t.Fatalf("重新查询标签失败：%v", err)
	}
	if stored.UsageCount != 0 {
		t.Fatalf("软删除后使用次数应为 0，实际 %d", stored.UsageCount)
	}
}

func TestUpdateTag(t *testing.T) {
	env := newTestEnv(t)

	admin := env.register(t, "admin", "admin123")
	user := env.register(t, "bob", "bob12345")

	tag := env.createTagWithColor(t, admin.Token, "Go", "#3b82f6")
	env.createPost(t, user.Token, PostInput{Title: "p", Body: "x", TagIDs: []uint{tag.ID}})
	other := env.createTag(t, admin.Token, "Gin")

	path := "/api/tag/" + idString(tag.ID)

	// 普通用户不能编辑
	code, resp := env.do(t, http.MethodPut, path, user.Token, TagInput{Name: "Golang"})
	if code != http.StatusForbidden || resp.Error == nil || resp.Error.Code != "FORBIDDEN" {
		t.Fatalf("普通用户编辑标签应 403 FORBIDDEN，实际 %d：%s", code, describe(resp))
	}

	// ID 非法 / 标签不存在
	code, resp = env.do(t, http.MethodPut, "/api/tag/abc", admin.Token, TagInput{Name: "Golang"})
	if code != http.StatusBadRequest || resp.Error == nil || resp.Error.Code != "INVALID_ID" {
		t.Fatalf("非法 ID 应 400 INVALID_ID，实际 %d：%s", code, describe(resp))
	}
	code, resp = env.do(t, http.MethodPut, "/api/tag/99999", admin.Token, TagInput{Name: "Golang"})
	if code != http.StatusNotFound || resp.Error == nil || resp.Error.Code != "TAG_NOT_FOUND" {
		t.Fatalf("不存在的标签应 404 TAG_NOT_FOUND，实际 %d：%s", code, describe(resp))
	}

	// 名称长度 / 颜色校验
	code, resp = env.do(t, http.MethodPut, path, admin.Token, TagInput{Name: strings.Repeat("a", 31)})
	if code != http.StatusBadRequest || resp.Error == nil || resp.Error.Code != "INVALID_NAME" {
		t.Fatalf("超长名称应 400 INVALID_NAME，实际 %d：%s", code, describe(resp))
	}
	code, resp = env.do(t, http.MethodPut, path, admin.Token, TagInput{Name: "Golang", Color: "#12345"})
	if code != http.StatusBadRequest || resp.Error == nil || resp.Error.Code != "INVALID_COLOR" {
		t.Fatalf("非法颜色应 400 INVALID_COLOR，实际 %d：%s", code, describe(resp))
	}

	// 不能改成其他标签的名字
	code, resp = env.do(t, http.MethodPut, path, admin.Token, TagInput{Name: other.Name})
	if code != http.StatusConflict || resp.Error == nil || resp.Error.Code != "TAG_EXIST" {
		t.Fatalf("重名应 409 TAG_EXIST，实际 %d：%s", code, describe(resp))
	}

	// 正常更新：去空格、#ABC 归一化为 #aabbcc、使用次数保持不变
	code, resp = env.do(t, http.MethodPut, path, admin.Token, TagInput{Name: " Golang ", Color: "#ABC"})
	if code != http.StatusOK {
		t.Fatalf("编辑标签应 200，实际 %d：%s", code, describe(resp))
	}
	updated := decode[Tag](t, resp)
	if updated.ID != tag.ID || updated.Name != "Golang" || updated.Color != "#aabbcc" {
		t.Fatalf("编辑结果异常：%+v", updated)
	}
	if updated.UsageCount != 1 {
		t.Fatalf("编辑后使用次数应保持 1，实际 %d", updated.UsageCount)
	}

	// 保持原名只改颜色不应误判重名；color 传空表示恢复默认灰
	code, resp = env.do(t, http.MethodPut, path, admin.Token, TagInput{Name: "Golang", Color: ""})
	if code != http.StatusOK {
		t.Fatalf("保持原名的编辑应 200，实际 %d：%s", code, describe(resp))
	}
	updated = decode[Tag](t, resp)
	if updated.Color != "" || updated.Name != "Golang" || updated.UsageCount != 1 {
		t.Fatalf("清空颜色结果异常：%+v", updated)
	}

	var stored Tag
	if err := env.db.First(&stored, tag.ID).Error; err != nil {
		t.Fatalf("查询标签失败：%v", err)
	}
	if stored.Name != "Golang" || stored.Color != "" {
		t.Fatalf("数据库中的标签未更新：%+v", stored)
	}
}

func TestDeleteTag(t *testing.T) {
	env := newTestEnv(t)

	admin := env.register(t, "admin", "admin123")
	user := env.register(t, "bob", "bob12345")

	tag := env.createTag(t, admin.Token, "Go")
	other := env.createTag(t, admin.Token, "Gin")
	post := env.createPost(t, user.Token, PostInput{
		Title: "p", Body: "x", TagIDs: []uint{tag.ID, other.ID},
	})

	path := "/api/tag/" + idString(tag.ID)

	// 普通用户不能删除
	code, resp := env.do(t, http.MethodDelete, path, user.Token, nil)
	if code != http.StatusForbidden || resp.Error == nil || resp.Error.Code != "FORBIDDEN" {
		t.Fatalf("普通用户删除标签应 403 FORBIDDEN，实际 %d：%s", code, describe(resp))
	}

	// ID 非法 / 标签不存在
	code, resp = env.do(t, http.MethodDelete, "/api/tag/abc", admin.Token, nil)
	if code != http.StatusBadRequest || resp.Error == nil || resp.Error.Code != "INVALID_ID" {
		t.Fatalf("非法 ID 应 400 INVALID_ID，实际 %d：%s", code, describe(resp))
	}
	code, resp = env.do(t, http.MethodDelete, "/api/tag/99999", admin.Token, nil)
	if code != http.StatusNotFound || resp.Error == nil || resp.Error.Code != "TAG_NOT_FOUND" {
		t.Fatalf("不存在的标签应 404 TAG_NOT_FOUND，实际 %d：%s", code, describe(resp))
	}

	// 管理员删除正在被文章使用的标签
	code, resp = env.do(t, http.MethodDelete, path, admin.Token, nil)
	if code != http.StatusOK {
		t.Fatalf("删除标签应 200，实际 %d：%s", code, describe(resp))
	}

	// 标签硬删除，post_tags 关联也被清理
	var count int64
	if err := env.db.Unscoped().Model(&Tag{}).Where("id = ?", tag.ID).Count(&count).Error; err != nil {
		t.Fatalf("查询标签失败：%v", err)
	}
	if count != 0 {
		t.Fatalf("标签应被硬删除，实际仍存在 %d 条", count)
	}
	if err := env.db.Table("post_tags").Where("tag_id = ?", tag.ID).Count(&count).Error; err != nil {
		t.Fatalf("查询关联表失败：%v", err)
	}
	if count != 0 {
		t.Fatalf("post_tags 关联应被清理，实际仍有 %d 条", count)
	}

	// 列表只剩 Gin
	code, resp = env.do(t, http.MethodGet, "/api/tag", admin.Token, nil)
	if code != http.StatusOK {
		t.Fatalf("标签列表应 200，实际 %d：%s", code, describe(resp))
	}
	remaining := decode[[]Tag](t, resp)
	if len(remaining) != 1 || remaining[0].ID != other.ID {
		t.Fatalf("删除后标签列表异常：%+v", remaining)
	}

	// 文章本身还在，只是不再带被删除的标签
	code, resp = env.do(t, http.MethodGet, "/api/post/"+idString(post.ID), user.Token, nil)
	if code != http.StatusOK {
		t.Fatalf("文章详情应 200，实际 %d：%s", code, describe(resp))
	}
	got := decode[Post](t, resp)
	if len(got.Tags) != 1 || got.Tags[0].ID != other.ID {
		t.Fatalf("删除标签后文章标签异常：%+v", got.Tags)
	}

	// 硬删除后可以重新创建同名标签
	recreated := env.createTag(t, admin.Token, "Go")
	if recreated.ID == 0 || recreated.Name != "Go" {
		t.Fatalf("重建同名标签失败：%+v", recreated)
	}
}
