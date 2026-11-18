package main

import (
	"net/http"
	"strings"
	"testing"
)

func TestExtractBodyTags(t *testing.T) {
	cases := []struct {
		name string
		body string
		want []string
	}{
		{"中文与英文", "今天聊 #Go 和 #后端开发。", []string{"Go", "后端开发"}},
		{"行首", "#行首标签 开头", []string{"行首标签"}},
		{"括号后", "这是（#括注）标签", []string{"括注"}},
		{"代码块忽略", "```\n#code\n```\n正文 #真实", []string{"真实"}},
		{"行内代码忽略", "`#inline` 与 #真实", []string{"真实"}},
		{"图片忽略", "![图#片](https://x/#a) 后 #真实", []string{"真实"}},
		{"链接忽略", "[链接#文字](https://x/#a) 后 #真实", []string{"真实"}},
		{"URL 片段忽略", "https://x/#anchor 后 #真实", []string{"真实"}},
		{"单词内忽略", "a#b 与 C#", nil},
		{"标题忽略", "## 标题", nil},
		{"尾部连字符", "#tag- 和 #tag_", []string{"tag"}},
		{"只有符号", "#--- #___", nil},
		{"去重保序", "#Go #Gin #Go", []string{"Go", "Gin"}},
		{"连续标签链", "#TEST#自动标签#ZZZ", []string{"TEST", "自动标签", "ZZZ"}},
		{"链内单字符", "#a#b", []string{"a", "b"}},
		{"双井号不识别", "#a##b", []string{"a"}},
		{"链内尾部连字符", "#a-#b", []string{"a", "b"}},
		{"链末尾井号", "#TEST#", []string{"TEST"}},
		{"URL 链片段忽略", "https://x/#a#b", nil},
		{"超长截断", "#" + strings.Repeat("a", 31), []string{strings.Repeat("a", 30)}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := extractBodyTags(tc.body)
			if len(got) != len(tc.want) {
				t.Fatalf("extractBodyTags(%q) = %v，期望 %v", tc.body, got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("extractBodyTags(%q) = %v，期望 %v", tc.body, got, tc.want)
				}
			}
		})
	}
}

func TestAutoTagsFromPostBody(t *testing.T) {
	env := newTestEnv(t)

	admin := env.register(t, "admin", "admin123")
	user := env.register(t, "bob", "bob12345")

	post := env.createPost(t, user.Token, PostInput{
		Title: "自动标签",
		Body:  "正文里写 #Go 和 #Gin。",
	})
	if len(post.Tags) != 0 {
		t.Fatalf("未审核标签不应出现在文章响应中，实际 %+v", post.Tags)
	}

	var goTag Tag
	if err := env.db.Where("name = ?", "Go").Take(&goTag).Error; err != nil {
		t.Fatalf("自动标签 Go 应已创建：%v", err)
	}
	if goTag.Reviewed {
		t.Fatal("自动创建的标签应为待审核")
	}
	if goTag.UsageCount != 1 {
		t.Fatalf("自动标签使用次数应为 1，实际 %d", goTag.UsageCount)
	}

	// 首页筛选用 /api/tag，不含待审核标签。
	code, resp := env.do(t, http.MethodGet, "/api/tag", user.Token, nil)
	if code != http.StatusOK {
		t.Fatalf("获取标签应 200，实际 %d：%s", code, describe(resp))
	}
	if tags := decode[[]Tag](t, resp); len(tags) != 0 {
		t.Fatalf("未审核标签不应出现在 /api/tag，实际 %+v", tags)
	}

	// 管理端能看到待审核标签。
	code, resp = env.do(t, http.MethodGet, "/api/admin/tags", admin.Token, nil)
	if code != http.StatusOK {
		t.Fatalf("管理端标签列表应 200，实际 %d：%s", code, describe(resp))
	}
	if adminTags := decode[[]Tag](t, resp); len(adminTags) != 2 {
		t.Fatalf("管理端应看到 2 个待审核标签，实际 %+v", adminTags)
	}

	reviewPath := "/api/admin/tags/" + idString(goTag.ID) + "/review"

	// 普通用户不能审核。
	code, resp = env.do(t, http.MethodPatch, reviewPath, user.Token, map[string]any{"reviewed": true})
	if code != http.StatusForbidden {
		t.Fatalf("普通用户审核应 403，实际 %d：%s", code, describe(resp))
	}

	// 审核通过后出现在筛选和文章标签区。
	code, resp = env.do(t, http.MethodPatch, reviewPath, admin.Token, map[string]any{"reviewed": true})
	if code != http.StatusOK {
		t.Fatalf("审核标签应 200，实际 %d：%s", code, describe(resp))
	}

	code, resp = env.do(t, http.MethodGet, "/api/tag", user.Token, nil)
	if code != http.StatusOK {
		t.Fatalf("获取标签应 200，实际 %d：%s", code, describe(resp))
	}
	approved := decode[[]Tag](t, resp)
	if len(approved) != 1 || approved[0].Name != "Go" || !approved[0].Reviewed {
		t.Fatalf("审核通过后 /api/tag 应只返回 Go，实际 %+v", approved)
	}

	code, resp = env.do(t, http.MethodGet, "/api/post/"+idString(post.ID), user.Token, nil)
	if code != http.StatusOK {
		t.Fatalf("获取文章应 200，实际 %d：%s", code, describe(resp))
	}
	detail := decode[Post](t, resp)
	if len(detail.Tags) != 1 || detail.Tags[0].Name != "Go" {
		t.Fatalf("审核通过后文章应返回标签 Go，实际 %+v", detail.Tags)
	}

	// 取消审核后再次从筛选与文章标签区消失。
	code, resp = env.do(t, http.MethodPatch, reviewPath, admin.Token, map[string]any{"reviewed": false})
	if code != http.StatusOK {
		t.Fatalf("取消审核应 200，实际 %d：%s", code, describe(resp))
	}

	code, resp = env.do(t, http.MethodGet, "/api/tag", user.Token, nil)
	if code != http.StatusOK {
		t.Fatalf("获取标签应 200，实际 %d：%s", code, describe(resp))
	}
	if tags := decode[[]Tag](t, resp); len(tags) != 0 {
		t.Fatalf("取消审核后 /api/tag 应为空，实际 %+v", tags)
	}

	code, resp = env.do(t, http.MethodGet, "/api/post/"+idString(post.ID), user.Token, nil)
	if code != http.StatusOK {
		t.Fatalf("获取文章应 200，实际 %d：%s", code, describe(resp))
	}
	detail = decode[Post](t, resp)
	if len(detail.Tags) != 0 {
		t.Fatalf("取消审核后文章不应返回该标签，实际 %+v", detail.Tags)
	}
}

