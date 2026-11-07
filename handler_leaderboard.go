package main

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type LeaderboardEntry struct {
	Post  Post  `json:"post"`
	Count int64 `json:"count"`
}

type LeaderboardResponse struct {
	Period   string             `json:"period"`
	Since    time.Time          `json:"since"`
	Like     []LeaderboardEntry `json:"like"`
	Dislike  []LeaderboardEntry `json:"dislike"`
	Question []LeaderboardEntry `json:"question"`
}

type leaderboardRow struct {
	PostID uint
	Count  int64
}

// GetLeaderboard 点赞/点踩/疑问排行榜 GET /api/leaderboard?period=week
// period: day / week / month / year；只统计原文，不统计评论。
func (d *Data) GetLeaderboard(c *gin.Context) {
	user, ok := CurrentUser(c)
	if !ok {
		Response{}.Fail(c, http.StatusUnauthorized, "UNAUTHORIZED", "请先登录")
		return
	}

	period := c.DefaultQuery("period", "week")
	now := time.Now()
	var since time.Time
	switch period {
	case "day":
		since = now.Add(-24 * time.Hour)
	case "week":
		since = now.Add(-7 * 24 * time.Hour)
	case "month":
		since = now.Add(-30 * 24 * time.Hour)
	case "year":
		since = now.Add(-365 * 24 * time.Hour)
	default:
		Response{}.Fail(c, http.StatusBadRequest, "INVALID_PERIOD", "period 必须是 day、week、month 或 year")
		return
	}

	likeRows, err := d.leaderboardRows(user, ReactionLike, since)
	if err != nil {
		Response{}.Fail(c, http.StatusInternalServerError, "QUERY_FAILED", err.Error())
		return
	}
	dislikeRows, err := d.leaderboardRows(user, ReactionDislike, since)
	if err != nil {
		Response{}.Fail(c, http.StatusInternalServerError, "QUERY_FAILED", err.Error())
		return
	}
	questionRows, err := d.leaderboardRows(user, ReactionQuestion, since)
	if err != nil {
		Response{}.Fail(c, http.StatusInternalServerError, "QUERY_FAILED", err.Error())
		return
	}

	like, err := d.leaderboardEntries(likeRows)
	if err != nil {
		Response{}.Fail(c, http.StatusInternalServerError, "QUERY_FAILED", err.Error())
		return
	}
	dislike, err := d.leaderboardEntries(dislikeRows)
	if err != nil {
		Response{}.Fail(c, http.StatusInternalServerError, "QUERY_FAILED", err.Error())
		return
	}
	question, err := d.leaderboardEntries(questionRows)
	if err != nil {
		Response{}.Fail(c, http.StatusInternalServerError, "QUERY_FAILED", err.Error())
		return
	}

	Response{}.Ok(c, http.StatusOK, LeaderboardResponse{
		Period:   period,
		Since:    since,
		Like:     like,
		Dislike:  dislike,
		Question: question,
	})
}

func (d *Data) leaderboardRows(user User, reactionType ReactionType, since time.Time) ([]leaderboardRow, error) {
	q := d.DB.Table("reactions").
		Select("reactions.post_id AS post_id, COUNT(*) AS count").
		Joins("JOIN posts ON posts.id = reactions.post_id AND posts.deleted_at IS NULL AND posts.root_id IS NULL").
		Where("reactions.type = ? AND reactions.created_at >= ?", reactionType, since)

	if user.Role != RoleAdmin {
		q = q.Where("(posts.is_private = ? OR posts.user = ?)", false, user.Name)
	}

	var rows []leaderboardRow
	if err := q.Group("reactions.post_id").
		Order("count DESC, reactions.post_id ASC").
		Limit(10).
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	if rows == nil {
		rows = []leaderboardRow{}
	}
	return rows, nil
}

func (d *Data) leaderboardEntries(rows []leaderboardRow) ([]LeaderboardEntry, error) {
	entries := make([]LeaderboardEntry, 0, len(rows))
	if len(rows) == 0 {
		return entries, nil
	}

	ids := make([]uint, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.PostID)
	}

	var posts []Post
	if err := d.DB.Select(
		"id", "created_at", "updated_at", "user", "title", "subtitle", "excerpt",
		"is_private", "parent_id", "root_id", "like_count", "dislike_count",
		"question_count", "emoji_counts",
	).Preload("Tags").Where("id IN ?", ids).Find(&posts).Error; err != nil {
		return nil, err
	}

	byID := make(map[uint]Post, len(posts))
	for _, post := range posts {
		byID[post.ID] = post
	}
	d.fillPostAvatars(posts)
	for _, post := range posts {
		byID[post.ID] = post
	}

	for _, row := range rows {
		post, ok := byID[row.PostID]
		if !ok {
			continue
		}
		entries = append(entries, LeaderboardEntry{Post: post, Count: row.Count})
	}
	return entries, nil
}

var _ = gorm.ErrRecordNotFound
