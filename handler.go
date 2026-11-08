package main

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// 进程启动时刻。
var startTime time.Time

// StartTimer 记录启动时刻。
func StartTimer() {
	startTime = time.Now()
}

// UptimeSeconds 返回启动至今的秒数。
func UptimeSeconds() int64 {
	if startTime.IsZero() {
		return 0
	}
	return int64(time.Since(startTime) / time.Second)
}

// Uptime GET /api/uptime
func (d *Data) Uptime(c *gin.Context) {
	payload := gin.H{
		"uptime_seconds": UptimeSeconds(),
	}
	// 未启动时不返回 started_at。
	if !startTime.IsZero() {
		payload["started_at"] = startTime
	}
	Response{}.Ok(c, http.StatusOK, payload)
}

// Status 是否已初始化。
func (d *Data) Status(c *gin.Context) {
	var count int64
	if err := d.DB.Model(&User{}).Count(&count).Error; err != nil {
		Response{}.Fail(c, http.StatusInternalServerError, "COUNT_FAILED", err.Error())
		return
	}

	Response{}.Ok(c, http.StatusOK, gin.H{"initialized": count > 0})
}
