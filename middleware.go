package main

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// ErrorHandler 捕获 handler 通过 c.Error 追加的错误，并返回统一格式的错误响应。
// 注意：handler 自己已经写过响应（c.Writer.Written()）时不再重复写。
func ErrorHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next() // 先执行后续 handler

		if len(c.Errors) > 0 && !c.Writer.Written() {
			err := c.Errors.Last().Err
			Response{}.Fail(c, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		}
	}
}