func TestAutoTagReusesManualTag(t *testing.T) {
	env := newTestEnv(t)

	admin := env.register(t, "admin", "admin123")
	user := env.register(t, "bob", "bob12345")

	manual := env.createTag(t, admin.Token, "Go")
	post := env.createPost(t, user.Token, PostInput{Title: "复用", Body: "#Go 正文"})

	var count int64
	if err := env.db.Model(&Tag{}).Where("name = ?", "Go").Count(&count).Error; err != nil {
		t.Fatalf("统计标签失败：%v", err)
	}
	if count != 1 {
		t.Fatalf("同名标签不应重复创建，实际 %d 条", count)
	}

	if len(post.Tags) != 1 || post.Tags[0].ID != manual.ID {
		t.Fatalf("文章应复用已审核标签 Go，实际 %+v", post.Tags)
	}

	var stored Tag
	if err := env.db.Take(&stored, manual.ID).Error; err != nil {
		t.Fatalf("查询标签失败：%v", err)
	}
	if stored.UsageCount != 1 {
		t.Fatalf("Go 使用次数应为 1，实际 %d", stored.UsageCount)
	}
}

func TestMergeManualAndAutoTags(t *testing.T) {
	env := newTestEnv(t)

	admin := env.register(t, "admin", "admin123")
	user := env.register(t, "bob", "bob12345")

	goTag := env.createTag(t, admin.Token, "Go")
	ginTag := env.createTag(t, admin.Token, "Gin")

	post := env.createPost(t, user.Token, PostInput{
		Title: "合并", Body: "#Go #Gin", TagIDs: []uint{goTag.ID},
	})
	if len(post.Tags) != 2 {
		t.Fatalf("手选与自动标签应合并为 2 个，实际 %+v", post.Tags)
	}

	var goStored Tag
	if err := env.db.Take(&goStored, goTag.ID).Error; err != nil {
		t.Fatalf("查询标签失败：%v", err)
	}
	if goStored.UsageCount != 1 {
		t.Fatalf("同名标签不应重复计数，实际 %d", goStored.UsageCount)
	}

	var ginStored Tag
	if err := env.db.Take(&ginStored, ginTag.ID).Error; err != nil {
		t.Fatalf("查询标签失败：%v", err)
	}
	if ginStored.UsageCount != 1 {
		t.Fatalf("Gin 使用次数应为 1，实际 %d", ginStored.UsageCount)
	}
}

func TestMigrateTagReview(t *testing.T) {
	env := newTestEnv(t)

	legacy := Tag{Name: "Legacy"}
	if err := env.db.Create(&legacy).Error; err != nil {
		t.Fatalf("创建历史标签失败：%v", err)
	}

	if err := migrateTagReview(env.db); err != nil {
		t.Fatalf("迁移标签审核状态失败：%v", err)
	}

	var stored Tag
	if err := env.db.Take(&stored, legacy.ID).Error; err != nil {
		t.Fatalf("查询历史标签失败：%v", err)
	}
	if !stored.Reviewed {
		t.Fatal("历史标签应被标记为已审核")
	}

	// 迁移只执行一次：之后新建的待审核标签保持不变。
	fresh := Tag{Name: "Fresh"}
	if err := env.db.Create(&fresh).Error; err != nil {
		t.Fatalf("创建新标签失败：%v", err)
	}
	if err := migrateTagReview(env.db); err != nil {
		t.Fatalf("重复迁移失败：%v", err)
	}
	var freshStored Tag
	if err := env.db.Take(&freshStored, fresh.ID).Error; err != nil {
		t.Fatalf("查询新标签失败：%v", err)
	}
	if freshStored.Reviewed {
		t.Fatal("迁移标记写入后不应再把新标签改为已审核")
	}
}
