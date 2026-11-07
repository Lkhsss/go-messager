package main

import (
	"net/http"
	"testing"
	"time"
)

func TestUptimeReturnsStartedAt(t *testing.T) {
	env := newTestEnv(t)

	startedAt := time.Now().Add(-time.Hour)
	previous := startTime
	startTime = startedAt
	t.Cleanup(func() {
		startTime = previous
	})

	code, resp := env.do(t, http.MethodGet, "/api/uptime", "", nil)
	if code != http.StatusOK {
		t.Fatalf("运行时长应 200，实际 %d：%s", code, describe(resp))
	}

	payload := decode[struct {
		UptimeSeconds int64      `json:"uptime_seconds"`
		StartedAt     *time.Time `json:"started_at"`
	}](t, resp)
	if payload.UptimeSeconds < 3599 {
		t.Fatalf("运行时长应约为 3600 秒，实际 %d", payload.UptimeSeconds)
	}
	if payload.StartedAt == nil || payload.StartedAt.Unix() != startedAt.Unix() {
		t.Fatalf("started_at 异常：%v，期望 %v", payload.StartedAt, startedAt)
	}
}
