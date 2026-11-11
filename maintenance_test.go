package main

import (
	"net/http"
	"strings"
	"testing"
)

func TestMaintenanceModeBlocksNonAdmins(t *testing.T) {
	env := newTestEnv(t)

	admin := env.register(t, "admin", "admin123") // 首个用户自动成为管理员
	plain := env.register(t, "bob", "bob12345")

	// 开启维护，标题留空时用默认文案。
	code, resp := env.do(t, http.MethodPut, "/api/admin/maintenance", admin.Token,
		maintenanceInput{Enabled: true, Body: "预计 22:00 恢复"})
	if code != http.StatusOK {
		t.Fatalf("保存维护配置应 200，实际 %d：%s", code, describe(resp))
	}
	if state := decode[MaintenanceState](t, resp); !state.Enabled ||
		state.Title != maintenanceDefaultTitle || state.Body != "预计 22:00 恢复" {
		t.Fatalf("维护配置异常：%+v", state)
	}

	// 普通用户的读写接口一律 503。
	for _, target := range []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/post"},
		{http.MethodGet, "/api/me"},
		{http.MethodPost, "/api/post/new"},
	} {
		code, resp = env.do(t, target.method, target.path, plain.Token, nil)
		if code != http.StatusServiceUnavailable {
			t.Fatalf("%s %s 维护期间应 503，实际 %d：%s", target.method, target.path, code, describe(resp))
		}
		if resp.Error == nil || resp.Error.Code != "MAINTENANCE" {
			t.Fatalf("%s %s 错误码应为 MAINTENANCE，实际 %+v", target.method, target.path, resp.Error)
		}
	}

	// 白名单接口保持可用：维护页要读状态，容器健康检查要读运行时长。
	code, resp = env.do(t, http.MethodGet, "/api/status", "", nil)
	if code != http.StatusOK {
		t.Fatalf("/api/status 维护期间应 200，实际 %d：%s", code, describe(resp))
	}
	status := decode[struct {
		Initialized bool             `json:"initialized"`
		Mode        string           `json:"mode"`
		Maintenance MaintenanceState `json:"maintenance"`
	}](t, resp)
	if !status.Maintenance.Enabled || status.Maintenance.Title != maintenanceDefaultTitle {
		t.Fatalf("/api/status 维护信息异常：%+v", status.Maintenance)
	}
	if code, resp = env.do(t, http.MethodGet, "/api/uptime", "", nil); code != http.StatusOK {
		t.Fatalf("/api/uptime 维护期间应 200，实际 %d：%s", code, describe(resp))
	}

	// 管理员完全放行。
	if code, resp = env.do(t, http.MethodGet, "/api/post", admin.Token, nil); code != http.StatusOK {
		t.Fatalf("管理员访问应 200，实际 %d：%s", code, describe(resp))
	}

	// 维护期间只有管理员能登录，注册一律拒绝。
	code, resp = env.do(t, http.MethodPost, "/api/authorize", "", Credentials{Name: "bob", Password: "bob12345"})
	if code != http.StatusServiceUnavailable || resp.Error == nil || resp.Error.Code != "MAINTENANCE" {
		t.Fatalf("维护期间普通用户登录应 503 MAINTENANCE，实际 %d：%s", code, describe(resp))
	}
	code, resp = env.do(t, http.MethodPost, "/api/authorize", "", Credentials{Name: "admin", Password: "admin123"})
	if code != http.StatusOK {
		t.Fatalf("维护期间管理员登录应 200，实际 %d：%s", code, describe(resp))
	}
	if relogin := decode[AuthResult](t, resp); relogin.Token == "" {
		t.Fatalf("维护期间管理员登录应返回 token")
	}
	code, resp = env.do(t, http.MethodPost, "/api/register", "", Credentials{Name: "carol", Password: "carol12345"})
	if code != http.StatusServiceUnavailable || resp.Error == nil || resp.Error.Code != "MAINTENANCE" {
		t.Fatalf("维护期间注册应 503 MAINTENANCE，实际 %d：%s", code, describe(resp))
	}

	// 管理员手动关闭后，普通用户立即恢复。
	code, resp = env.do(t, http.MethodPut, "/api/admin/maintenance", admin.Token, maintenanceInput{Enabled: false})
	if code != http.StatusOK {
		t.Fatalf("关闭维护应 200，实际 %d：%s", code, describe(resp))
	}
	if state := decode[MaintenanceState](t, resp); state.Enabled {
		t.Fatalf("关闭后状态应为 false：%+v", state)
	}
	if code, resp = env.do(t, http.MethodGet, "/api/post", plain.Token, nil); code != http.StatusOK {
		t.Fatalf("恢复后普通用户应 200，实际 %d：%s", code, describe(resp))
	}
}

func TestMaintenanceValidation(t *testing.T) {
	env := newTestEnv(t)

	admin := env.register(t, "admin", "admin123")
	plain := env.register(t, "bob", "bob12345")

	// 非管理员不能修改维护配置。
	if code, resp := env.do(t, http.MethodPut, "/api/admin/maintenance", plain.Token,
		maintenanceInput{Enabled: true}); code != http.StatusForbidden {
		t.Fatalf("非管理员保存维护配置应 403，实际 %d：%s", code, describe(resp))
	}

	// 标题超长。
	code, resp := env.do(t, http.MethodPut, "/api/admin/maintenance", admin.Token,
		maintenanceInput{Enabled: true, Title: strings.Repeat("标", maintenanceTitleMaxRune+1)})
	if code != http.StatusBadRequest || resp.Error == nil || resp.Error.Code != "TITLE_TOO_LONG" {
		t.Fatalf("超长标题应 400 TITLE_TOO_LONG，实际 %d：%s", code, describe(resp))
	}

	// 正文超长。
	code, resp = env.do(t, http.MethodPut, "/api/admin/maintenance", admin.Token,
		maintenanceInput{Enabled: true, Title: "维护", Body: strings.Repeat("文", maintenanceBodyMaxRune+1)})
	if code != http.StatusBadRequest || resp.Error == nil || resp.Error.Code != "BODY_TOO_LONG" {
		t.Fatalf("超长正文应 400 BODY_TOO_LONG，实际 %d：%s", code, describe(resp))
	}

	// 默认关闭，标题用默认文案。
	code, resp = env.do(t, http.MethodGet, "/api/admin/maintenance", admin.Token, nil)
	if code != http.StatusOK {
		t.Fatalf("读取维护配置应 200，实际 %d：%s", code, describe(resp))
	}
	if state := decode[MaintenanceState](t, resp); state.Enabled || state.Title != maintenanceDefaultTitle {
		t.Fatalf("默认维护配置异常：%+v", state)
	}
}
