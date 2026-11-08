package main

import (
	"bytes"
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"regexp"
	"strings"
)

const (
	// 图片字节上限
	maxAvatarBytes = 300 * 1024
	// 单边像素上限
	maxAvatarSide = 512
	// data URL 长度上限
	maxAvatarBase64 = 420 * 1024
)

var (
	errInvalidAvatar     = errors.New("头像数据无效")
	errUnsupportedAvatar = errors.New("只支持 JPEG/PNG 头像")
	errAvatarTooLarge    = errors.New("头像超出大小限制")
)

var avatarDataURLPattern = regexp.MustCompile(`^data:image/(jpeg|png);base64,`)

// normalizeAvatar 校验 data URL，只接受 JPEG/PNG。
func normalizeAvatar(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", errInvalidAvatar
	}
	if len(raw) > maxAvatarBase64 {
		return "", errAvatarTooLarge
	}
	if !avatarDataURLPattern.MatchString(raw) {
		return "", errUnsupportedAvatar
	}

	_, data, err := decodeAvatarDataURL(raw)
	if err != nil {
		return "", err
	}
	if len(data) > maxAvatarBytes {
		return "", errAvatarTooLarge
	}

	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return "", errInvalidAvatar
	}
	if cfg.Width <= 0 || cfg.Height <= 0 || cfg.Width > maxAvatarSide || cfg.Height > maxAvatarSide {
		return "", errAvatarTooLarge
	}

	return raw, nil
}

func decodeAvatarDataURL(raw string) (mime string, data []byte, err error) {
	if !avatarDataURLPattern.MatchString(raw) {
		return "", nil, errUnsupportedAvatar
	}

	comma := strings.IndexByte(raw, ',')
	if comma < 0 {
		return "", nil, errInvalidAvatar
	}

	mime = strings.TrimSuffix(raw[len("data:"):comma], ";base64")
	data, err = base64.StdEncoding.DecodeString(raw[comma+1:])
	if err != nil {
		return "", nil, errInvalidAvatar
	}

	return mime, data, nil
}

// avatarVersion 头像内容哈希，用于缓存版本。
func avatarVersion(raw string) string {
	if strings.TrimSpace(raw) == "" {
		return ""
	}

	sum := md5.Sum([]byte(raw))
	return hex.EncodeToString(sum[:])[:12]
}

// fillPostAvatars 批量补作者头像版本。
func (d *Data) fillPostAvatars(posts []Post) {
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
	if err := d.DB.Select("name", "avatar").Where("name IN ?", names).Find(&users).Error; err != nil {
		return // 头像只是展示信息，查询失败不影响文章
	}

	versions := make(map[string]string, len(users))
	for _, user := range users {
		versions[user.Name] = avatarVersion(user.Avatar)
	}

	for i := range posts {
		posts[i].UserAvatarVersion = versions[posts[i].User]
	}
}
