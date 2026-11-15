package service

import (
	"testing"
	"time"
)

func TestLockedMessageRoundsUpToMinutes(t *testing.T) {
	cases := []struct {
		remaining time.Duration
		want      string
	}{
		{15 * time.Minute, "账号已锁定，请 15 分钟后再试"},
		{14*time.Minute + time.Second, "账号已锁定，请 15 分钟后再试"},
		{10 * time.Second, "账号已锁定，请 1 分钟后再试"},
		{0, "账号已锁定，请 1 分钟后再试"},
		{-time.Second, "账号已锁定，请 1 分钟后再试"},
	}
	for _, c := range cases {
		if got := lockedMessage(c.remaining); got != c.want {
			t.Errorf("lockedMessage(%v) = %q, want %q", c.remaining, got, c.want)
		}
	}
}
