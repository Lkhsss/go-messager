package main

import (
	"net/http"
	"time"
	"unicode"

	"github.com/gin-gonic/gin"
)

type UserStatsResponse struct {
	PostCount            int64 `json:"post_count"`
	WordCount            int64 `json:"word_count"`
	Like                 int64 `json:"like"`
	Dislike              int64 `json:"dislike"`
	Question             int64 `json:"question"`
	CommentCount         int64 `json:"comment_count"`
	ReceivedCommentCount int64 `json:"received_comment_count"`
}

type ReactionStats struct {
	Like     int64 `json:"like"`
	Dislike  int64 `json:"dislike"`
	Question int64 `json:"question"`
	Total    int64 `json:"total"`
}

type TimePoint struct {
	Label string `json:"label"`
	Count int64  `json:"count"`
}

type TagUsage struct {
	Name  string `json:"name"`
	Count int64  `json:"count"`
}

type HeatmapPoint struct {
	Date  string `json:"date"`
	Count int64  `json:"count"`
}

type AdminStatsResponse struct {
	PostCount          int64          `json:"post_count"`
	UserCount          int64          `json:"user_count"`
	CommentCount       int64          `json:"comment_count"`
	LoginCount         int64          `json:"login_count"`
	RegistrationTotal  int64          `json:"registration_total"`
	RegistrationRecent int64          `json:"registration_recent"`
	TagCount           int64          `json:"tag_count"`
	Reactions          ReactionStats  `json:"reactions"`
	Unit               string         `json:"unit"`
	ArticleSeries      []TimePoint    `json:"article_series"`
	TagUsage           []TagUsage     `json:"tag_usage"`
	Heatmap            []HeatmapPoint `json:"heatmap"`
}

// GetMyStats 当前用户的文章/互动统计 GET /api/me/stats
func (d *Data) GetMyStats(c *gin.Context) {
	user, ok := CurrentUser(c)
	if !ok {
		Response{}.Fail(c, http.StatusUnauthorized, "UNAUTHORIZED", "请先登录")
		return
	}

	stats, err := d.loadUserStats(user.Name)
	if err != nil {
		Response{}.Fail(c, http.StatusInternalServerError, "QUERY_FAILED", err.Error())
		return
	}

	Response{}.Ok(c, http.StatusOK, stats)
}

// loadUserStats 按用户名统计文章/互动数据。
// 个人数据页与管理员用户详情共用，确保两处数字一致。
func (d *Data) loadUserStats(name string) (UserStatsResponse, error) {
	var posts []Post
	if err := d.DB.Select("char_count", "like_count", "dislike_count", "question_count").
		Where("user = ? AND root_id IS NULL", name).
		Find(&posts).Error; err != nil {
		return UserStatsResponse{}, err
	}

	stats := UserStatsResponse{PostCount: int64(len(posts))}
	for _, post := range posts {
		stats.WordCount += post.CharCount
		stats.Like += post.LikeCount
		stats.Dislike += post.DislikeCount
		stats.Question += post.QuestionCount
	}

	if err := d.DB.Model(&Post{}).
		Where("user = ? AND root_id IS NOT NULL", name).
		Count(&stats.CommentCount).Error; err != nil {
		return UserStatsResponse{}, err
	}

	if err := d.DB.Raw(`
SELECT COUNT(*) FROM posts c
JOIN posts r ON r.id = c.root_id AND r.deleted_at IS NULL
WHERE c.deleted_at IS NULL AND c.root_id IS NOT NULL AND r.user = ?
`, name).Scan(&stats.ReceivedCommentCount).Error; err != nil {
		return UserStatsResponse{}, err
	}

	return stats, nil
}

// countTextCharacters 按 Unicode 字符计数，忽略空白字符。
func countTextCharacters(parts ...string) int64 {
	var count int64
	for _, part := range parts {
		for _, r := range part {
			if !unicode.IsSpace(r) {
				count++
			}
		}
	}
	return count
}

