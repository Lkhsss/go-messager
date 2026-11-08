package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// 主反应列。
var primaryReactionColumns = map[ReactionType]string{
	ReactionLike:     "like_count",
	ReactionDislike:  "dislike_count",
	ReactionQuestion: "question_count",
}

// 允许的反应类型。
var allowedReactionTypes = map[ReactionType]struct{}{
	ReactionLike:     {},
	ReactionDislike:  {},
	ReactionQuestion: {},
	ReactionLaugh:    {},
	ReactionHooray:   {},
	ReactionHeart:    {},
	ReactionRocket:   {},
	ReactionEyes:     {},
}

func isValidReactionType(t ReactionType) bool {
	_, ok := allowedReactionTypes[t]
	return ok
}

// buildReactionSummary 组装反应汇总。
func buildReactionSummary(like, dislike, question int64, emojiJSON string) ReactionSummary {
	return ReactionSummary{
		Like:     like,
		Dislike:  dislike,
		Question: question,
		Emoji:    parseEmojiCounts(emojiJSON),
	}
}

func parseEmojiCounts(raw string) map[string]int64 {
	counts := map[string]int64{}
	if strings.TrimSpace(raw) == "" {
		return counts
	}
	if err := json.Unmarshal([]byte(raw), &counts); err != nil {
		return map[string]int64{}
	}
	return counts
}

// fillPostReactions 批量填充反应汇总和我的反应。
func (d *Data) fillPostReactions(posts []Post, userID uint) {
	if len(posts) == 0 {
		return
	}

	ids := make([]uint, 0, len(posts))
	for i := range posts {
		posts[i].Reactions = buildReactionSummary(
			posts[i].LikeCount, posts[i].DislikeCount, posts[i].QuestionCount, posts[i].EmojiCounts,
		)
		posts[i].MyReactions = []ReactionType{}
		ids = append(ids, posts[i].ID)
	}

	if userID == 0 {
		return
	}

	var rows []Reaction
	if err := d.DB.Select("post_id", "type").
		Where("user_id = ? AND post_id IN ?", userID, ids).
		Find(&rows).Error; err != nil {
		return // 我的反应只是展示信息，查询失败不影响文章列表
	}

	mine := make(map[uint][]ReactionType, len(posts))
	for _, row := range rows {
		mine[row.PostID] = append(mine[row.PostID], row.Type)
	}
	for i := range posts {
		if types, ok := mine[posts[i].ID]; ok {
			posts[i].MyReactions = types
		}
	}
}

func (d *Data) myReactionTypes(postID, userID uint) []ReactionType {
	result := []ReactionType{}

	var rows []Reaction
	if err := d.DB.Select("type").
		Where("post_id = ? AND user_id = ?", postID, userID).
		Find(&rows).Error; err != nil {
		return result
	}

	for _, row := range rows {
		result = append(result, row.Type)
	}
	return result
}

// ToggleReaction 切换反应。
func (d *Data) ToggleReaction(c *gin.Context) {
	user, ok := CurrentUser(c)
	if !ok {
		Response{}.Fail(c, http.StatusUnauthorized, "UNAUTHORIZED", "请先登录")
		return
	}

	postID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || postID == 0 {
		Response{}.Fail(c, http.StatusBadRequest, "INVALID_ID", "文章 ID 无效")
		return
	}

	var req ReactionInput
	if err := c.ShouldBindJSON(&req); err != nil {
		Response{}.Fail(c, http.StatusBadRequest, "INVALID_BODY", err.Error())
		return
	}
	if !isValidReactionType(req.Type) {
		Response{}.Fail(c, http.StatusBadRequest, "INVALID_REACTION", "不支持的反应类型")
		return
	}

	// JSON 列的读改写用进程内锁保护。
	d.mu.Lock()
	defer d.mu.Unlock()

	var post Post
	if err := d.DB.Select("id", "user", "is_private", "root_id").Take(&post, postID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			Response{}.Fail(c, http.StatusNotFound, "POST_NOT_FOUND", "文章不存在")
			return
		}
		Response{}.Fail(c, http.StatusInternalServerError, "QUERY_FAILED", err.Error())
		return
	}

	if !d.canViewPostThread(post, user) {
		Response{}.Fail(c, http.StatusNotFound, "POST_NOT_FOUND", "文章不存在")
		return
	}
	// 作者用户名，用于聚合作者总计。
	var authorID uint
	var author User
	if err := d.DB.Select("id").Where("name = ?", post.User).Take(&author).Error; err == nil {
		authorID = author.ID
	}

	err = d.DB.Transaction(func(tx *gorm.DB) error {
		var existing Reaction
		findErr := tx.Where("post_id = ? AND user_id = ? AND type = ?", post.ID, user.ID, req.Type).
			Take(&existing).Error

		delta := int64(1)
		switch {
		case errors.Is(findErr, gorm.ErrRecordNotFound):
			if err := tx.Create(&Reaction{PostID: post.ID, UserID: user.ID, Type: req.Type}).Error; err != nil {
				return err
			}
		case findErr != nil:
			return findErr
		default:
			if err := tx.Delete(&Reaction{}, existing.ID).Error; err != nil {
				return err
			}
			delta = -1
		}

		return applyReactionDelta(tx, post.ID, authorID, req.Type, delta)
	})
	if err != nil {
		Response{}.Fail(c, http.StatusInternalServerError, "REACTION_FAILED", err.Error())
		return
	}

	var updated Post
	if err := d.DB.Select("id", "like_count", "dislike_count", "question_count", "emoji_counts").
		Take(&updated, post.ID).Error; err != nil {
		Response{}.Fail(c, http.StatusInternalServerError, "QUERY_FAILED", err.Error())
		return
	}

	Response{}.Ok(c, http.StatusOK, ReactionResult{
		Reactions: buildReactionSummary(
			updated.LikeCount, updated.DislikeCount, updated.QuestionCount, updated.EmojiCounts,
		),
		MyReactions: d.myReactionTypes(post.ID, user.ID),
	})
}

