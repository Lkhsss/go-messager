package main

import (
	"encoding/base64"
	"image/png"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestCaptchaChallengeIsPNG(t *testing.T) {
	env := newTestEnv(t)

	code, resp := env.do(t, http.MethodGet, "/api/captcha", "", nil)
	if code != http.StatusOK {
		t.Fatalf("获取验证码应 200，实际 %d：%s", code, describe(resp))
	}

	challenge := decode[CaptchaChallenge](t, resp)
	if challenge.ID == "" || challenge.ExpiresIn <= 0 {
		t.Fatalf("验证码响应异常：%+v", challenge)
	}

	const prefix = "data:image/png;base64,"
	if !strings.HasPrefix(challenge.Image, prefix) {
		t.Fatalf("验证码图片应为 PNG data URL，实际前 32 字符：%s", firstRunes(challenge.Image, 32))
	}

	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(challenge.Image, prefix))
	if err != nil {
		t.Fatalf("解码验证码图片失败：%v", err)
	}
	img, err := png.Decode(strings.NewReader(string(raw)))
	if err != nil {
		t.Fatalf("验证码图片不是合法 PNG：%v", err)
	}
	if img.Bounds().Dx() != captchaImageWidth || img.Bounds().Dy() != captchaImageHeight {
		t.Fatalf("验证码图片尺寸异常：%v", img.Bounds())
	}

	// 答案只存在服务端，长度和字符集符合约定。
	answer, ok := env.data.captcha.peekAnswer(challenge.ID)
	if !ok || len(answer) != captchaCodeLen {
		t.Fatalf("验证码答案异常：%q（存在=%v）", answer, ok)
	}
	for _, ch := range answer {
		if !strings.ContainsRune(captchaAlphabet, ch) {
			t.Fatalf("验证码答案包含非法字符：%q", answer)
		}
	}
}

func TestFirstUserSkipsCaptcha(t *testing.T) {
	env := newTestEnv(t)

	// 全新库初始化管理员不需要人机验证，否则首次部署会被锁死。
	code, resp := env.do(t, http.MethodPost, "/api/register", "",
		RegisterInput{Name: "admin", Password: "admin123"})
	if code != http.StatusCreated {
		t.Fatalf("首个用户注册应 201，实际 %d：%s", code, describe(resp))
	}
	if user := decode[AuthResult](t, resp).User; user.Role != RoleAdmin {
		t.Fatalf("首个用户应为管理员，实际 %+v", user)
	}
}

func TestRegisterRequiresCaptchaTicket(t *testing.T) {
	env := newTestEnv(t)
	env.register(t, "admin", "admin123")

	code, resp := env.do(t, http.MethodPost, "/api/register", "",
		RegisterInput{Name: "bob", Password: "bob12345"})
	if code != http.StatusBadRequest {
		t.Fatalf("缺少 ticket 应 400，实际 %d：%s", code, describe(resp))
	}
	if resp.Error == nil || resp.Error.Code != "CAPTCHA_REQUIRED" {
		t.Fatalf("缺少 ticket 错误码应为 CAPTCHA_REQUIRED，实际 %+v", resp.Error)
	}

	// 带上验证流程签发的 ticket 就能正常注册。
	if user := env.register(t, "bob", "bob12345"); user.User.Name != "bob" {
		t.Fatalf("带 ticket 注册失败：%+v", user.User)
	}
}

func TestCaptchaTicketSingleUse(t *testing.T) {
	env := newTestEnv(t)
	env.register(t, "admin", "admin123")

	ticket := env.solveCaptcha(t)

	code, resp := env.do(t, http.MethodPost, "/api/register", "",
		RegisterInput{Name: "bob", Password: "bob12345", CaptchaTicket: ticket})
	if code != http.StatusCreated {
		t.Fatalf("带 ticket 注册应 201，实际 %d：%s", code, describe(resp))
	}

	// 同一个 ticket 不能给第二个账号复用。
	code, resp = env.do(t, http.MethodPost, "/api/register", "",
		RegisterInput{Name: "carol", Password: "carol1234", CaptchaTicket: ticket})
	if code != http.StatusBadRequest || resp.Error == nil || resp.Error.Code != "CAPTCHA_REQUIRED" {
		t.Fatalf("重复使用 ticket 应 400 CAPTCHA_REQUIRED，实际 %d：%s", code, describe(resp))
	}
}

func TestCaptchaVerifyIsCaseInsensitiveAndOneShot(t *testing.T) {
	env := newTestEnv(t)

	code, resp := env.do(t, http.MethodGet, "/api/captcha", "", nil)
	if code != http.StatusOK {
		t.Fatalf("获取验证码应 200，实际 %d：%s", code, describe(resp))
	}
	challenge := decode[CaptchaChallenge](t, resp)
	answer, ok := env.data.captcha.peekAnswer(challenge.ID)
	if !ok {
		t.Fatalf("验证码[%s]不存在", challenge.ID)
	}

	// 输入小写字母也应通过。
	code, resp = env.do(t, http.MethodPost, "/api/captcha/verify", "",
		captchaVerifyInput{ID: challenge.ID, Answer: strings.ToLower(answer)})
	if code != http.StatusOK {
		t.Fatalf("小写答案应通过校验，实际 %d：%s", code, describe(resp))
	}
	if ticket := decode[CaptchaTicket](t, resp).Ticket; ticket == "" {
		t.Fatal("校验通过但没有签发 ticket")
	}

	// 同一道题只能通过一次。
	code, resp = env.do(t, http.MethodPost, "/api/captcha/verify", "",
		captchaVerifyInput{ID: challenge.ID, Answer: answer})
	if code != http.StatusBadRequest || resp.Error == nil || resp.Error.Code != codeCaptchaExpired {
		t.Fatalf("重复作答应 400 CAPTCHA_EXPIRED，实际 %d：%s", code, describe(resp))
	}
}

