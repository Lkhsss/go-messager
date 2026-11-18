package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// 正文与标题长度上限。
const (
	maxPostBodyRunes     = 100000
	maxPostTitleRunes    = 120
	maxPostSubtitleRunes = 200
	excerptLength        = 300
)

var (
	markdownFencePattern = regexp.MustCompile("(?s)```.*?```")
	markdownImagePattern = regexp.MustCompile(`!\[[^\]]*\]\([^)]*\)`)
	markdownLinkPattern  = regexp.MustCompile(`\[([^\]]*)\]\([^)]*\)`)
)

func validatePostLengths(title, subtitle, body string) (string, string) {
	if len([]rune(title)) > maxPostTitleRunes {
		return "TITLE_TOO_LONG", "标题最多 120 个字符"
	}
	if len([]rune(subtitle)) > maxPostSubtitleRunes {
		return "SUBTITLE_TOO_LONG", "副标题最多 200 个字符"
	}
	if len([]rune(body)) > maxPostBodyRunes {
		return "BODY_TOO_LONG", "正文最多 100000 个字符"
	}
	return "", ""
}

// writeKind 区分文章与评论的写入配额。
type writeKind int

const (
	writeArticle writeKind = iota
	writeComment
)

// postBodyHash 忽略空白后计算 SHA-256，用于拦截同一用户的重复内容。
func postBodyHash(body string) string {
	normalized := strings.Join(strings.Fields(body), "")
	sum := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(sum[:])
}

// startOfDay 返回服务器本地时区的当天零点。
func startOfDay(now time.Time) time.Time {
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
}

// duplicateBodyExists 检查同一用户在窗口内是否已发布相同正文。
func (d *Data) duplicateBodyExists(user, hash string, comment bool, since time.Time, excludeID uint) (bool, error) {
	query := d.DB.Model(&Post{}).
		Where("user = ? AND body_hash = ? AND created_at >= ?", user, hash, since)
	if comment {
		query = query.Where("root_id IS NOT NULL")
	} else {
		query = query.Where("root_id IS NULL")
	}
	if excludeID > 0 {
		query = query.Where("id <> ?", excludeID)
	}

	var count int64
	if err := query.Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// allowWrite 检查分钟级频率和当天配额。返回非 0 状态码表示拒绝。
func (d *Data) allowWrite(user User, kind writeKind) (int, string, string) {
	var (
		limiter   *rateLimiter
		perMinute int
		perDay    int
		newPerDay int
		rateCode  string
		rateMsg   string
	)
	switch kind {
	case writeArticle:
		limiter, perMinute, perDay, newPerDay = d.postLimiter, d.limits.PostPerMinute, d.limits.PostPerDay, d.limits.NewAccountPostPerDay
		rateCode, rateMsg = "POST_RATE_LIMITED", "发文过于频繁，请稍后再试"
	case writeComment:
		limiter, perMinute, perDay, newPerDay = d.commentLimiter, d.limits.CommentPerMinute, d.limits.CommentPerDay, d.limits.NewAccountCommentPerDay
		rateCode, rateMsg = "COMMENT_RATE_LIMITED", "评论过于频繁，请稍后再试"
	default:
		return 0, "", ""
	}

	if limiter == nil || !limiter.Allow(user.Name, perMinute) {
		return http.StatusTooManyRequests, rateCode, rateMsg
	}

	limit := perDay
	if !user.Trusted && user.Role != RoleAdmin && d.limits.NewAccountHours > 0 &&
		time.Since(user.CreatedAt) < d.limits.NewAccountHours {
		limit = newPerDay
	}
	if limit <= 0 {
		return 0, "", ""
	}

	query := d.DB.Unscoped().Model(&Post{}).
		Where("user = ? AND created_at >= ?", user.Name, startOfDay(time.Now()))
	if kind == writeArticle {
		query = query.Where("root_id IS NULL")
	} else {
		query = query.Where("root_id IS NOT NULL")
	}

	var count int64
	if err := query.Count(&count).Error; err != nil {
		return http.StatusInternalServerError, "QUERY_FAILED", err.Error()
	}
	if count < int64(limit) {
		return 0, "", ""
	}

	if kind == writeArticle {
		return http.StatusTooManyRequests, "POST_QUOTA_EXCEEDED", fmt.Sprintf("今日最多发布 %d 篇文章", limit)
	}
	return http.StatusTooManyRequests, "COMMENT_QUOTA_EXCEEDED", fmt.Sprintf("今日最多发表 %d 条评论", limit)
}

// buildExcerpt 从 Markdown 生成纯文本摘要。
func buildExcerpt(body string) string {
	text := markdownFencePattern.ReplaceAllString(body, " ")
	text = markdownImagePattern.ReplaceAllString(text, " ")
	text = markdownLinkPattern.ReplaceAllString(text, "$1")
	text = strings.NewReplacer("`", "", "*", "", "_", "", "~", "").Replace(text)

	lines := strings.Split(text, "\n")
	cleaned := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(strings.TrimLeft(line, "#>-+ \t"))
		if line != "" {
			cleaned = append(cleaned, line)
		}
	}
	text = strings.Join(strings.Fields(strings.Join(cleaned, " ")), " ")

	runes := []rune(text)
	if len(runes) > excerptLength {
		text = string(runes[:excerptLength]) + "…"
	}
	return text
}

