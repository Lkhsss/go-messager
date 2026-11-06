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
	user, ok := CurrentUser(c)
	if !ok {
		Response{}.Fail(c, http.StatusUnauthorized, "UNAUTHORIZED", "请先登录")
		return
	}

	include, exclude, filterErr := parseTagFilters(c)
	if filterErr != nil {
		Response{}.Fail(c, http.StatusBadRequest, filterErr.Code, filterErr.Message)
		return
	}

	d.listPosts(c, func(db *gorm.DB) *gorm.DB {
		return applyPostVisibility(applyTagFilter(db, include, exclude).Where("root_id IS NULL"), user)
	})
}

// applyPostVisibility 普通用户只看公开文章 + 自己发布的 Private 文章；管理员看全部未删除文章。
func applyPostVisibility(db *gorm.DB, user User) *gorm.DB {
	if user.Role == RoleAdmin {
		return db
	}
	return db.Where("is_private = ? OR user = ?", false, user.Name)
}

// canViewPost 判断当前用户能否查看某篇 Private 文章。
func canViewPost(post Post, user User) bool {
	return !post.IsPrivate || user.Role == RoleAdmin || post.User == user.Name
}

// canViewPostThread 评论额外校验所属原文的可见性，避免绕过原文 Private 状态直接给评论点赞。
func (d *Data) canViewPostThread(post Post, user User) bool {
	if !canViewPost(post, user) {
		return false
	}
	if post.RootID == nil {
		return true
	}

	var root Post
	if err := d.DB.Take(&root, *post.RootID).Error; err != nil {
		return false
	}
	return canViewPost(root, user)
}

// postPrivateInput 管理员切换文章 Private 状态的请求体。
type postPrivateInput struct {
	Private *bool `json:"private"`
}

// SetPostPrivate 管理员隐藏/取消隐藏文章 PATCH /api/post/:id/private
func (d *Data) SetPostPrivate(c *gin.Context) {
	id, ok := parsePostID(c)
	if !ok {
		return
	}

	var req postPrivateInput
	if err := c.ShouldBindJSON(&req); err != nil || req.Private == nil {
		Response{}.Fail(c, http.StatusBadRequest, "INVALID_BODY", "private 必须是布尔值")
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

	if post.RootID != nil {
		Response{}.Fail(c, http.StatusNotFound, "POST_NOT_FOUND", "文章不存在")
		return
	}
	if err := d.DB.Model(&post).Update("is_private", *req.Private).Error; err != nil {
		Response{}.Fail(c, http.StatusInternalServerError, "UPDATE_FAILED", err.Error())
		return
	}
	post.IsPrivate = *req.Private

	result := []Post{post}
	d.fillPostAvatars(result)
	user, _ := CurrentUser(c)
	d.fillPostReactions(result, user.ID)
	Response{}.Ok(c, http.StatusOK, result[0])
}

// GetMyPosts 当前用户自己的文章 GET /api/me/posts?page=1&per_page=20
func (d *Data) GetMyPosts(c *gin.Context) {
	user, ok := CurrentUser(c)
	if !ok {
		Response{}.Fail(c, http.StatusUnauthorized, "UNAUTHORIZED", "请先登录")
		return
	}

	d.listPosts(c, func(db *gorm.DB) *gorm.DB {
		return db.Where("user = ? AND root_id IS NULL", user.Name)
	})
}

// UpdatePost 编辑自己的文章 PUT /api/post/:id
// 只能修改标题、副标题、正文和标签；作者与反应统计不受影响。
func (d *Data) UpdatePost(c *gin.Context) {
	user, ok := CurrentUser(c)
	if !ok {
		Response{}.Fail(c, http.StatusUnauthorized, "UNAUTHORIZED", "请先登录")
		return
	}

	id, ok := parsePostID(c)
	if !ok {
		return
	}

	var req PostInput
	if err := c.ShouldBindJSON(&req); err != nil {
		Response{}.Fail(c, http.StatusBadRequest, "INVALID_BODY", err.Error())
		return
	}

	tagIDs := uniqueIDs(req.TagIDs)
	if len(tagIDs) > maxTagsPerPost {
		Response{}.Fail(c, http.StatusBadRequest, "TOO_MANY_TAGS", "每篇文章最多选择 5 个标签")
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
	if post.RootID != nil {
		Response{}.Fail(c, http.StatusNotFound, "POST_NOT_FOUND", "文章不存在")
		return
	}
	if post.User != user.Name {
		Response{}.Fail(c, http.StatusForbidden, "FORBIDDEN", "只能编辑自己的文章")
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

	// 文章字段与多对多关联放在同一事务里；标签使用次数全量校准，避免增量维护出错
	err := d.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&post).Updates(map[string]any{
			"title":    req.Title,
			"subtitle": req.Subtitle,
			"body":     req.Body,
		}).Error; err != nil {
			return err
		}
		if err := tx.Model(&post).Association("Tags").Replace(tags); err != nil {
			return err
		}
		return syncTagUsageCounts(tx)
	})
	if err != nil {
		Response{}.Fail(c, http.StatusInternalServerError, "UPDATE_FAILED", err.Error())
		return
	}

	post.Title = req.Title
	post.Subtitle = req.Subtitle
	post.Body = req.Body
	post.Tags = tags

	result := []Post{post}
	d.fillPostAvatars(result)
	d.fillPostReactions(result, user.ID)
	post = result[0]

	Response{}.Ok(c, http.StatusOK, post)
}

