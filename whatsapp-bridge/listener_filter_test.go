package main

import (
	"testing"
	"time"
)

func wl(jids ...string) map[string]bool {
	m := map[string]bool{}
	for _, j := range jids {
		m[j] = true
	}
	return m
}

func TestPassesPrefilter(t *testing.T) {
	now := time.Date(2026, 6, 11, 12, 0, 0, 0, time.UTC)
	whitelist := wl("trip@g.us")
	cases := []struct {
		name string
		msg  LiveMsg
		want bool
	}{
		{"ok", LiveMsg{ChatJID: "trip@g.us", Sender: "alice", Content: "hi", Timestamp: now.Add(-10 * time.Second)}, true},
		{"not whitelisted", LiveMsg{ChatJID: "other@g.us", Content: "hi", Timestamp: now}, false},
		{"from me", LiveMsg{ChatJID: "trip@g.us", Content: "hi", Timestamp: now, IsFromMe: true}, false},
		{"stale (reconnect replay)", LiveMsg{ChatJID: "trip@g.us", Content: "hi", Timestamp: now.Add(-3 * time.Minute)}, false},
		{"future clock skew ok", LiveMsg{ChatJID: "trip@g.us", Content: "hi", Timestamp: now.Add(5 * time.Second)}, true},
	}
	for _, tc := range cases {
		got, reason := PassesPrefilter(whitelist, tc.msg, now, 2*time.Minute)
		if got != tc.want {
			t.Errorf("%s: got %v (%s), want %v", tc.name, got, reason, tc.want)
		}
	}
}

func TestInQuietHours(t *testing.T) {
	mk := func(h, m int) time.Time { return time.Date(2026, 6, 11, h, m, 0, 0, time.Local) }
	cases := []struct {
		spec string
		at   time.Time
		want bool
	}{
		{"", mk(3, 0), false},
		{"23:00-08:00", mk(23, 30), true}, // wraps midnight
		{"23:00-08:00", mk(7, 59), true},
		{"23:00-08:00", mk(8, 0), false},
		{"23:00-08:00", mk(12, 0), false},
		{"13:00-14:00", mk(13, 30), true}, // same-day window
		{"13:00-14:00", mk(14, 30), false},
	}
	for _, tc := range cases {
		if got := InQuietHours(tc.spec, tc.at); got != tc.want {
			t.Errorf("spec %q at %v: got %v want %v", tc.spec, tc.at, got, tc.want)
		}
	}
}

func TestParseQuietHoursRejectsGarbage(t *testing.T) {
	for _, bad := range []string{"23:00", "25:00-08:00", "aa:bb-cc:dd", "23:00-"} {
		if _, _, err := parseQuietHours(bad); err == nil {
			t.Errorf("%q should error", bad)
		}
	}
}