// NewPost 创建文章。
func (d *Data) NewPost(c *gin.Context) {
	user, ok := CurrentUser(c)
	if !ok {
		// 路由已挂 AuthRequired，这里兜底。
		Response{}.Fail(c, http.StatusUnauthorized, "UNAUTHORIZED", "请先登录")
		return
	}

	var req PostInput
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBindError(c, err, "INVALID_BODY")
		return
	}

	if code, message := validatePostLengths(req.Title, req.Subtitle, req.Body); code != "" {
		Response{}.Fail(c, http.StatusBadRequest, code, message)
		return
	}

	// 先做内存限流和当天配额，再做内容去重；两道检查都能在写库前拦截灌水。
	if status, code, message := d.allowWrite(user, writeArticle); status != 0 {
		Response{}.Fail(c, status, code, message)
		return
	}
	bodyHash := postBodyHash(req.Body)
	if d.limits.DuplicateWindow > 0 {
		duplicated, err := d.duplicateBodyExists(user.Name, bodyHash, false, time.Now().Add(-d.limits.DuplicateWindow), 0)
		if err != nil {
			Response{}.Fail(c, http.StatusInternalServerError, "QUERY_FAILED", err.Error())
			return
		}
		if duplicated {
			Response{}.Fail(c, http.StatusConflict, "DUPLICATE_POST", "请勿重复发布相同内容")
			return
		}
	}

	// 标签去重；手选数量不设上限。
	tagIDs := uniqueIDs(req.TagIDs)

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
		User:      user.Name, // 作者由服务端决定，忽略客户端传来的 user
		Title:     req.Title,
		Subtitle:  req.Subtitle,
		Body:      req.Body,
		Excerpt:   buildExcerpt(req.Body),
		CharCount: countTextCharacters(req.Title, req.Subtitle, req.Body),
		BodyHash:  bodyHash,
		Tags:      tags,
	}

	// 正文中的 #标签 自动关联；新标签先进入待审核状态。
	bodyTagNames := extractBodyTags(req.Body)

	// 文章和标签计数放在同一事务。
	err := d.DB.Transaction(func(tx *gorm.DB) error {
		autoTags, err := d.resolveBodyTags(tx, bodyTagNames)
		if err != nil {
			return err
		}
		post.Tags = mergeTags(tags, autoTags)

		if err := tx.Create(&post).Error; err != nil {
			return err
		}

		ids := tagIDsOf(post.Tags)
		if len(ids) == 0 {
			return nil
		}

		return tx.Model(&Tag{}).
			Where("id IN ?", ids).
			UpdateColumn("usage_count", gorm.Expr("usage_count + 1")).Error
	})
	if err != nil {
		Response{}.Fail(c, http.StatusInternalServerError, "CREATE_FAILED", err.Error())
		return
	}

	// 未审核标签不出现在文章下方标签区。
	post.Tags = filterReviewedTags(post.Tags)

	post.UserAvatarVersion = avatarVersion(user.Avatar)
	post.UserLevel, _, _ = levelFromExp(user.Exp)
	post.Reactions = buildReactionSummary(0, 0, 0, "")
	post.MyReactions = []ReactionType{}

	// 达到字数要求时发放每日写文章经验，失败不影响发布。
	if post.CharCount >= DailyArticleMinChars {
		if gain, xpErr := d.awardDailyTask(user.ID, DailyTaskArticle, DailyArticleXP, time.Now()); xpErr == nil {
			post.XP = gain
		}
	}

	Response{}.Ok(c, http.StatusCreated, post)
}

// GetPost 文章列表，支持标签过滤。
func (d *Data) GetPost(c *gin.Context) {
	// 隐私模式由 ReadAuth 保证已登录；公开模式匿名为零值 User，只返回公开文章。
	user, _ := CurrentUser(c)

	include, exclude, filterErr := parseTagFilters(c)
	if filterErr != nil {
		Response{}.Fail(c, http.StatusBadRequest, filterErr.Code, filterErr.Message)
		return
	}

	d.listPosts(c, func(db *gorm.DB) *gorm.DB {
		return applyPostVisibility(applyTagFilter(db, include, exclude).Where("root_id IS NULL"), user)
	})
}

