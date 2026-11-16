package main

import (
	"errors"
	"time"

	"gorm.io/gorm"
)

// 每日任务 key。
type DailyTaskKey string

const (
	DailyTaskLogin       DailyTaskKey = "login"
	DailyTaskArticle     DailyTaskKey = "article"
	DailyTaskComment     DailyTaskKey = "comment"
	DailyTaskInteraction DailyTaskKey = "interaction"
)

// 每日任务奖励与达成条件。
const (
	DailyLoginXP          int64 = 1
	DailyArticleXP        int64 = 5
	DailyCommentXP        int64 = 3
	DailyInteractionXP    int64 = 1
	DailyInteractionLimit int64 = 5
	DailyArticleMinChars  int64 = 200
	DailyCommentMinChars  int64 = 100
)

// XPGain 一次经验变化，随业务响应返回给前端用于 toast。
type XPGain struct {
	Task      DailyTaskKey `json:"task"`
	TaskLabel string       `json:"task_label"`
	Gained    int64        `json:"gained"`
	Exp       int64        `json:"exp"`
	Level     int          `json:"level"`
	LeveledUp bool         `json:"leveled_up"`
}

// DailyTaskState 每日任务当前状态。
type DailyTaskState struct {
	Key         DailyTaskKey `json:"key"`
	Label       string       `json:"label"`
	Description string       `json:"description"`
	Reward      int64        `json:"reward"`
	Progress    int64        `json:"progress"`
	Target      int64        `json:"target"`
	Completed   bool         `json:"completed"`
}

// ExperienceStatus 设置页经验条与每日任务数据。
type ExperienceStatus struct {
	Date             string           `json:"date"`
	Exp              int64            `json:"exp"`
	Level            int              `json:"level"`
	LevelStartExp    int64            `json:"level_start_exp"`
	NextLevelExp     int64            `json:"next_level_exp"`
	LevelProgressExp int64            `json:"level_progress_exp"`
	LevelNeedExp     int64            `json:"level_need_exp"`
	Tasks            []DailyTaskState `json:"tasks"`
}

// dailyTaskDate 每日任务按服务器本地日期分桶。
func dailyTaskDate(now time.Time) string {
	return now.Format("2006-01-02")
}

// dailyTaskLabel 任务展示名。
func dailyTaskLabel(task DailyTaskKey) string {
	switch task {
	case DailyTaskLogin:
		return "每日登录"
	case DailyTaskArticle:
		return "写文章"
	case DailyTaskComment:
		return "评论"
	case DailyTaskInteraction:
		return "互动"
	default:
		return string(task)
	}
}

// dailyLoginPending 当天是否还没领取登录经验。已完成时只有一次查询，
// 避免 /api/me 这类每次冷启动都会调用的接口反复开写事务。
func (d *Data) dailyLoginPending(userID uint, date string) (bool, error) {
	var progress DailyTaskProgress
	err := d.DB.Select("id", "login_done").
		Where("user_id = ? AND date = ?", userID, date).
		Take(&progress).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return true, nil
	}
	if err != nil {
		return false, err
	}

	return !progress.LoginDone, nil
}

// ensureDailyProgress 取当天进度，没有则创建；唯一索引兜底并发创建。
func ensureDailyProgress(tx *gorm.DB, userID uint, date string) (DailyTaskProgress, error) {
	var progress DailyTaskProgress
	err := tx.Where("user_id = ? AND date = ?", userID, date).Take(&progress).Error
	if err == nil {
		return progress, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return DailyTaskProgress{}, err
	}

	progress = DailyTaskProgress{UserID: userID, Date: date}
	if err := tx.Create(&progress).Error; err != nil {
		// 并发插入时唯一索引会报错，重新读取一次。
		var existing DailyTaskProgress
		if readErr := tx.Where("user_id = ? AND date = ?", userID, date).Take(&existing).Error; readErr == nil {
			return existing, nil
		}
		return DailyTaskProgress{}, err
	}

	return progress, nil
}

// markDailyTask 把一次性任务标记为完成，只有本次真正写入才发经验。
func markDailyTask(tx *gorm.DB, progressID uint, column string) (bool, error) {
	result := tx.Model(&DailyTaskProgress{}).
		Where("id = ? AND "+column+" = ?", progressID, false).
		Update(column, true)
	if result.Error != nil {
		return false, result.Error
	}

	return result.RowsAffected > 0, nil
}

// countDailyInteraction 累加互动次数，超过上限后不再计数。
func countDailyInteraction(tx *gorm.DB, progressID uint) (bool, error) {
	result := tx.Model(&DailyTaskProgress{}).
		Where("id = ? AND interaction_count < ?", progressID, DailyInteractionLimit).
		UpdateColumn("interaction_count", gorm.Expr("interaction_count + 1"))
	if result.Error != nil {
		return false, result.Error
	}

	return result.RowsAffected > 0, nil
}