// applyReactionDelta 更新文章和作者计数。
func applyReactionDelta(tx *gorm.DB, postID, authorID uint, reactionType ReactionType, delta int64) error {
	column, isPrimary := primaryReactionColumns[reactionType]
	if isPrimary {
		if err := tx.Model(&Post{}).Where("id = ?", postID).
			UpdateColumn(column, gorm.Expr("MAX("+column+" + ?, 0)", delta)).Error; err != nil {
			return err
		}
		if authorID == 0 {
			return nil
		}
		return tx.Model(&User{}).Where("id = ?", authorID).
			UpdateColumn(column, gorm.Expr("MAX("+column+" + ?, 0)", delta)).Error
	}

	if err := updateEmojiCounts(tx, &Post{}, postID, reactionType, delta); err != nil {
		return err
	}
	if authorID == 0 {
		return nil
	}
	return updateEmojiCounts(tx, &User{}, authorID, reactionType, delta)
}

func updateEmojiCounts(tx *gorm.DB, model any, id uint, reactionType ReactionType, delta int64) error {
	var current struct {
		EmojiCounts string
	}
	if err := tx.Model(model).Select("emoji_counts").Where("id = ?", id).Take(&current).Error; err != nil {
		return err
	}

	counts := parseEmojiCounts(current.EmojiCounts)
	key := string(reactionType)
	counts[key] += delta
	if counts[key] <= 0 {
		delete(counts, key)
	}

	raw, err := json.Marshal(counts)
	if err != nil {
		return err
	}

	return tx.Model(model).Where("id = ?", id).UpdateColumn("emoji_counts", string(raw)).Error
}

// syncReactionCounts 全量重算反应计数，启动时执行。
func syncReactionCounts(db *gorm.DB) error {
	type row struct {
		PostID uint
		Owner  string
		Type   ReactionType
		Total  int64
	}

	var rows []row
	if err := db.Table("reactions").
		Select("reactions.post_id AS post_id, posts.user AS owner, reactions.type AS type, COUNT(*) AS total").
		Joins("JOIN posts ON posts.id = reactions.post_id AND posts.deleted_at IS NULL").
		Group("reactions.post_id, posts.user, reactions.type").
		Scan(&rows).Error; err != nil {
		return fmt.Errorf("统计反应明细失败：%w", err)
	}

	type aggregate struct {
		like, dislike, question int64
		emoji                   map[string]int64
	}
	newAggregate := func() *aggregate {
		return &aggregate{emoji: map[string]int64{}}
	}
	addTo := func(agg *aggregate, reactionType ReactionType, total int64) {
		switch reactionType {
		case ReactionLike:
			agg.like += total
		case ReactionDislike:
			agg.dislike += total
		case ReactionQuestion:
			agg.question += total
		default:
			agg.emoji[string(reactionType)] += total
		}
	}

	postAggregates := make(map[uint]*aggregate)
	userAggregates := make(map[string]*aggregate)

	for _, item := range rows {
		if _, ok := postAggregates[item.PostID]; !ok {
			postAggregates[item.PostID] = newAggregate()
		}
		addTo(postAggregates[item.PostID], item.Type, item.Total)

		if item.Owner == "" {
			continue
		}
		if _, ok := userAggregates[item.Owner]; !ok {
			userAggregates[item.Owner] = newAggregate()
		}
		addTo(userAggregates[item.Owner], item.Type, item.Total)
	}

	// 先清零，再写回有计数的行。
	if err := db.Exec("UPDATE posts SET like_count = 0, dislike_count = 0, question_count = 0, emoji_counts = ''").Error; err != nil {
		return fmt.Errorf("重置文章反应统计失败：%w", err)
	}
	if err := db.Exec("UPDATE users SET like_count = 0, dislike_count = 0, question_count = 0, emoji_counts = ''").Error; err != nil {
		return fmt.Errorf("重置用户反应统计失败：%w", err)
	}

	for postID, agg := range postAggregates {
		emoji, err := json.Marshal(agg.emoji)
		if err != nil {
			return err
		}
		if err := db.Exec(
			"UPDATE posts SET like_count = ?, dislike_count = ?, question_count = ?, emoji_counts = ? WHERE id = ?",
			agg.like, agg.dislike, agg.question, string(emoji), postID,
		).Error; err != nil {
			return fmt.Errorf("更新文章反应统计失败：%w", err)
		}
	}

	for name, agg := range userAggregates {
		emoji, err := json.Marshal(agg.emoji)
		if err != nil {
			return err
		}
		if err := db.Exec(
			"UPDATE users SET like_count = ?, dislike_count = ?, question_count = ?, emoji_counts = ? WHERE name = ?",
			agg.like, agg.dislike, agg.question, string(emoji), name,
		).Error; err != nil {
			return fmt.Errorf("更新用户反应统计失败：%w", err)
		}
	}

	return nil
}
