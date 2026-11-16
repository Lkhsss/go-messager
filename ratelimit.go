package main

import (
	"sync"
	"time"
)

// 限流器容量与清理间隔：攻击者用大量 IP/账号时不能让 map 无限增长，
// 也不能每次请求都遍历整个 map。
const (
	rateLimiterMaxEntries    = 100000
	rateLimiterSweepInterval = time.Second
)

// rateLimiter 固定窗口限流器，按 key 独立计数；过期窗口惰性清理。
type rateLimiter struct {
	mu        sync.Mutex
	window    time.Duration
	entries   map[string]rateWindowState
	lastSweep time.Time
}

// newRateLimiter 创建限流器，窗口非法时回退 1 分钟。
func newRateLimiter(window time.Duration) *rateLimiter {
	if window <= 0 {
		window = time.Minute
	}

	return &rateLimiter{
		window:  window,
		entries: make(map[string]rateWindowState),
	}
}

// Allow 在窗口内自增计数，超过上限返回 false。
func (l *rateLimiter) Allow(key string, limit int) bool {
	return l.allowAt(time.Now(), key, limit)
}

func (l *rateLimiter) allowAt(now time.Time, key string, limit int) bool {
	if limit <= 0 {
		return false
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	l.sweepLocked(now)

	entry, ok := l.entries[key]
	if !ok || now.Sub(entry.start) >= l.window {
		l.entries[key] = rateWindowState{start: now, count: 1}
		return true
	}
	if entry.count >= limit {
		return false
	}

	entry.count++
	l.entries[key] = entry

	return true
}

// Exceeded 判断 key 是否已达上限，不自增。
func (l *rateLimiter) Exceeded(key string, limit int) bool {
	return l.exceededAt(time.Now(), key, limit)
}

func (l *rateLimiter) exceededAt(now time.Time, key string, limit int) bool {
	if limit <= 0 {
		return true
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	entry, ok := l.entries[key]
	if !ok || now.Sub(entry.start) >= l.window {
		return false
	}

	return entry.count >= limit
}

// Reset 清除 key 的计数，登录成功后调用。
func (l *rateLimiter) Reset(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	delete(l.entries, key)
}

// sweepLocked 清理过期窗口，避免长期运行内存只增不减。
// 正常情况最多每秒遍历一次；达到容量上限时只淘汰一个条目，避免 CPU 被拖垮。
func (l *rateLimiter) sweepLocked(now time.Time) {
	if !l.lastSweep.IsZero() && now.Sub(l.lastSweep) < rateLimiterSweepInterval {
		if len(l.entries) >= rateLimiterMaxEntries {
			l.evictOneLocked()
		}
		return
	}

	for key, entry := range l.entries {
		if now.Sub(entry.start) >= l.window {
			delete(l.entries, key)
		}
	}
	l.lastSweep = now

	for len(l.entries) >= rateLimiterMaxEntries {
		l.evictOneLocked()
	}
}

// evictOneLocked 在容量打满时淘汰一个任意条目。
func (l *rateLimiter) evictOneLocked() {
	for key := range l.entries {
		delete(l.entries, key)
		return
	}
}