// GetAdminStats 管理员全站统计 GET /api/admin/stats?unit=day
func (d *Data) GetAdminStats(c *gin.Context) {
	unit := c.DefaultQuery("unit", "day")
	switch unit {
	case "hour", "day", "week", "month":
	default:
		unit = "day"
	}

	now := time.Now()
	out := AdminStatsResponse{Unit: unit}
	fail := func(err error) bool {
		if err == nil {
			return false
		}
		Response{}.Fail(c, http.StatusInternalServerError, "QUERY_FAILED", err.Error())
		return true
	}

	if fail(d.DB.Model(&Post{}).Where("root_id IS NULL").Count(&out.PostCount).Error) {
		return
	}
	if fail(d.DB.Model(&User{}).Count(&out.UserCount).Error) {
		return
	}
	if fail(d.DB.Model(&Post{}).Where("root_id IS NOT NULL").Count(&out.CommentCount).Error) {
		return
	}

	var loginRow struct {
		Total int64
	}
	if fail(d.DB.Model(&User{}).Select("COALESCE(SUM(login_count), 0) AS total").Scan(&loginRow).Error) {
		return
	}
	out.LoginCount = loginRow.Total
	out.RegistrationTotal = out.UserCount

	if fail(d.DB.Model(&User{}).Where("created_at >= ?", now.AddDate(0, 0, -30)).Count(&out.RegistrationRecent).Error) {
		return
	}
	if fail(d.DB.Model(&Tag{}).Count(&out.TagCount).Error) {
		return
	}

	var reactionRows []struct {
		Type  ReactionType
		Total int64
	}
	if fail(d.DB.Model(&Reaction{}).Select("type, COUNT(*) AS total").Group("type").Scan(&reactionRows).Error) {
		return
	}
	for _, row := range reactionRows {
		out.Reactions.Total += row.Total
		switch row.Type {
		case ReactionLike:
			out.Reactions.Like = row.Total
		case ReactionDislike:
			out.Reactions.Dislike = row.Total
		case ReactionQuestion:
			out.Reactions.Question = row.Total
		}
	}

	var err error
	out.TagUsage, err = d.loadTagUsage()
	if fail(err) {
		return
	}
	out.ArticleSeries, err = d.articleSeries(unit, now)
	if fail(err) {
		return
	}
	out.Heatmap, err = d.interactionHeatmap(now)
	if fail(err) {
		return
	}

	Response{}.Ok(c, http.StatusOK, out)
}

func (d *Data) loadTagUsage() ([]TagUsage, error) {
	var tags []TagUsage
	if err := d.DB.Model(&Tag{}).
		Select("name, usage_count AS count").
		Where("usage_count > 0").
		Order("usage_count DESC, name ASC").
		Scan(&tags).Error; err != nil {
		return nil, err
	}
	if tags == nil {
		tags = []TagUsage{}
	}
	return tags, nil
}

func (d *Data) articleSeries(unit string, now time.Time) ([]TimePoint, error) {
	var start time.Time
	var size int
	var bucketAt func(int) time.Time
	var indexOf func(time.Time) int

	switch unit {
	case "hour":
		start = now.Truncate(time.Hour).Add(-23 * time.Hour)
		size = 24
		bucketAt = func(i int) time.Time { return start.Add(time.Duration(i) * time.Hour) }
		indexOf = func(t time.Time) int { return int(t.Sub(start) / time.Hour) }
	case "week":
		today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		weekday := (int(today.Weekday()) + 6) % 7
		start = today.AddDate(0, 0, -weekday-11*7)
		size = 12
		bucketAt = func(i int) time.Time { return start.AddDate(0, 0, i*7) }
		indexOf = func(t time.Time) int { return int(t.Sub(start) / (7 * 24 * time.Hour)) }
	case "month":
		start = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location()).AddDate(0, -11, 0)
		size = 12
		bucketAt = func(i int) time.Time { return start.AddDate(0, i, 0) }
		indexOf = func(t time.Time) int { return (t.Year()-start.Year())*12 + int(t.Month()-start.Month()) }
	default:
		today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		start = today.AddDate(0, 0, -29)
		size = 30
		bucketAt = func(i int) time.Time { return start.AddDate(0, 0, i) }
		indexOf = func(t time.Time) int { return int(t.Sub(start) / (24 * time.Hour)) }
	}

	points := make([]TimePoint, size)
	for i := 0; i < size; i++ {
		points[i] = TimePoint{Label: formatSeriesLabel(unit, bucketAt(i))}
	}

	var rows []struct {
		CreatedAt time.Time
	}
	if err := d.DB.Model(&Post{}).
		Select("created_at").
		Where("root_id IS NULL AND created_at >= ?", start).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		idx := indexOf(row.CreatedAt.In(now.Location()))
		if idx >= 0 && idx < size {
			points[idx].Count++
		}
	}

	return points, nil
}

func formatSeriesLabel(unit string, t time.Time) string {
	switch unit {
	case "hour":
		return t.Format("15:04")
	case "month":
		return t.Format("2006-01")
	default:
		return t.Format("01-02")
	}
}

func (d *Data) interactionHeatmap(now time.Time) ([]HeatmapPoint, error) {
	loc := now.Location()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	start := today.AddDate(0, 0, -364)

	points := make([]HeatmapPoint, 365)
	indexByDate := make(map[string]int, len(points))
	for i := range points {
		date := start.AddDate(0, 0, i)
		key := date.Format("2006-01-02")
		points[i] = HeatmapPoint{Date: key}
		indexByDate[key] = i
	}
	add := func(t time.Time) {
		key := t.In(loc).Format("2006-01-02")
		if i, ok := indexByDate[key]; ok {
			points[i].Count++
		}
	}

	var posts []struct {
		CreatedAt time.Time
	}
	if err := d.DB.Model(&Post{}).Select("created_at").Where("created_at >= ?", start).Find(&posts).Error; err != nil {
		return nil, err
	}
	for _, row := range posts {
		add(row.CreatedAt)
	}

	var reactions []struct {
		CreatedAt time.Time
	}
	if err := d.DB.Model(&Reaction{}).Select("created_at").Where("created_at >= ?", start).Find(&reactions).Error; err != nil {
		return nil, err
	}
	for _, row := range reactions {
		add(row.CreatedAt)
	}

	return points, nil
}
