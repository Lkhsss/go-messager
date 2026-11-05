package main

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// startTime 记录进程启动时刻，由 StartTimer 在 main 开头写入。
// 这里不用 time.Ticker / goroutine 累加：time.Since 基于单调时钟读取，
// 零开销且不受系统时间调整（NTP 校时、手动改表）影响。
var startTime time.Time

// StartTimer 启动计时器：记录进程启动时刻，需在 main() 最开始调用。
func StartTimer() {
	startTime = time.Now()
}

// UptimeSeconds 返回从启动到现在的秒数（向下取整）。
// 未调用 StartTimer 时返回 0，避免返回无意义的时间差。
func UptimeSeconds() int64 {
	if startTime.IsZero() {
		return 0
	}
	return int64(time.Since(startTime) / time.Second)
}

// Uptime 运行时长 GET /api/uptime
func (d *Data) Uptime(c *gin.Context) {
	Response{}.Ok(c, http.StatusOK, gin.H{
		"uptime_seconds": UptimeSeconds(),
	})
}

// Status 系统状态 GET /api/status
// 前端用它判断是否需要进入「初始化系统」流程（创建第一个用户）。
func (d *Data) Status(c *gin.Context) {
	var count int64
	if err := d.DB.Model(&User{}).Count(&count).Error; err != nil {
		Response{}.Fail(c, http.StatusInternalServerError, "COUNT_FAILED", err.Error())
		return
	}

	Response{}.Ok(c, http.StatusOK, gin.H{"initialized": count > 0})
}
