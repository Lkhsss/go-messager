package main

import (
	"fmt"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
)

// maintenanceInput 管理员保存维护模式配置的入参。
type maintenanceInput struct {
	Enabled bool   `json:"enabled"`
	Title   string `json:"title"`
	Body    string `json:"body"`
}

// GetMaintenance 读取维护模式配置 GET /api/admin/maintenance
func (d *Data) GetMaintenance(c *gin.Context) {
	Response{}.Ok(c, http.StatusOK, d.MaintenanceState())
}

// UpdateMaintenance 保存维护模式配置 PUT /api/admin/maintenance
func (d *Data) UpdateMaintenance(c *gin.Context) {
	var req maintenanceInput
	if err := c.ShouldBindJSON(&req); err != nil {
		Response{}.Fail(c, http.StatusBadRequest, "INVALID_BODY", err.Error())
		return
	}

	state := MaintenanceState{
		Enabled: req.Enabled,
		Title:   strings.TrimSpace(req.Title),
		Body:    strings.TrimSpace(req.Body),
	}
	if state.Title == "" {
		state.Title = maintenanceDefaultTitle
	}
	if utf8.RuneCountInString(state.Title) > maintenanceTitleMaxRune {
		Response{}.Fail(c, http.StatusBadRequest, "TITLE_TOO_LONG",
			fmt.Sprintf("标题最多 %d 个字符", maintenanceTitleMaxRune))
		return
	}
	if utf8.RuneCountInString(state.Body) > maintenanceBodyMaxRune {
		Response{}.Fail(c, http.StatusBadRequest, "BODY_TOO_LONG",
			fmt.Sprintf("正文最多 %d 个字符", maintenanceBodyMaxRune))
		return
	}

	if err := d.saveMaintenance(state); err != nil {
		Response{}.Fail(c, http.StatusInternalServerError, "SAVE_FAILED", err.Error())
		return
	}

	Response{}.Ok(c, http.StatusOK, d.MaintenanceState())
}
