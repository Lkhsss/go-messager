package main

import (
	"regexp"
	"strings"
	"unicode"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	// 标签名长度上限，与 Tag.Name 的校验保持一致。
	maxAutoTagRunes = maxTagNameLength
	// 自动标签分批写入大小，避免单条 SQL 变量过多。
	autoTagInsertBatch = 100
	// 单篇自动标签安全护栏：仅防止异常正文构造数千标签拖垮 SQLite，正常写作不会触及。
	maxAutoTagsPerPost = 500
)

var (
	// 行内代码先移除，避免把代码里的 # 当成标签。
	inlineCodePattern = regexp.MustCompile("`[^`\\n]*`")
	// 标签链：#TEST#自动标签#ZZZ。只有链起点做前置字符检查，链内的 # 直接作为分隔符。
	// 前端 markdown.ts 的 tag_pill 规则与此保持一致。
	bodyTagChainPattern = regexp.MustCompile(`(?:^|[^\p{L}\p{N}_#/&?=.:-])((?:#[\p{L}\p{N}_-]{1,30})+)`)
	// 拆分链内的单个标签。
	bodyTagSegmentPattern = regexp.MustCompile(`#([\p{L}\p{N}_-]{1,30})`)
)

// extractBodyTags 提取正文中的 #标签，按首次出现顺序去重。
// 代码块、行内代码、图片和链接整段忽略，与前端 markdown-it 的行为一致。
func extractBodyTags(body string) []string {
	text := markdownFencePattern.ReplaceAllString(body, " ")
	text = markdownImagePattern.ReplaceAllString(text, " ")
	text = markdownLinkPattern.ReplaceAllString(text, " ")
	text = inlineCodePattern.ReplaceAllString(text, " ")

	chains := bodyTagChainPattern.FindAllStringSubmatch(text, -1)
	names := make([]string, 0, len(chains))
	seen := make(map[string]struct{}, len(chains))

	for _, chain := range chains {
		if len(chain) < 2 {
			continue
		}
		// 一条链可能包含多个标签：逐个拆分、归一化、去重。
		for _, segment := range bodyTagSegmentPattern.FindAllStringSubmatch(chain[1], -1) {
			if len(segment) < 2 {
				continue
			}
			name := normalizeAutoTagName(segment[1])
			if name == "" {
				continue
			}
			if _, ok := seen[name]; ok {
				continue
			}
			seen[name] = struct{}{}
			names = append(names, name)
			if len(names) >= maxAutoTagsPerPost {
				return names
			}
		}
	}

	return names
}

// normalizeAutoTagName 去掉首尾空白和末尾的 -_，并要求至少含一个字母或数字。
func normalizeAutoTagName(raw string) string {
	name := strings.TrimRight(strings.TrimSpace(raw), "-_")
	if name == "" || len([]rune(name)) > maxAutoTagRunes {
		return ""
	}
	for _, r := range name {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			return name
		}
	}
	return ""
}

// resolveBodyTags 确保正文标签存在，自动创建的新标签为待审核状态。
func (d *Data) resolveBodyTags(tx *gorm.DB, names []string) ([]Tag, error) {
	if len(names) == 0 {
		return nil, nil
	}

	existing, err := loadTagsByNames(tx, names)
	if err != nil {
		return nil, err
	}

	missing := make([]Tag, 0, len(names))
	for _, name := range names {
		if _, ok := existing[name]; ok {
			continue
		}
		missing = append(missing, Tag{Name: name})
	}

	// 分批插入；唯一索引冲突时保留已有记录。
	for start := 0; start < len(missing); start += autoTagInsertBatch {
		end := start + autoTagInsertBatch
		if end > len(missing) {
			end = len(missing)
		}
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(missing[start:end]).Error; err != nil {
			return nil, err
		}
	}

	// 重新查询，补齐并发插入或冲突跳过的记录。
	existing, err = loadTagsByNames(tx, names)
	if err != nil {
		return nil, err
	}

	result := make([]Tag, 0, len(names))
	for _, name := range names {
		if tag, ok := existing[name]; ok {
			result = append(result, tag)
		}
	}
	return result, nil
}

// loadTagsByNames 按名称查询标签，返回 name -> Tag。
func loadTagsByNames(tx *gorm.DB, names []string) (map[string]Tag, error) {
	tags := make([]Tag, 0, len(names))
	if err := tx.Where("name IN ?", names).Find(&tags).Error; err != nil {
		return nil, err
	}

	byName := make(map[string]Tag, len(tags))
	for _, tag := range tags {
		byName[tag.Name] = tag
	}
	return byName, nil
}

// mergeTags 合并手选与自动标签，按 ID 去重，手选在前。
func mergeTags(manual, auto []Tag) []Tag {
	result := make([]Tag, 0, len(manual)+len(auto))
	seen := make(map[uint]struct{}, len(manual)+len(auto))

	for _, list := range [][]Tag{manual, auto} {
		for _, tag := range list {
			if _, ok := seen[tag.ID]; ok {
				continue
			}
			seen[tag.ID] = struct{}{}
			result = append(result, tag)
		}
	}
	return result
}

// filterReviewedTags 只保留已审核标签，供对外响应使用。
func filterReviewedTags(tags []Tag) []Tag {
	result := make([]Tag, 0, len(tags))
	for _, tag := range tags {
		if tag.Reviewed {
			result = append(result, tag)
		}
	}
	return result
}

// tagIDsOf 提取标签 ID。
func tagIDsOf(tags []Tag) []uint {
	ids := make([]uint, 0, len(tags))
	for _, tag := range tags {
		if tag.ID != 0 {
			ids = append(ids, tag.ID)
		}
	}
	return ids
}
