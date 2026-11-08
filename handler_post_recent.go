package main

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// GetRecentPostCount 近 24 小时原文数。
func (d *Data) GetRecentPostCount(c *gin.Context) {
	user, ok := CurrentUser(c)
	if !ok {
		Response{}.Fail(c, http.StatusUnauthorized, "UNAUTHORIZED", "请先登录")
		return
	}

	var count int64
	db := d.DB.Model(&Post{}).
		Where("root_id IS NULL AND created_at >= ?", time.Now().Add(-24*time.Hour))
	if err := applyPostVisibility(db, user).Count(&count).Error; err != nil {
		Response{}.Fail(c, http.StatusInternalServerError, "COUNT_FAILED", err.Error())
		return
	}

	Response{}.Ok(c, http.StatusOK, gin.H{"count": count})
}
