package main

import (
	"crypto/md5"
	"encoding/hex"
	"errors"
	"net/mail"
	"strings"
)

var (
	errInvalidEmail = errors.New("邮箱格式无效")
	errEmailTooLong = errors.New("邮箱过长")
)

// gravatarHash 按 Gravatar 规范对邮箱取 MD5：去空格、转小写。
// 这个哈希就是 gravatar.com 的公开标识，不是安全用途。
func gravatarHash(email string) string {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return ""
	}

	sum := md5.Sum([]byte(email))
	return hex.EncodeToString(sum[:])
}

// normalizeEmail 校验并规范化邮箱，空字符串表示清除（不使用 Gravatar）。
func normalizeEmail(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	if len(raw) > 254 {
		return "", errEmailTooLong
	}

	// 只接受纯邮箱地址，拒绝 "Name <a@b.com>" 这种带显示名的形式
	addr, err := mail.ParseAddress(raw)
	if err != nil || addr.Address != raw {
		return "", errInvalidEmail
	}

	return strings.ToLower(addr.Address), nil
}

// fillPostGravatars 批量为文章补上作者头像哈希，避免逐条查询用户。
// 头像只是展示信息，查询失败时静默跳过，不影响文章列表。
func (d *Data) fillPostGravatars(posts []Post) {
	names := make([]string, 0, len(posts))
	seen := make(map[string]struct{}, len(posts))

	for _, post := range posts {
		if post.User == "" {
			continue
		}
		if _, ok := seen[post.User]; ok {
			continue
		}
		seen[post.User] = struct{}{}
		names = append(names, post.User)
	}

	if len(names) == 0 {
		return
	}

	var users []User
	if err := d.DB.Select("name", "email").Where("name IN ?", names).Find(&users).Error; err != nil {
		return
	}

	hashes := make(map[string]string, len(users))
	for _, user := range users {
		hashes[user.Name] = gravatarHash(user.Email)
	}

	for i := range posts {
		posts[i].UserGravatar = hashes[posts[i].User]
	}
}
