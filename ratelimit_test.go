package main

import (
	"testing"
	"time"
)

func TestRateLimiterAllowAndWindow(t *testing.T) {
	limiter := newRateLimiter(time.Minute)
	now := time.Now()

	for i := 0; i < 3; i++ {
		if !limiter.allowAt(now, "key", 3) {
			t.Fatalf("第 %d 次请求应放行", i+1)
		}
	}
	if limiter.allowAt(now, "key", 3) {
		t.Fatal("超过上限应拒绝")
	}
	if !limiter.allowAt(now, "other", 3) {
		t.Fatal("不同 key 应独立计数")
	}
	if !limiter.allowAt(now.Add(time.Minute), "key", 3) {
		t.Fatal("窗口过期后应重新放行")
	}
}

func TestRateLimiterExceededDoesNotIncrement(t *testing.T) {
	limiter := newRateLimiter(time.Minute)
	now := time.Now()

	limiter.allowAt(now, "key", 2)
	if limiter.exceededAt(now, "key", 2) {
		t.Fatal("未达上限不应判定为超限")
	}

	limiter.allowAt(now, "key", 2)
	if !limiter.exceededAt(now, "key", 2) {
		t.Fatal("达到上限应判定为超限")
	}
	if limiter.exceededAt(now.Add(time.Minute), "key", 2) {
		t.Fatal("窗口过期后应解除超限")
	}
}

func TestRateLimiterReset(t *testing.T) {
	limiter := newRateLimiter(time.Minute)
	now := time.Now()

	limiter.allowAt(now, "key", 1)
	if limiter.allowAt(now, "key", 1) {
		t.Fatal("达到上限应拒绝")
	}

	limiter.Reset("key")
	if !limiter.allowAt(now, "key", 1) {
		t.Fatal("Reset 后应重新放行")
	}
}

func TestRateLimiterInvalidLimit(t *testing.T) {
	limiter := newRateLimiter(time.Minute)
	if limiter.Allow("key", 0) {
		t.Fatal("limit<=0 时应拒绝")
	}
	if !limiter.Exceeded("key", 0) {
		t.Fatal("limit<=0 时应判定为超限")
	}
}
