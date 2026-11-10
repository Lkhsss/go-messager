package main

// 等级经验：第 1 级升第 2 级需要 100 经验，之后每级所需经验按等差数列递增 100。
// 即 100 / 200 / 300 / 400…，达到 L 级所需累计经验为 50*L*(L-1)。
const (
	levelBaseXP int64 = 100
	levelStepXP int64 = 100
)

// levelRequirement 返回第 level 级升到下一级所需的经验。
func levelRequirement(level int) int64 {
	if level < 1 {
		level = 1
	}

	return levelBaseXP + int64(level-1)*levelStepXP
}

// levelFromExp 根据累计经验计算等级、本级起点经验和下一级门槛（均为累计值）。
func levelFromExp(exp int64) (level int, levelStartExp, nextLevelExp int64) {
	if exp < 0 {
		exp = 0
	}

	level = 1
	levelStartExp = 0
	nextLevelExp = levelRequirement(level)
	for exp >= nextLevelExp {
		level++
		levelStartExp = nextLevelExp
		nextLevelExp += levelRequirement(level)
	}

	return level, levelStartExp, nextLevelExp
}