// applyPostVisibility 普通用户只看公开文章和自己的私有文章。
func applyPostVisibility(db *gorm.DB, user User) *gorm.DB {
	if user.Role == RoleAdmin {
		return db
	}
	return db.Where("is_private = ? OR user = ?", false, user.Name)
}

// canViewPost 判断能否查看私有文章。
func canViewPost(post Post, user User) bool {
	return !post.IsPrivate || user.Role == RoleAdmin || post.User == user.Name
}

// canViewPostThread 评论还要校验原文可见性。
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

type postPrivateInput struct {
	Private *bool `json:"private"`
}

// SetPostPrivate 隐藏/取消隐藏文章。
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
	if err := d.DB.Preload("Tags", "reviewed = ?", true).Take(&post, id).Error; err != nil {
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

// GetMyPosts 当前用户的文章。
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

// UpdatePost 编辑自己的文章。
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
		respondBindError(c, err, "INVALID_BODY")
		return
	}

	if code, message := validatePostLengths(req.Title, req.Subtitle, req.Body); code != "" {
		Response{}.Fail(c, http.StatusBadRequest, code, message)
		return
	}

	bodyHash := postBodyHash(req.Body)
	if d.limits.DuplicateWindow > 0 {
		duplicated, err := d.duplicateBodyExists(user.Name, bodyHash, false, time.Now().Add(-d.limits.DuplicateWindow), id)
		if err != nil {
			Response{}.Fail(c, http.StatusInternalServerError, "QUERY_FAILED", err.Error())
			return
		}
		if duplicated {
			Response{}.Fail(c, http.StatusConflict, "DUPLICATE_POST", "请勿重复发布相同内容")
			return
		}
	}

	tagIDs := uniqueIDs(req.TagIDs)

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

	excerpt := buildExcerpt(req.Body)
	charCount := countTextCharacters(req.Title, req.Subtitle, req.Body)

	// 正文中的 #标签 自动关联；新标签先进入待审核状态。
	bodyTagNames := extractBodyTags(req.Body)
	mergedTags := tags

	// 更新字段和标签，校准使用次数。
	err := d.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&post).Updates(map[string]any{
			"title":      req.Title,
			"subtitle":   req.Subtitle,
			"body":       req.Body,
			"excerpt":    excerpt,
			"char_count": charCount,
			"body_hash":  bodyHash,
		}).Error; err != nil {
			return err
		}
		autoTags, err := d.resolveBodyTags(tx, bodyTagNames)
		if err != nil {
			return err
		}
		mergedTags = mergeTags(tags, autoTags)
		if err := tx.Model(&post).Association("Tags").Replace(mergedTags); err != nil {
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
	post.Excerpt = excerpt
	post.CharCount = charCount
	post.BodyHash = bodyHash
	// 未审核标签不出现在文章下方标签区。
	post.Tags = filterReviewedTags(mergedTags)

	result := []Post{post}
	d.fillPostAvatars(result)
	d.fillPostReactions(result, user.ID)
	post = result[0]

	Response{}.Ok(c, http.StatusOK, post)
}

// DeletePost 软删除文章并校准标签计数。
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
		message := "只能删除自己的文章"
		if post.RootID != nil {
			message = "只能删除自己的评论"
		}
		Response{}.Fail(c, http.StatusForbidden, "FORBIDDEN", message)
		return
	}

	if err := d.DB.Transaction(func(tx *gorm.DB) error {
		ids := []uint{post.ID}
		if post.RootID != nil {
			var err error
			ids, err = collectCommentDescendants(tx, post.ID)
			if err != nil {
				return err
			}
		}
		if err := tx.Where("id IN ?", ids).Delete(&Post{}).Error; err != nil {
			return err
		}
		return syncTagUsageCounts(tx)
	}); err != nil {
		Response{}.Fail(c, http.StatusInternalServerError, "DELETE_FAILED", err.Error())
		return
	}
	Response{}.Ok(c, http.StatusOK, gin.H{"id": id})
}

// collectCommentDescendants 收集评论及所有回复。
func collectCommentDescendants(tx *gorm.DB, rootID uint) ([]uint, error) {
	ids := []uint{rootID}
	frontier := []uint{rootID}
	seen := map[uint]struct{}{rootID: {}}

	for len(frontier) > 0 {
		var children []uint
		if err := tx.Model(&Post{}).Where("parent_id IN ?", frontier).Pluck("id", &children).Error; err != nil {
			return nil, err
		}
		next := frontier[:0]
		for _, id := range children {
			if _, ok := seen[id]; ok {
				continue
			}
			seen[id] = struct{}{}
			ids = append(ids, id)
			next = append(next, id)
		}
		frontier = next
	}

	return ids, nil
}

