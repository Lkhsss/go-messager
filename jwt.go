package main

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const jwtIssuer = "go-messager"

var errInvalidToken = errors.New("无效的令牌")

// Claims 是 token 里携带的信息。
// Subject 存用户 ID（改名不影响身份），Name/Role 只是方便前端展示；
// 真正的权限判断以数据库为准，见 AuthRequired。
type Claims struct {
	Name string `json:"name"`
	Role Role   `json:"role"`
	jwt.RegisteredClaims
}

// issueToken 为用户签发 HS256 token，同时返回过期时间供前端保存。
func (d *Data) issueToken(user User) (string, time.Time, error) {
	now := time.Now()
	expiresAt := now.Add(d.JWTTTL)

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

	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(d.JWTSecret)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("签发令牌失败：%w", err)
	}

	return token, expiresAt, nil
}

// parseToken 校验签名、签发者与有效期。
// 限定 HS256，避免 alg 混淆攻击（例如伪造 alg=none）。
func (d *Data) parseToken(raw string) (*Claims, error) {
	claims := &Claims{}
	_, err := jwt.ParseWithClaims(raw, claims, func(*jwt.Token) (any, error) {
		return d.JWTSecret, nil
	},
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(jwtIssuer),
		jwt.WithExpirationRequired(),
	)
	if err != nil {
		return nil, err
	}

	if claims.Subject == "" {
		return nil, errInvalidToken
	}

	return claims, nil
}

// userIDFromClaims 把 subject 解析成用户 ID。
func userIDFromClaims(claims *Claims) (uint, error) {
	id, err := strconv.ParseUint(claims.Subject, 10, 64)
	if err != nil {
		return 0, errInvalidToken
	}

	return uint(id), nil
}

// bearerToken 从 Authorization 头中取出 Bearer token，大小写不敏感。
func bearerToken(header string) (string, bool) {
	const prefix = "bearer "

	header = strings.TrimSpace(header)
	if len(header) <= len(prefix) || !strings.EqualFold(header[:len(prefix)], prefix) {
		return "", false
	}

	token := strings.TrimSpace(header[len(prefix):])
	if token == "" {
		return "", false
	}

	return token, true
}