// DeletePost 删除文章 DELETE /api/post/:id（作者或管理员）
// 使用软删除，文章不再出现在列表/详情；同时校准标签使用次数。
func (d *Data) DeletePost(c *gin.Context) {
	user, ok := CurrentUser(c)
	if !ok {
		Response{}.Fail(c, http.StatusUnauthorized, "UNAUTHORIZED", "请先登录")
		return
	}

	id, ok := parsePostID(c)
	if !ok {
		return
	}

	var post Post
	if err := d.DB.Take(&post, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			Response{}.Fail(c, http.StatusNotFound, "POST_NOT_FOUND", "文章不存在")
			return
		}
		Response{}.Fail(c, http.StatusInternalServerError, "QUERY_FAILED", err.Error())
		return
	}
	if post.User != user.Name && user.Role != RoleAdmin {
		Response{}.Fail(c, http.StatusForbidden, "FORBIDDEN", "只能删除自己的文章")
		return
	}

	if err := d.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Delete(&post).Error; err != nil {
			return err
		}
		return syncTagUsageCounts(tx)
	}); err != nil {
		Response{}.Fail(c, http.StatusInternalServerError, "DELETE_FAILED", err.Error())
		return
	}

	Response{}.Ok(c, http.StatusOK, gin.H{"id": id})
}

const maxCommentLength = 10000

// GetComments 获取某篇文章的评论（扁平列表，前端按 parent_id 组树）GET /api/post/:id/comments
func (d *Data) GetComments(c *gin.Context) {
	rootID, ok := parsePostID(c)
	if !ok {
		return
	}

	user, ok := CurrentUser(c)
	if !ok {
		Response{}.Fail(c, http.StatusUnauthorized, "UNAUTHORIZED", "请先登录")
		return
	}

	var root Post
	if err := d.DB.Take(&root, rootID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			Response{}.Fail(c, http.StatusNotFound, "POST_NOT_FOUND", "文章不存在")
			return
		}
		Response{}.Fail(c, http.StatusInternalServerError, "QUERY_FAILED", err.Error())
		return
	}
	if root.RootID != nil || !canViewPost(root, user) {
		Response{}.Fail(c, http.StatusNotFound, "POST_NOT_FOUND", "文章不存在")
		return
	}

	var comments []Post
	if err := d.DB.Where("root_id = ?", rootID).
		Order("created_at ASC, id ASC").
		Find(&comments).Error; err != nil {
		Response{}.Fail(c, http.StatusInternalServerError, "QUERY_FAILED", err.Error())
		return
	}
	if comments == nil {
		comments = []Post{}
	}

	d.fillPostAvatars(comments)
	d.fillPostReactions(comments, user.ID)
	Response{}.Ok(c, http.StatusOK, comments)
}

