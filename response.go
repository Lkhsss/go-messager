package main

import (
	"github.com/gin-gonic/gin"
)

// Response is the standard API envelope.
type Response struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   *ErrorInfo  `json:"error,omitempty"`
	Meta    *Meta       `json:"meta,omitempty"`
}

type ErrorInfo struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type Meta struct {
	Page       int `json:"page"`
	PerPage    int `json:"per_page"`
	Total      int `json:"total"`
	TotalPages int `json:"total_pages"`
}

func (r Response) Ok(c *gin.Context, status int, data any) {
	c.JSON(status, Response{
		Success: true,
		Data:    data,
	})
}

// OkWithMeta 返回带分页元信息（页码/每页数量/总数/总页数）的成功响应
func (r Response) OkWithMeta(c *gin.Context, status int, data any, meta *Meta) {
	c.JSON(status, Response{
		Success: true,
		Data:    data,
		Meta:    meta,
	})
}

func (r Response) Fail(c *gin.Context, status int, code, message string) {
	c.JSON(status, Response{
		Success: false,
		Error:   &ErrorInfo{Code: code, Message: message},
	})
}
