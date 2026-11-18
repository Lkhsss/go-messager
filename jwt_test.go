package main

import (
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// signTestToken 用测试环境的密钥签发指定到期时间的 token。
func signTestToken(t *testing.T, env *testEnv, user User, expiresAt time.Time) string {
	t.Helper()

	now := time.Now()
	claims := Claims{
		Name: user.Name,
		Role: user.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    jwtIssuer,
			Subject:   strconv.FormatUint(uint64(user.ID), 10),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
	}

	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(env.data.JWTSecret)
	if err != nil {
		t.Fatalf("签发测试令牌失败：%v", err)
	}
	return token
}

func TestRenewTokenWhenExpiring(t *testing.T) {
	// 避免外部 JWT_TTL 环境变量覆盖测试配置。
	t.Setenv("JWT_TTL", "")

	env := newTestEnvWithConfig(t, func(cfg *Config) {
		cfg.JWT.TTL = 2 * time.Minute
	})
	alice := env.register(t, "alice", "alice123")

	// 新签发的 token 还有约 2 分钟，不在续期窗口（30s）内。
	code, headers, resp := env.doRecorder(t, http.MethodGet, "/api/me", alice.Token, nil)
	if code != http.StatusOK {
		t.Fatalf("新 token 访问 /api/me 应 200，实际 %d：%s", code, describe(resp))
	}
	if renewed := headers.Get(headerAuthToken); renewed != "" {
		t.Fatalf("有效期充足时不应续期，实际下发[%s]", renewed)
	}

	user, err := env.data.GetUserByName("alice")
	if err != nil {
		t.Fatalf("查询用户失败：%v", err)
	}

	// 只剩 10 秒，进入续期窗口。
	expiring := signTestToken(t, env, user, time.Now().Add(10*time.Second))
	code, headers, resp = env.doRecorder(t, http.MethodGet, "/api/me", expiring, nil)
	if code != http.StatusOK {
		t.Fatalf("临期 token 访问 /api/me 应 200，实际 %d：%s", code, describe(resp))
	}

	renewed := headers.Get(headerAuthToken)
	if renewed == "" {
		t.Fatal("临期 token 应通过 X-Auth-Token 下发续期令牌")
	}
	if expiresAt := headers.Get(headerAuthExpiresAt); expiresAt == "" {
		t.Fatal("续期时应同时下发 X-Auth-Expires-At")
	}

	// 续期后的 token 应能继续访问。
	code, _, resp = env.doRecorder(t, http.MethodGet, "/api/me", renewed, nil)
	if code != http.StatusOK {
		t.Fatalf("续期令牌应可继续访问，实际 %d：%s", code, describe(resp))
	}
}

func TestRenewWindow(t *testing.T) {
	cases := []struct {
		name string
		ttl  time.Duration
		want time.Duration
	}{
		{name: "默认 24h 取四分之一", ttl: 24 * time.Hour, want: 6 * time.Hour},
		{name: "长 TTL 上限 12h", ttl: 168 * time.Hour, want: 12 * time.Hour},
		{name: "短 TTL 保底 30s", ttl: time.Minute, want: 30 * time.Second},
		{name: "不超过 TTL 一半", ttl: 40 * time.Second, want: 20 * time.Second},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			data := &Data{JWTTTL: tc.ttl}
			if got := data.renewWindow(); got != tc.want {
				t.Fatalf("renewWindow(%s) = %s，期望 %s", tc.ttl, got, tc.want)
			}
		})
	}
}