// dailyTaskCompleted 判断当天任务是否已完成。已完成时直接跳过写事务，
// 避免高频写文章/互动时反复 INSERT ... ON CONFLICT 和条件 UPDATE。
func (d *Data) dailyTaskCompleted(userID uint, task DailyTaskKey, date string) (bool, error) {
	var progress DailyTaskProgress
	err := d.DB.Select("login_done", "article_done", "comment_done", "interaction_count").
		Where("user_id = ? AND date = ?", userID, date).
		Take(&progress).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	switch task {
	case DailyTaskLogin:
		return progress.LoginDone, nil
	case DailyTaskArticle:
		return progress.ArticleDone, nil
	case DailyTaskComment:
		return progress.CommentDone, nil
	case DailyTaskInteraction:
		return progress.InteractionCount >= DailyInteractionLimit, nil
	default:
		return true, nil
	}
}

// awardDailyTask 发放每日任务经验。今日已完成或达到上限时返回 nil。
func (d *Data) awardDailyTask(userID uint, task DailyTaskKey, reward int64, now time.Time) (*XPGain, error) {
	date := dailyTaskDate(now)

	done, err := d.dailyTaskCompleted(userID, task, date)
	if err != nil {
		return nil, err
	}
	if done {
		return nil, nil
	}

	var gain *XPGain
	err = d.DB.Transaction(func(tx *gorm.DB) error {
		progress, err := ensureDailyProgress(tx, userID, date)
		if err != nil {
			return err
		}

		var applied bool
		switch task {
		case DailyTaskLogin:
			applied, err = markDailyTask(tx, progress.ID, "login_done")
		case DailyTaskArticle:
			applied, err = markDailyTask(tx, progress.ID, "article_done")
		case DailyTaskComment:
			applied, err = markDailyTask(tx, progress.ID, "comment_done")
		case DailyTaskInteraction:
			applied, err = countDailyInteraction(tx, progress.ID)
		default:
			return nil
		}
		if err != nil || !applied {
			return err
		}

		var after int64
		if err := tx.Model(&User{}).Where("id = ?", userID).
			UpdateColumn("exp", gorm.Expr("exp + ?", reward)).Error; err != nil {
			return err
		}
		if err := tx.Model(&User{}).Where("id = ?", userID).Pluck("exp", &after).Error; err != nil {
			return err
		}

		before := after - reward
		beforeLevel, _, _ := levelFromExp(before)
		afterLevel, _, _ := levelFromExp(after)
		gain = &XPGain{
			Task:      task,
			TaskLabel: dailyTaskLabel(task),
			Gained:    reward,
			Exp:       after,
			Level:     afterLevel,
			LeveledUp: afterLevel > beforeLevel,
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return gain, nil
}

// experienceStatus 组装经验条和每日任务状态。
func (d *Data) experienceStatus(userID uint, now time.Time) (ExperienceStatus, error) {
	user, err := d.GetUserByID(userID)
	if err != nil {
		return ExperienceStatus{}, err
	}

	date := dailyTaskDate(now)
	var progress DailyTaskProgress
	if err := d.DB.Where("user_id = ? AND date = ?", userID, date).Take(&progress).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return ExperienceStatus{}, err
		}
		progress = DailyTaskProgress{UserID: userID, Date: date}
	}

	level, levelStart, nextLevel := levelFromExp(user.Exp)

	return ExperienceStatus{
		Date:             date,
		Exp:              user.Exp,
		Level:            level,
		LevelStartExp:    levelStart,
		NextLevelExp:     nextLevel,
		LevelProgressExp: user.Exp - levelStart,
		LevelNeedExp:     nextLevel - levelStart,
		Tasks: []DailyTaskState{
			{
				Key:         DailyTaskArticle,
				Label:       "写文章",
				Description: "发布一篇不少于 200 字的文章",
				Reward:      DailyArticleXP,
				Progress:    doneCount(progress.ArticleDone),
				Target:      1,
				Completed:   progress.ArticleDone,
			},
			{
				Key:         DailyTaskComment,
				Label:       "评论",
				Description: "发表一条不少于 100 字的评论",
				Reward:      DailyCommentXP,
				Progress:    doneCount(progress.CommentDone),
				Target:      1,
				Completed:   progress.CommentDone,
			},
			{
				Key:         DailyTaskInteraction,
				Label:       "互动",
				Description: "点赞、点踩或添加表情，每次 +1",
				Reward:      DailyInteractionXP,
				Progress:    progress.InteractionCount,
				Target:      DailyInteractionLimit,
				Completed:   progress.InteractionCount >= DailyInteractionLimit,
			},
			{
				Key:         DailyTaskLogin,
				Label:       "每日登录",
				Description: "每天首次登录自动发放",
				Reward:      DailyLoginXP,
				Progress:    doneCount(progress.LoginDone),
				Target:      1,
				Completed:   progress.LoginDone,
			},
		},
	}, nil
}

func doneCount(done bool) int64 {
	if done {
		return 1
	}

	return 0
}
