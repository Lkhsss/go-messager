package main

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

// captchaVerifyInput 验证码作答请求体。
type captchaVerifyInput struct {
	ID     string `json:"id" binding:"required"`
	Answer string `json:"answer" binding:"required"`
}

// GetCaptcha GET /api/captcha 生成注册用图形验证码。
func (d *Data) GetCaptcha(c *gin.Context) {
	// 验证码一次有效，禁止浏览器和中间层缓存。
	c.Header("Cache-Control", "no-store")

	challenge, err := d.captcha.Create(c.ClientIP())
	if err != nil {
		respondCaptchaError(c, err)
		return
	}

	Response{}.Ok(c, http.StatusOK, challenge)
}

// VerifyCaptcha POST /api/captcha/verify 校验答案并签发一次性 ticket。
func (d *Data) VerifyCaptcha(c *gin.Context) {
	c.Header("Cache-Control", "no-store")

	var req captchaVerifyInput
	if err := c.ShouldBind(&req); err != nil {
		Response{}.Fail(c, http.StatusBadRequest, "INVALID_BODY", err.Error())
		return
	}

	ticket, err := d.captcha.Verify(c.ClientIP(), req.ID, req.Answer)
	if err != nil {
		respondCaptchaError(c, err)
		return
	}

	Response{}.Ok(c, http.StatusOK, ticket)
}

// respondCaptchaError 把验证码错误映射成统一响应。
func respondCaptchaError(c *gin.Context, err error) {
	var captchaErr *captchaError
	if errors.As(err, &captchaErr) {
		status := http.StatusBadRequest
		switch captchaErr.Code {
		case codeCaptchaRateLimited, codeCaptchaTooMany:
			status = http.StatusTooManyRequests
		}
		Response{}.Fail(c, status, captchaErr.Code, captchaErr.Message)
		return
	}

	Response{}.Fail(c, http.StatusInternalServerError, "CAPTCHA_FAILED", err.Error())
}
