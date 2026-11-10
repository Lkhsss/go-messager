package main

import "testing"

func TestLevelRequirementArithmetic(t *testing.T) {
	// 第一级 100，之后按公差 100 递增。
	for level := 1; level <= 10; level++ {
		want := int64(100 * level)
		if got := levelRequirement(level); got != want {
			t.Fatalf("第 %d 级升级所需经验应为 %d，实际 %d", level, want, got)
		}
	}
}

func TestLevelFromExp(t *testing.T) {
	cases := []struct {
		exp   int64
		level int
		start int64
		next  int64
	}{
		{0, 1, 0, 100},
		{99, 1, 0, 100},
		{100, 2, 100, 300},
		{299, 2, 100, 300},
		{300, 3, 300, 600},
		{599, 3, 300, 600},
		{600, 4, 600, 1000},
		{1000, 5, 1000, 1500},
		{-10, 1, 0, 100},
	}

	for _, tc := range cases {
		level, start, next := levelFromExp(tc.exp)
		if level != tc.level || start != tc.start || next != tc.next {
			t.Fatalf("经验 %d 应得到 level=%d start=%d next=%d，实际 level=%d start=%d next=%d",
				tc.exp, tc.level, tc.start, tc.next, level, start, next)
		}
	}
}
