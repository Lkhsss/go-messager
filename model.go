package main

import "gorm.io/gorm"

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
}

// UserView 是对外暴露的用户信息，不含密码 hash
type UserView struct {
	Name string `json:"name"`
	Role Role   `json:"role"`
}

// View 转成可安全返回给客户端的结构
func (u User) View() UserView {
	return UserView{Name: u.Name, Role: u.Role}
}

// Credentials 是登录/注册的请求体，避免客户端直接绑定到 User（防止塞 ID、Role 等字段）
type Credentials struct {
	Name     string `form:"name" json:"name" binding:"required"`
	Password string `form:"password" json:"password" binding:"required"`
}

type Post struct {
	gorm.Model
	User     string `json:"user"` // 作者由服务端根据登录态写入，不接受客户端传值
	Title    string `json:"title" binding:"required"`
	Subtitle string `json:"subtitle"`
	Body     string `json:"body" binding:"required"`
	// Tags 与 Tag 是多对多关系，可通过 post.Tags 查标签，也可通过 tag.Posts 反查文章
	Tags []Tag `gorm:"many2many:post_tags;" json:"tags"`
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

// 设置的条目
type Setting struct {
	gorm.Model
	Key   string `gorm:"uniqueIndex" json:"key"`
	Value string `json:"value"`
}
