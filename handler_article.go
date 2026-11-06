package main

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// 每篇文章最多可选择的标签数
const maxTagsPerPost = 5

// 创建文章 POST /api/post/new（需登录，作者取自 token，标签最多 5 个）
func (d *Data) NewPost(c *gin.Context) {
	user, ok := CurrentUser(c)
	if !ok {
		// 正常走不到这里（路由上已挂 AuthRequired），兜底避免拿不到用户
		Response{}.Fail(c, http.StatusUnauthorized, "UNAUTHORIZED", "请先登录")
		return
	}

	var req PostInput
	if err := c.ShouldBindJSON(&req); err != nil {
		Response{}.Fail(c, http.StatusBadRequest, "INVALID_BODY", err.Error())
		return
	}

	// 去重后再判断数量，防止用重复 ID 绕过上限
	tagIDs := uniqueIDs(req.TagIDs)
	if len(tagIDs) > maxTagsPerPost {
		Response{}.Fail(c, http.StatusBadRequest, "TOO_MANY_TAGS", "每篇文章最多选择 5 个标签")
		return
	}

	tags := []Tag{}
	if len(tagIDs) > 0 {
		if err := d.DB.Where("id IN ?", tagIDs).Find(&tags).Error; err != nil {
			Response{}.Fail(c, http.StatusInternalServerError, "QUERY_FAILED", err.Error())
			return
		}
		if len(tags) != len(tagIDs) {
			Response{}.Fail(c, http.StatusBadRequest, "INVALID_TAGS", "包含不存在的标签")
			return
		}
	}

	post := Post{
		User:     user.Name, // 作者由服务端决定，忽略客户端传来的 user
		Title:    req.Title,
		Subtitle: req.Subtitle,
		Body:     req.Body,
		Tags:     tags,
	}

	// 创建文章与累加标签使用次数放在同一个事务里，避免计数和关联表不一致
	err := d.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&post).Error; err != nil {
			return err
		}
		if len(tagIDs) == 0 {
			return nil
		}

		return tx.Model(&Tag{}).
			Where("id IN ?", tagIDs).
			UpdateColumn("usage_count", gorm.Expr("usage_count + 1")).Error
	})
	if err != nil {
		Response{}.Fail(c, http.StatusInternalServerError, "CREATE_FAILED", err.Error())
		return
	}

	post.UserAvatarVersion = avatarVersion(user.Avatar)
	post.Reactions = buildReactionSummary(0, 0, 0, "")
	post.MyReactions = []ReactionType{}

	Response{}.Ok(c, http.StatusCreated, post)
}

// 文章列表 GET /api/post?page=1&per_page=20
//
//	&include_tag_ids=1,2   必须同时包含所有列出的标签（AND）
//	&exclude_tag_ids=3,4   不能包含其中任意一个（NOT）
func (d *Data) GetPost(c *gin.Context) {
	include, exclude, filterErr := parseTagFilters(c)
	if filterErr != nil {
		Response{}.Fail(c, http.StatusBadRequest, filterErr.Code, filterErr.Message)
		return
	}

	d.listPosts(c, func(db *gorm.DB) *gorm.DB {
		return applyTagFilter(db, include, exclude)
	})
}

// 文章详情 GET /api/post/:id（含标签）
func (d *Data) GetPostByID(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		Response{}.Fail(c, http.StatusBadRequest, "INVALID_ID", "文章 ID 无效")
		return
	}

	var post Post
	if err := d.DB.Preload("Tags").Take(&post, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			Response{}.Fail(c, http.StatusNotFound, "POST_NOT_FOUND", "文章不存在")
			return
		}
		Response{}.Fail(c, http.StatusInternalServerError, "QUERY_FAILED", err.Error())
		return
	}

	if post.Tags == nil {
		post.Tags = []Tag{}
	}

	result := []Post{post}
	d.fillPostAvatars(result)
	currentUser, _ := CurrentUser(c)
	d.fillPostReactions(result, currentUser.ID)
	post = result[0]

	Response{}.Ok(c, http.StatusOK, post)
}