const (
	maxCommentLength       = 10000
	commentDuplicateWindow = 10 * time.Minute
)

// GetComments 文章评论（扁平列表）。
func (d *Data) GetComments(c *gin.Context) {
	rootID, ok := parsePostID(c)
	if !ok {
		return
	}

	// 隐私模式由 ReadAuth 保证已登录；公开模式匿名为零值 User。
	user, _ := CurrentUser(c)

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
		Order("id ASC").
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

// CreateComment 发表评论或回复。
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
		respondBindError(c, err, "INVALID_BODY")
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

	if status, code, message := d.allowWrite(user, writeComment); status != 0 {
		Response{}.Fail(c, status, code, message)
		return
	}
	bodyHash := postBodyHash(body)
	if d.limits.DuplicateWindow > 0 {
		duplicated, err := d.duplicateBodyExists(user.Name, bodyHash, true, time.Now().Add(-commentDuplicateWindow), 0)
		if err != nil {
			Response{}.Fail(c, http.StatusInternalServerError, "QUERY_FAILED", err.Error())
			return
		}
		if duplicated {
			Response{}.Fail(c, http.StatusConflict, "DUPLICATE_COMMENT", "请勿重复发布相同评论")
			return
		}
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
		BodyHash: bodyHash,
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

	// 达到字数要求时发放每日评论经验，失败不影响评论。
	if countTextCharacters(body) >= DailyCommentMinChars {
		if gain, xpErr := d.awardDailyTask(user.ID, DailyTaskComment, DailyCommentXP, time.Now()); xpErr == nil {
			result[0].XP = gain
		}
	}

	Response{}.Ok(c, http.StatusCreated, result[0])
}

// parsePostID 解析文章 ID。
func parsePostID(c *gin.Context) (uint, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		Response{}.Fail(c, http.StatusBadRequest, "INVALID_ID", "文章 ID 无效")
		return 0, false
	}

	return uint(id), true
}

// GetPostByID 文章详情。
func (d *Data) GetPostByID(c *gin.Context) {
	id, ok := parsePostID(c)
	if !ok {
		return
	}

	// 隐私模式由 ReadAuth 保证已登录；公开模式匿名为零值 User。
	user, _ := CurrentUser(c)

	var post Post
	if err := d.DB.Preload("Tags", "reviewed = ?", true).Take(&post, id).Error; err != nil {
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

// listPosts 分页查询文章。
func (d *Data) listPosts(c *gin.Context, filter func(*gorm.DB) *gorm.DB) {
	page, perPage := parsePagination(c)

	base := func() *gorm.DB {
		db := d.DB.Model(&Post{})
		if filter != nil {
			db = filter(db)
		}
		return db
	}

	// 总数用于计算页数。
	var total int64
	if err := base().Count(&total).Error; err != nil {
		Response{}.Fail(c, http.StatusInternalServerError, "COUNT_FAILED", err.Error())
		return
	}

	// 新文章在前。id 是自增主键，反向扫描即可，避免对大表做临时排序。
	var posts []Post
	if err := base().
		Select("id", "created_at", "updated_at", "deleted_at", "user", "title", "subtitle", "excerpt", "is_private", "parent_id", "root_id", "like_count", "dislike_count", "question_count", "emoji_counts", "char_count").
		Preload("Tags", "reviewed = ?", true).
		Order("id DESC").
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

// 每侧最多过滤标签数。
const maxFilterTags = 20

// parseTagFilters 解析包含/排除标签。
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

// parseIDList 解析逗号分隔的 ID。
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

// applyTagFilter 追加标签过滤条件。
func applyTagFilter(db *gorm.DB, include, exclude []uint) *gorm.DB {
	for _, id := range include {
		db = db.Where("posts.id IN (SELECT post_id FROM post_tags WHERE tag_id = ?)", id)
	}

	if len(exclude) > 0 {
		db = db.Where("posts.id NOT IN (SELECT post_id FROM post_tags WHERE tag_id IN ?)", exclude)
	}

	return db
}

// uniqueIDs 去重并忽略 0。
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

// ensurePostTags 只保留已审核标签，并保证序列化为 []。
func ensurePostTags(posts []Post) {
	for i := range posts {
		posts[i].Tags = filterReviewedTags(posts[i].Tags)
	}
}

// 分页默认值和上限。
const (
	defaultPerPage = 20
	maxPerPage     = 100
)

// parsePagination 解析 page/per_page。
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
