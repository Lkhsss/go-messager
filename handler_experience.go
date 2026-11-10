package main

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// GetExperience 当前用户经验条与每日任务状态 GET /api/me/experience
func (d *Data) GetExperience(c *gin.Context) {
	user, ok := CurrentUser(c)
	if !ok {
		Response{}.Fail(c, http.StatusUnauthorized, "UNAUTHORIZED", "请先登录")
		return
	}

	status, err := d.experienceStatus(user.ID, time.Now())
	if err != nil {
		Response{}.Fail(c, http.StatusInternalServerError, "QUERY_FAILED", err.Error())
		return
	}

	Response{}.Ok(c, http.StatusOK, status)
}