// listPosts 分页查询文章并预加载标签，filter 用于追加过滤条件（如按标签过滤）
func (d *Data) listPosts(c *gin.Context, filter func(*gorm.DB) *gorm.DB) {
	page, perPage := parsePagination(c)

	base := func() *gorm.DB {
		db := d.DB.Model(&Post{})
		if filter != nil {
			db = filter(db)
		}
		return db
	}

	// 总数用于计算总页数，不需要 order/limit/offset
	var total int64
	if err := base().Count(&total).Error; err != nil {
		Response{}.Fail(c, http.StatusInternalServerError, "COUNT_FAILED", err.Error())
		return
	}

	// 新文章在前：created_at DESC，同一时间戳用 id DESC 兜底，保证翻页顺序稳定
	var posts []Post
	if err := base().
		Preload("Tags").
		Order("created_at DESC, id DESC").
		Limit(perPage).
		Offset((page - 1) * perPage).
		Find(&posts).Error; err != nil {
		Response{}.Fail(c, http.StatusInternalServerError, "QUERY_FAILED", err.Error())
		return
	}

	ensurePostTags(posts)
	d.fillPostAvatars(posts)

	currentUser, _ := CurrentUser(c)
	d.fillPostReactions(posts, currentUser.ID)

	totalPages := 0
	if total > 0 {
		totalPages = int((total + int64(perPage) - 1) / int64(perPage)) // 向上取整
	}

	Response{}.OkWithMeta(c, http.StatusOK, posts, &Meta{
		Page:       page,
		PerPage:    perPage,
		Total:      int(total),
		TotalPages: totalPages,
	})
}

// 每侧（包含 / 排除）最多允许的标签数
const maxFilterTags = 20

// parseTagFilters 解析 include_tag_ids / exclude_tag_ids，返回去重后的列表。
// 同一标签同时出现在两边时排除优先，不报错。
func parseTagFilters(c *gin.Context) (include, exclude []uint, filterErr *ErrorInfo) {
	include, ok := parseIDList(c.Query("include_tag_ids"))
	if !ok {
		return nil, nil, &ErrorInfo{Code: "INVALID_TAG_FILTER", Message: "include_tag_ids 格式无效"}
	}

	exclude, ok = parseIDList(c.Query("exclude_tag_ids"))
	if !ok {
		return nil, nil, &ErrorInfo{Code: "INVALID_TAG_FILTER", Message: "exclude_tag_ids 格式无效"}
	}

	if len(include) > maxFilterTags || len(exclude) > maxFilterTags {
		return nil, nil, &ErrorInfo{Code: "TOO_MANY_FILTERS", Message: "每侧最多选择 20 个标签"}
	}

	excludeSet := make(map[uint]struct{}, len(exclude))
	for _, id := range exclude {
		excludeSet[id] = struct{}{}
	}

	kept := include[:0]
	for _, id := range include {
		if _, isExcluded := excludeSet[id]; !isExcluded {
			kept = append(kept, id)
		}
	}

	return kept, exclude, nil
}

// parseIDList 解析 "1,2,3" 形式的标签 ID 列表：
// 容忍空格与尾随逗号，去重，0 或非数字视为非法。
func parseIDList(raw string) ([]uint, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, true
	}

	parts := strings.Split(raw, ",")
	seen := make(map[uint]struct{}, len(parts))
	ids := make([]uint, 0, len(parts))

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		id, err := strconv.ParseUint(part, 10, 64)
		if err != nil || id == 0 {
			return nil, false
		}

		value := uint(id)
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		ids = append(ids, value)
	}

	return ids, true
}

// applyTagFilter 给文章查询追加标签条件。
// include 用多个 IN 子查询实现 AND；exclude 用一个 NOT IN 子查询实现 NOT。
func applyTagFilter(db *gorm.DB, include, exclude []uint) *gorm.DB {
	for _, id := range include {
		db = db.Where("posts.id IN (SELECT post_id FROM post_tags WHERE tag_id = ?)", id)
	}

	if len(exclude) > 0 {
		db = db.Where("posts.id NOT IN (SELECT post_id FROM post_tags WHERE tag_id IN ?)", exclude)
	}

	return db
}

// uniqueIDs 去掉重复与 0 值，保持原有顺序
func uniqueIDs(ids []uint) []uint {
	seen := make(map[uint]struct{}, len(ids))
	result := make([]uint, 0, len(ids))

	for _, id := range ids {
		if id == 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}

	return result
}

// ensurePostTags 保证 tags 序列化成 [] 而不是 null，方便前端处理
func ensurePostTags(posts []Post) {
	for i := range posts {
		if posts[i].Tags == nil {
			posts[i].Tags = []Tag{}
		}
	}
}

// 分页默认值与上限
const (
	defaultPerPage = 20
	maxPerPage     = 100
)

// parsePagination 解析 page / per_page 查询参数，非法或缺失时回落到默认值。
// page 最小为 1，per_page 范围为 1~100，默认 20。
func parsePagination(c *gin.Context) (page, perPage int) {
	page, err := strconv.Atoi(c.DefaultQuery("page", "1"))
	if err != nil || page < 1 {
		page = 1
	}

	perPage, err = strconv.Atoi(c.DefaultQuery("per_page", strconv.Itoa(defaultPerPage)))
	if err != nil || perPage < 1 {
		perPage = defaultPerPage
	}
	if perPage > maxPerPage {
		perPage = maxPerPage
	}

	return page, perPage
}
