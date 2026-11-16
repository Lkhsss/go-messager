package main

import (
	"net/http"
	"testing"
)

type statusWithRegisterPayload struct {
	Initialized     bool `json:"initialized"`
	RegisterEnabled bool `json:"register_enabled"`
}

// newRegisterDisabledEnv 构造关闭注册的测试环境。
func newRegisterDisabledEnv(t *testing.T) *testEnv {
	t.Helper()
	return newTestEnvWithConfig(t, func(cfg *Config) {
		cfg.Register.Enabled = false
	})
}

func TestRegisterDisabledAllowsFirstAdmin(t *testing.T) {
	env := newRegisterDisabledEnv(t)

	admin := env.register(t, "admin", "admin123")
	if admin.User.Role != RoleAdmin {
		t.Fatalf("首个用户应为管理员，实际 %s", admin.User.Role)
	}

	code, resp := env.do(t, http.MethodGet, "/api/status", "", nil)
	if code != http.StatusOK {
		t.Fatalf("系统状态应 200，实际 %d：%s", code, describe(resp))
	}
	if payload := decode[statusWithRegisterPayload](t, resp); payload.RegisterEnabled {
		t.Fatalf("register_enabled 应为 false：%+v", payload)
	}
}

func TestRegisterDisabledBlocksLaterUsers(t *testing.T) {
	env := newRegisterDisabledEnv(t)

	admin := env.register(t, "admin", "admin123")

	// 关闭注册后不再要求验证码，直接返回注册已关闭。
	code, resp := env.do(t, http.MethodPost, "/api/register", "",
		RegisterInput{Name: "bob", Password: "bob12345"})
	if code != http.StatusForbidden {
		t.Fatalf("关闭注册后注册应 403，实际 %d：%s", code, describe(resp))
	}
	if resp.Error == nil || resp.Error.Code != "REGISTER_DISABLED" {
		t.Fatalf("错误码应为 REGISTER_DISABLED：%s", describe(resp))
	}

	var count int64
	if err := env.db.Model(&User{}).Where("name = ?", "bob").Count(&count).Error; err != nil {
		t.Fatalf("统计 bob 失败：%v", err)
	}
	if count != 0 {
		t.Fatalf("关闭注册后不应创建用户，实际 %d", count)
	}

	// 关闭注册不影响已有账号登录。
	code, resp = env.do(t, http.MethodPost, "/api/authorize", "",
		Credentials{Name: "admin", Password: "admin123"})
	if code != http.StatusOK {
		t.Fatalf("关闭注册不应影响登录，实际 %d：%s", code, describe(resp))
	}
	if result := decode[AuthResult](t, resp); result.Token == "" || result.User.Name != admin.User.Name {
		t.Fatalf("登录结果异常：%+v", result)
	}
}

func TestRegisterEnabledByDefault(t *testing.T) {
	env := newTestEnv(t)

	code, resp := env.do(t, http.MethodGet, "/api/status", "", nil)
	if code != http.StatusOK {
		t.Fatalf("系统状态应 200，实际 %d：%s", code, describe(resp))
	}
	if payload := decode[statusWithRegisterPayload](t, resp); !payload.RegisterEnabled {
		t.Fatalf("默认应开启注册：%+v", payload)
	}

	env.register(t, "admin", "admin123")
	env.register(t, "bob", "bob12345")
}
