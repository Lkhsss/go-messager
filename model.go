package main

import (
	"time"

	"gorm.io/gorm"
)

// Role 用户权限。用字符串而不是数字存库：可读、便于以后加角色，也不会因为调整顺序而改变含义。
type Role string

const (
	RoleAdmin Role = "admin" // 系统里第一个注册的用户，管理员
	RoleUser  Role = "user"  // 之后注册的普通用户
	RoleNone  Role = "none"  // 封禁/停用。暂时没有管理入口，但登录与鉴权时必须拦截
)

type User struct {
	gorm.Model
	Name string `gorm:"uniqueIndex" form:"name" json:"name" binding:"required"`
	// Password 存 bcrypt hash；json:"-" 防止以后误把 User 直接序列化出去
	Password string `form:"password" json:"-" binding:"required"`
	Role     Role   `gorm:"type:text;not null;default:'user'" json:"role"`
	// Avatar 头像 data URL（base64 存在数据库里，不直接序列化给客户端）
	Avatar string `gorm:"type:text" json:"-"`
	// 该用户发布的文章收到的反应总数（作者声望），启动时按 reactions 校准
	LikeCount     int64  `gorm:"not null;default:0" json:"-"`
	DislikeCount  int64  `gorm:"not null;default:0" json:"-"`
	QuestionCount int64  `gorm:"not null;default:0" json:"-"`
	EmojiCounts   string `gorm:"type:text" json:"-"`

	// 登录统计：只统计成功登录 /api/authorize 的次数
	LoginCount  int64      `gorm:"not null;default:0" json:"-"`
	LastLoginAt *time.Time `json:"-"`
}

// UserView 是对外暴露的用户信息，不含密码 hash 与头像 base64
type UserView struct {
	Name          string `json:"name"`
	Role          Role   `json:"role"`
	AvatarVersion string `json:"avatar_version,omitempty"` // 头像内容哈希，前端拼 /api/avatar 地址
	// 该用户文章收到的反应总和
	ReactionTotals ReactionSummary `json:"reaction_totals"`
}

// View 转成可安全返回给客户端的结构
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

// Credentials 是登录/注册的请求体，避免客户端直接绑定到 User（防止塞 ID、Role 等字段）
type Credentials struct {
	Name     string `form:"name" json:"name" binding:"required"`
	Password string `form:"password" json:"password" binding:"required"`
}

// RegisterInput 注册请求体
type RegisterInput struct {
	Name     string `json:"name" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// AvatarInput 上传头像的请求体（data URL）
type AvatarInput struct {
	Avatar string `json:"avatar"`
}

type Post struct {
	gorm.Model
	User     string `json:"user"` // 作者由服务端根据登录态写入，不接受客户端传值
	Title    string `json:"title" binding:"required"`
	Subtitle string `json:"subtitle"`
	Body     string `json:"body" binding:"required"`
	// ParentID/RootID 用于评论树：原文两者都为空；顶层评论 RootID=原文，ParentID 为空；回复同时指向父评论和原文
	ParentID *uint `gorm:"index" json:"parent_id,omitempty"`
	RootID   *uint `gorm:"index" json:"root_id,omitempty"`

	// IsPrivate 为 true 时仅作者与管理员可见；删除仍使用 gorm.Model 的 DeletedAt 软删除
	IsPrivate bool `gorm:"not null;default:false" json:"private"`
	// Tags 与 Tag 是多对多关系，可通过 post.Tags 查标签，也可通过 tag.Posts 反查文章
	Tags []Tag `gorm:"many2many:post_tags;" json:"tags"`
	// UserAvatarVersion 是作者头像内容的哈希，不落库；前端用它拼 /api/avatar 地址
	UserAvatarVersion string `gorm:"-" json:"user_avatar_version,omitempty"`

	// 反应统计（冗余列，启动时按 reactions 全量校准）
	LikeCount     int64  `gorm:"not null;default:0" json:"-"`
	DislikeCount  int64  `gorm:"not null;default:0" json:"-"`
	QuestionCount int64  `gorm:"not null;default:0" json:"-"`
	EmojiCounts   string `gorm:"type:text" json:"-"`

	// 响应专用（不落库）
	Reactions   ReactionSummary `gorm:"-" json:"reactions"`
	MyReactions []ReactionType  `gorm:"-" json:"my_reactions"`
}

// ReactionType 反应类型。前三个是主反应，其余为 GitHub 风格表情。
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

// Reaction 一条反应明细。(post_id, user_id, type) 唯一：一人每种只能点一次。
type Reaction struct {
	ID        uint         `gorm:"primaryKey"`
	PostID    uint         `gorm:"uniqueIndex:idx_reaction_unique,priority:1"`
	UserID    uint         `gorm:"uniqueIndex:idx_reaction_unique,priority:2"`
	Type      ReactionType `gorm:"uniqueIndex:idx_reaction_unique,priority:3;size:16"`
	CreatedAt time.Time
}

// ReactionSummary 反应统计（文章或用户维度）
type ReactionSummary struct {
	Like     int64            `json:"like"`
	Dislike  int64            `json:"dislike"`
	Question int64            `json:"question"`
	Emoji    map[string]int64 `json:"emoji,omitempty"`
}

// ReactionInput 切换反应的请求体
type ReactionInput struct {
	Type ReactionType `json:"type" binding:"required"`
}

// ReactionResult 切换反应后的最新状态
type ReactionResult struct {
	Reactions   ReactionSummary `json:"reactions"`
	MyReactions []ReactionType  `json:"my_reactions"`
}

// Tag 文章标签。只能由管理员创建，普通用户发帖时最多选择 5 个。
type Tag struct {
	gorm.Model
	Name string `gorm:"uniqueIndex" form:"name" json:"name" binding:"required"`
	// Color 存 #rrggbb；空字符串表示前端使用默认的高级灰
	Color string `gorm:"size:7" json:"color"`
	// UsageCount 统计被多少篇（未软删除的）文章使用，用于首页热门标签
	UsageCount int64  `gorm:"not null;default:0" json:"usage_count"`
	Posts      []Post `gorm:"many2many:post_tags;" json:"posts,omitempty"`
}

// TagInput 创建标签的请求体
type TagInput struct {
	Name  string `json:"name" binding:"required"`
	Color string `json:"color"` // 可选，#RGB / #RRGGBB，可省略 #
}

// PostInput 是创建帖子的请求体：没有 user 字段，作者只能来自 token
type PostInput struct {
	Title    string `json:"title" binding:"required"`
	Subtitle string `json:"subtitle"`
	Body     string `json:"body" binding:"required"`
	// TagIDs 是已有标签的 ID，最多 maxTagsPerPost 个
	TagIDs []uint `json:"tag_ids"`
}

// CommentInput 发表评论/回复的请求体。
type CommentInput struct {
	Body     string `json:"body" binding:"required"`
	ParentID *uint  `json:"parent_id"`
}

// 设置的条目
type Setting struct {
	gorm.Model
	Key   string `gorm:"uniqueIndex" json:"key"`
	Value string `json:"value"`
}
