package main

import (
	"time"

	"gorm.io/gorm"
)

// Role 用户权限。存字符串，调整顺序不会改变含义。
type Role string

const (
	RoleAdmin Role = "admin" // 首个用户
	RoleUser  Role = "user"  // 普通用户
	RoleNone  Role = "none"  // 封禁/停用
)

type User struct {
	gorm.Model
	Name string `gorm:"uniqueIndex" form:"name" json:"name" binding:"required"`
	// Password 存 bcrypt hash，不序列化。
	Password string `form:"password" json:"-" binding:"required"`
	Role     Role   `gorm:"type:text;not null;default:'user'" json:"role"`
	// Avatar 头像 data URL，不序列化。
	Avatar string `gorm:"type:text" json:"-"`
	// 文章收到的反应总数，启动时按 reactions 校准。
	LikeCount     int64  `gorm:"not null;default:0" json:"-"`
	DislikeCount  int64  `gorm:"not null;default:0" json:"-"`
	QuestionCount int64  `gorm:"not null;default:0" json:"-"`
	EmojiCounts   string `gorm:"type:text" json:"-"`

	// 登录统计，只计成功登录。
	LoginCount  int64      `gorm:"not null;default:0" json:"-"`
	LastLoginAt *time.Time `json:"-"`
}

// UserView 对外用户信息。
type UserView struct {
	Name           string          `json:"name"`
	Role           Role            `json:"role"`
	AvatarVersion  string          `json:"avatar_version,omitempty"`
	ReactionTotals ReactionSummary `json:"reaction_totals"`
}

// View 返回对外结构。
func (u User) View() UserView {
	return UserView{
		Name:          u.Name,
		Role:          u.Role,
		AvatarVersion: avatarVersion(u.Avatar),
		ReactionTotals: buildReactionSummary(
			u.LikeCount, u.DislikeCount, u.QuestionCount, u.EmojiCounts,
		),
	}
}

// Credentials 登录/注册请求体，避免绑定 User。
type Credentials struct {
	Name     string `form:"name" json:"name" binding:"required"`
	Password string `form:"password" json:"password" binding:"required"`
}

type RegisterInput struct {
	Name     string `json:"name" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// AvatarInput 头像 data URL。
type AvatarInput struct {
	Avatar string `json:"avatar"`
}

type Post struct {
	gorm.Model
	User     string `json:"user"` // 作者由服务端根据登录态写入，不接受客户端传值
	Title    string `json:"title" binding:"required"`
	Subtitle string `json:"subtitle"`
	Body     string `json:"body,omitempty" binding:"required"`
	// 评论树：原文为空；顶层评论只有 RootID；回复同时有 ParentID 和 RootID。
	ParentID *uint `gorm:"index" json:"parent_id,omitempty"`
	RootID   *uint `gorm:"index" json:"root_id,omitempty"`

	// 仅作者与管理员可见。
	IsPrivate bool `gorm:"not null;default:false" json:"private"`
	// Excerpt 纯文本摘要，CharCount 字数。
	Excerpt   string `gorm:"type:text" json:"excerpt"`
	CharCount int64  `gorm:"not null;default:0" json:"-"`

	// 多对多标签。
	Tags []Tag `gorm:"many2many:post_tags;" json:"tags"`
	// 作者头像版本，不落库。
	UserAvatarVersion string `gorm:"-" json:"user_avatar_version,omitempty"`

	// 冗余计数，启动时按 reactions 校准。
	LikeCount     int64  `gorm:"not null;default:0" json:"-"`
	DislikeCount  int64  `gorm:"not null;default:0" json:"-"`
	QuestionCount int64  `gorm:"not null;default:0" json:"-"`
	EmojiCounts   string `gorm:"type:text" json:"-"`

	// 仅响应使用。
	Reactions   ReactionSummary `gorm:"-" json:"reactions"`
	MyReactions []ReactionType  `gorm:"-" json:"my_reactions"`
}

// 前三个为主反应，其余为表情。
type ReactionType string

const (
	ReactionLike     ReactionType = "like"
	ReactionDislike  ReactionType = "dislike"
	ReactionQuestion ReactionType = "question"
	ReactionLaugh    ReactionType = "laugh"
	ReactionHooray   ReactionType = "hooray"
	ReactionHeart    ReactionType = "heart"
	ReactionRocket   ReactionType = "rocket"
	ReactionEyes     ReactionType = "eyes"
)

// 一人对一篇文章每种反应只能有一条。
type Reaction struct {
	ID        uint         `gorm:"primaryKey"`
	PostID    uint         `gorm:"uniqueIndex:idx_reaction_unique,priority:1"`
	UserID    uint         `gorm:"uniqueIndex:idx_reaction_unique,priority:2"`
	Type      ReactionType `gorm:"uniqueIndex:idx_reaction_unique,priority:3;size:16"`
	CreatedAt time.Time
}

type ReactionSummary struct {
	Like     int64            `json:"like"`
	Dislike  int64            `json:"dislike"`
	Question int64            `json:"question"`
	Emoji    map[string]int64 `json:"emoji,omitempty"`
}

type ReactionInput struct {
	Type ReactionType `json:"type" binding:"required"`
}

type ReactionResult struct {
	Reactions   ReactionSummary `json:"reactions"`
	MyReactions []ReactionType  `json:"my_reactions"`
}

// Tag 标签，仅管理员可创建。
type Tag struct {
	gorm.Model
	Name string `gorm:"uniqueIndex" form:"name" json:"name" binding:"required"`
	// #rrggbb，空值使用前端默认色。
	Color string `gorm:"size:7" json:"color"`
	// 未删除文章的使用次数。
	UsageCount int64  `gorm:"not null;default:0" json:"usage_count"`
	Posts      []Post `gorm:"many2many:post_tags;" json:"posts,omitempty"`
}

type TagInput struct {
	Name  string `json:"name" binding:"required"`
	Color string `json:"color"` // 可选，#RGB / #RRGGBB，可省略 #
}

// PostInput 文章请求体，作者由 token 决定。
type PostInput struct {
	Title    string `json:"title" binding:"required"`
	Subtitle string `json:"subtitle"`
	Body     string `json:"body" binding:"required"`
	// 最多 maxTagsPerPost 个。
	TagIDs []uint `json:"tag_ids"`
}

type CommentInput struct {
	Body     string `json:"body" binding:"required"`
	ParentID *uint  `json:"parent_id"`
}

// Setting 键值配置。
type Setting struct {
	gorm.Model
	Key   string `gorm:"uniqueIndex" json:"key"`
	Value string `json:"value"`
}