func TestCaptchaWrongAnswerAttempts(t *testing.T) {
	env := newTestEnv(t)

	code, resp := env.do(t, http.MethodGet, "/api/captcha", "", nil)
	if code != http.StatusOK {
		t.Fatalf("获取验证码应 200，实际 %d：%s", code, describe(resp))
	}
	challenge := decode[CaptchaChallenge](t, resp)
	answer, ok := env.data.captcha.peekAnswer(challenge.ID)
	if !ok {
		t.Fatalf("验证码[%s]不存在", challenge.ID)
	}
	wrong := "AAAA"
	if answer == wrong {
		wrong = "BBBB"
	}

	for i := 1; i < captchaMaxAttempts; i++ {
		code, resp = env.do(t, http.MethodPost, "/api/captcha/verify", "",
			captchaVerifyInput{ID: challenge.ID, Answer: wrong})
		if code != http.StatusBadRequest || resp.Error == nil || resp.Error.Code != codeCaptchaInvalid {
			t.Fatalf("第 %d 次错误应 400 CAPTCHA_INVALID，实际 %d：%s", i, code, describe(resp))
		}
	}

	// 最后一次错误后作废。
	code, resp = env.do(t, http.MethodPost, "/api/captcha/verify", "",
		captchaVerifyInput{ID: challenge.ID, Answer: wrong})
	if code != http.StatusTooManyRequests || resp.Error == nil || resp.Error.Code != codeCaptchaTooMany {
		t.Fatalf("错误次数耗尽应 429 CAPTCHA_TOO_MANY_ATTEMPTS，实际 %d：%s", code, describe(resp))
	}

	// 作废后即使答对也只能重新取题。
	code, resp = env.do(t, http.MethodPost, "/api/captcha/verify", "",
		captchaVerifyInput{ID: challenge.ID, Answer: answer})
	if code != http.StatusBadRequest || resp.Error == nil || resp.Error.Code != codeCaptchaExpired {
		t.Fatalf("作废后作答应 400 CAPTCHA_EXPIRED，实际 %d：%s", code, describe(resp))
	}
}

func TestCaptchaExpires(t *testing.T) {
	env := newTestEnv(t)

	// 用已过期的时间点生成，模拟用户题目停留过久。
	challenge, err := env.data.captcha.create(time.Now().Add(-3*captchaTTL), "192.0.2.1")
	if err != nil {
		t.Fatalf("生成验证码失败：%v", err)
	}

	code, resp := env.do(t, http.MethodPost, "/api/captcha/verify", "",
		captchaVerifyInput{ID: challenge.ID, Answer: "AAAA"})
	if code != http.StatusBadRequest || resp.Error == nil || resp.Error.Code != codeCaptchaExpired {
		t.Fatalf("过期验证码应 400 CAPTCHA_EXPIRED，实际 %d：%s", code, describe(resp))
	}
}

func TestCaptchaTicketExpires(t *testing.T) {
	store := NewCaptchaStore()

	challenge, err := store.create(time.Now(), "192.0.2.1")
	if err != nil {
		t.Fatalf("生成验证码失败：%v", err)
	}
	answer, ok := store.peekAnswer(challenge.ID)
	if !ok {
		t.Fatalf("验证码[%s]不存在", challenge.ID)
	}
	ticket, err := store.verify(time.Now(), "192.0.2.1", challenge.ID, answer)
	if err != nil {
		t.Fatalf("校验验证码失败：%v", err)
	}

	if store.consumeTicket(time.Now().Add(captchaTicketTTL+time.Second), ticket.Ticket) {
		t.Fatal("过期 ticket 不应通过")
	}
}

func TestCaptchaGenerationRateLimited(t *testing.T) {
	env := newTestEnv(t)

	for i := 0; i < captchaGeneratePerMinute; i++ {
		code, resp := env.do(t, http.MethodGet, "/api/captcha", "", nil)
		if code != http.StatusOK {
			t.Fatalf("第 %d 次获取验证码应 200，实际 %d：%s", i+1, code, describe(resp))
		}
	}

	code, resp := env.do(t, http.MethodGet, "/api/captcha", "", nil)
	if code != http.StatusTooManyRequests || resp.Error == nil || resp.Error.Code != codeCaptchaRateLimited {
		t.Fatalf("超过生成频率应 429 CAPTCHA_RATE_LIMITED，实际 %d：%s", code, describe(resp))
	}
}

func firstRunes(value string, n int) string {
	runes := []rune(value)
	if len(runes) <= n {
		return value
	}
	return string(runes[:n])
}

func TestCaptchaAlphabetHasGlyphs(t *testing.T) {
	for _, ch := range captchaAlphabet {
		if _, ok := captchaGlyphs[ch]; !ok {
			t.Fatalf("字符 %q 缺少点阵字模", ch)
		}
	}
	if len(captchaAlphabet) != 32 {
		t.Fatalf("字符集长度应为 32，实际 %d", len(captchaAlphabet))
	}
}