// CreateComment 发表评论 POST /api/post/:id/comments
// 省略 parent_id 是顶层评论；带 parent_id 时必须是同一原文下的评论。
func (d *Data) CreateComment(c *gin.Context) {
	rootID, ok := parsePostID(c)
	if !ok {
		return
	}

	user, ok := CurrentUser(c)
	if !ok {
		Response{}.Fail(c, http.StatusUnauthorized, "UNAUTHORIZED", "请先登录")
		return
	}

	var req CommentInput
	if err := c.ShouldBindJSON(&req); err != nil {
		Response{}.Fail(c, http.StatusBadRequest, "INVALID_BODY", err.Error())
		return
	}
	body := strings.TrimSpace(req.Body)
	if body == "" {
		Response{}.Fail(c, http.StatusBadRequest, "INVALID_BODY", "评论内容不能为空")
		return
	}
	if len([]rune(body)) > maxCommentLength {
		Response{}.Fail(c, http.StatusBadRequest, "INVALID_BODY", "评论内容最多 10000 个字符")
		return
	}

	var root Post
	if err := d.DB.Take(&root, rootID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			Response{}.Fail(c, http.StatusNotFound, "POST_NOT_FOUND", "文章不存在")
			return
		}
		Response{}.Fail(c, http.StatusInternalServerError, "QUERY_FAILED", err.Error())
		return
	}
	if root.RootID != nil || !canViewPost(root, user) {
		Response{}.Fail(c, http.StatusNotFound, "POST_NOT_FOUND", "文章不存在")
		return
	}

	var parentID *uint
	if req.ParentID != nil && *req.ParentID > 0 {
		var parent Post
		if err := d.DB.Take(&parent, *req.ParentID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				Response{}.Fail(c, http.StatusNotFound, "COMMENT_NOT_FOUND", "父评论不存在")
				return
			}
			Response{}.Fail(c, http.StatusInternalServerError, "QUERY_FAILED", err.Error())
			return
		}
		if parent.RootID == nil || *parent.RootID != root.ID {
			Response{}.Fail(c, http.StatusBadRequest, "INVALID_PARENT", "父评论不属于当前文章")
			return
		}
		parentID = &parent.ID
	}

	comment := Post{
		User:     user.Name,
		Body:     body,
		RootID:   &root.ID,
		ParentID: parentID,
	}
	if err := d.DB.Create(&comment).Error; err != nil {
		Response{}.Fail(c, http.StatusInternalServerError, "CREATE_FAILED", err.Error())
		return
	}

	result := []Post{comment}
	d.fillPostAvatars(result)
	d.fillPostReactions(result, user.ID)
	Response{}.Ok(c, http.StatusCreated, result[0])
}

// parsePostID 解析路径里的文章 ID，非法时直接写 400 响应。
func parsePostID(c *gin.Context) (uint, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		Response{}.Fail(c, http.StatusBadRequest, "INVALID_ID", "文章 ID 无效")
		return 0, false
	}

	return uint(id), true
}

// 文章详情 GET /api/post/:id（含标签）
func (d *Data) GetPostByID(c *gin.Context) {
	id, ok := parsePostID(c)
	if !ok {
		return
	}

	user, ok := CurrentUser(c)
	if !ok {
		Response{}.Fail(c, http.StatusUnauthorized, "UNAUTHORIZED", "请先登录")
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

	if post.RootID != nil {
		Response{}.Fail(c, http.StatusNotFound, "POST_NOT_FOUND", "文章不存在")
		return
	}
	if !canViewPost(post, user) {
		Response{}.Fail(c, http.StatusNotFound, "POST_NOT_FOUND", "文章不存在")
		return
	}

	if post.Tags == nil {
		post.Tags = []Tag{}
	}

	result := []Post{post}
	d.fillPostAvatars(result)
	d.fillPostReactions(result, user.ID)
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
