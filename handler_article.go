package main

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// 创建文章 POST /api/post/new
func (d *Data) NewPost(c *gin.Context) {
	var post Post
	if err := c.ShouldBindJSON(&post); err != nil {
		Response{}.Fail(c, http.StatusBadRequest, "INVALID_BODY", err.Error())
		return
	}

	if err := d.DB.Create(&post).Error; err != nil {
		Response{}.Fail(c, http.StatusInternalServerError, "CREATE_FAILED", err.Error())
		return
	}

	Response{}.Ok(c, http.StatusCreated, post)
}

// 文章列表 GET /api/post?page=1&per_page=20
func (d *Data) GetPost(c *gin.Context) {
	page, perPage := parsePagination(c)

	// 总数用于计算总页数，不需要 order/limit/offset
	total, err := gorm.G[Post](d.DB).Count(c, "*")
	if err != nil {
		Response{}.Fail(c, http.StatusInternalServerError, "COUNT_FAILED", err.Error())
		return
	}

	// 新文章在前：created_at DESC，同一时间戳用 id DESC 兜底，保证翻页顺序稳定
	posts, err := gorm.G[Post](d.DB).
		Order("created_at DESC, id DESC").
		Limit(perPage).
		Offset((page - 1) * perPage).
		Find(c)
	if err != nil {
		Response{}.Fail(c, http.StatusInternalServerError, "QUERY_FAILED", err.Error())
		return
	}

	if posts == nil {
		posts = []Post{} // 空页返回 [] 而不是 null，前端好处理
	}

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
