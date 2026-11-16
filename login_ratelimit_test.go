package main

import (
	"net/http"
	"testing"
)

func TestLoginRateLimitedByIP(t *testing.T) {
	env := newTestEnv(t)
	env.register(t, "alice", "alice123")

	// 打满 IP 每分钟上限：前几次返回 401，账号失败锁定后返回 429，但都会计入 IP 次数。
	for i := 0; i < loginIPPerMinute; i++ {
		env.do(t, http.MethodPost, "/api/authorize", "", Credentials{Name: "nobody", Password: "wrong-pass"})
	}

	code, resp := env.do(t, http.MethodPost, "/api/authorize", "",
		Credentials{Name: "alice", Password: "alice123"})
	if code != http.StatusTooManyRequests {
		t.Fatalf("超过 IP 限流后应 429，实际 %d：%s", code, describe(resp))
	}
	if resp.Error == nil || resp.Error.Code != "LOGIN_RATE_LIMITED" {
		t.Fatalf("错误码应为 LOGIN_RATE_LIMITED：%s", describe(resp))
	}

	// 重置内部计数后可正常登录（模拟窗口过期）。
	env.data.loginIPLimiter.Reset("192.0.2.1")
	env.data.loginFailLimiter.Reset(loginFailureKey("192.0.2.1", "alice"))
	code, resp = env.do(t, http.MethodPost, "/api/authorize", "",
		Credentials{Name: "alice", Password: "alice123"})
	if code != http.StatusOK {
		t.Fatalf("重置限流后登录应 200，实际 %d：%s", code, describe(resp))
	}
}

func TestLoginLockedAfterFailures(t *testing.T) {
	env := newTestEnv(t)
	env.register(t, "alice", "alice123")

	for i := 0; i < loginFailureLimit; i++ {
		code, resp := env.do(t, http.MethodPost, "/api/authorize", "",
			Credentials{Name: "alice", Password: "wrong-pass"})
		if code != http.StatusUnauthorized {
			t.Fatalf("第 %d 次错误密码应 401，实际 %d：%s", i+1, code, describe(resp))
		}
	}

	// 达到失败上限后，即使密码正确也会被临时锁定。
	code, resp := env.do(t, http.MethodPost, "/api/authorize", "",
		Credentials{Name: "alice", Password: "alice123"})
	if code != http.StatusTooManyRequests {
		t.Fatalf("失败次数超限应 429，实际 %d：%s", code, describe(resp))
	}
	if resp.Error == nil || resp.Error.Code != "LOGIN_LOCKED" {
		t.Fatalf("错误码应为 LOGIN_LOCKED：%s", describe(resp))
	}
}

func TestLoginSuccessResetsFailureCount(t *testing.T) {
	env := newTestEnv(t)
	env.register(t, "alice", "alice123")

	for i := 0; i < loginFailureLimit-1; i++ {
		if code, resp := env.do(t, http.MethodPost, "/api/authorize", "",
			Credentials{Name: "alice", Password: "wrong-pass"}); code != http.StatusUnauthorized {
			t.Fatalf("错误密码应 401，实际 %d：%s", code, describe(resp))
		}
	}

	if code, resp := env.do(t, http.MethodPost, "/api/authorize", "",
		Credentials{Name: "alice", Password: "alice123"}); code != http.StatusOK {
		t.Fatalf("正确密码应 200，实际 %d：%s", code, describe(resp))
	}

	// 成功登录清零失败计数，再错一次仍是 401 而不是 429。
	code, resp := env.do(t, http.MethodPost, "/api/authorize", "",
		Credentials{Name: "alice", Password: "wrong-pass"})
	if code != http.StatusUnauthorized {
		t.Fatalf("成功登录后失败计数应清零，实际 %d：%s", code, describe(resp))
	}
}
