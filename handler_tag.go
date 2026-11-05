package main

import (
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

const (
	// 标签名长度上限（按字符数算，中文也算 1 个）
	maxTagNameLength = 30
	// 标签列表一次最多返回多少条
	maxTagLimit = 200
)

var hexColorPattern = regexp.MustCompile(`^#?([0-9a-fA-F]{3}|[0-9a-fA-F]{6})$`)

// 创建标签 POST /api/tag（仅管理员）
func (d *Data) CreateTag(c *gin.Context) {
	var req TagInput
	if err := c.ShouldBindJSON(&req); err != nil {
		Response{}.Fail(c, http.StatusBadRequest, "INVALID_BODY", err.Error())
		return
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		Response{}.Fail(c, http.StatusBadRequest, "NEED_NAME", "标签名不能为空")
		return
	}
	if len([]rune(name)) > maxTagNameLength {
		Response{}.Fail(c, http.StatusBadRequest, "INVALID_NAME", "标签名最多 30 个字符")
		return
	}

	color, ok := normalizeTagColor(req.Color)
	if !ok {
		Response{}.Fail(c, http.StatusBadRequest, "INVALID_COLOR", "颜色需为 #RGB 或 #RRGGBB 格式的十六进制")
		return
	}

	exists, err := d.IfTagExist(name)
	if err != nil {
		Response{}.Fail(c, http.StatusInternalServerError, "QUERY_FAILED", err.Error())
		return
	}
	if exists {
		Response{}.Fail(c, http.StatusConflict, "TAG_EXIST", "标签已存在")
		return
	}

	tag := Tag{Name: name, Color: color}
	if err := d.DB.Create(&tag).Error; err != nil {
		Response{}.Fail(c, http.StatusInternalServerError, "CREATE_FAILED", err.Error())
		return
	}

	Response{}.Ok(c, http.StatusCreated, tag)
}

// GetTags 标签列表 GET /api/tag?q=go&sort=hot&limit=13
//   - q：按名称模糊搜索
//   - sort：name（默认，按名称）/ hot（按使用次数倒序，同次数按名称）
//   - limit：可选，1~200；不传表示全部（前端折叠时传 13 以便判断是否还有更多）
func (d *Data) GetTags(c *gin.Context) {
	keyword := strings.TrimSpace(c.Query("q"))
	sortBy := c.DefaultQuery("sort", "name")
	limit := parseTagLimit(c.Query("limit"))

	db := d.DB.Model(&Tag{})
	if keyword != "" {
		// 转义通配符，避免用户输入的 % / _ 变成通配
		db = db.Where("name LIKE ? ESCAPE '\\'", "%"+escapeLike(keyword)+"%")
	}

	switch sortBy {
	case "hot":
		db = db.Order("usage_count DESC, name ASC")
	default:
		db = db.Order("name ASC")
	}

	if limit > 0 {
		db = db.Limit(limit)
	}

	var tags []Tag
	if err := db.Find(&tags).Error; err != nil {
		Response{}.Fail(c, http.StatusInternalServerError, "QUERY_FAILED", err.Error())
		return
	}

	if tags == nil {
		tags = []Tag{} // 空表返回 [] 而不是 null
	}

	Response{}.Ok(c, http.StatusOK, tags)
}

// normalizeTagColor 把 #RGB / #RRGGBB（# 可省略）归一化为小写 #rrggbb。
// 空字符串表示不指定颜色，由前端使用默认的高级灰。
func normalizeTagColor(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", true
	}
	if !hexColorPattern.MatchString(raw) {
		return "", false
	}

	hex := strings.ToLower(strings.TrimPrefix(raw, "#"))
	if len(hex) == 3 {
		hex = string([]byte{hex[0], hex[0], hex[1], hex[1], hex[2], hex[2]})
	}

	return "#" + hex, true
}

// parseTagLimit 解析 limit，0 表示不限制；非法值宽松回落到不限制。
func parseTagLimit(raw string) int {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0
	}

	limit, err := strconv.Atoi(raw)
	if err != nil || limit <= 0 {
		return 0
	}
	if limit > maxTagLimit {
		return maxTagLimit
	}

	return limit
}

// escapeLike 转义 LIKE 的通配符，配合 ESCAPE '\' 使用
func escapeLike(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "%", "\\%")
	s = strings.ReplaceAll(s, "_", "\\_")
	return s
}

