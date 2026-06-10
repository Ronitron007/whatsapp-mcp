package main

import (
	"testing"
	"time"
)

func TestLimiterPerMinuteWindow(t *testing.T) {
	now := time.Date(2026, 6, 11, 12, 0, 0, 0, time.UTC)
	l := NewLimiter(99, 99, 2, 99) // 2/min global, other caps effectively off
	if ok, _ := l.Allow("a", now); !ok {
		t.Fatal("first must pass")
	}
	l.RecordBotReply("a", now)
	if ok, _ := l.Allow("b", now.Add(10*time.Second)); !ok {
		t.Fatal("second within the minute must pass")
	}
	l.RecordBotReply("b", now.Add(10*time.Second))
	// across chats: 2 already sent this minute -> 3rd blocked regardless of chat
	if ok, reason := l.Allow("c", now.Add(20*time.Second)); ok {
		t.Fatal("third in same minute must block (global 2/min)")
	} else if reason == "" {
		t.Fatal("blocked Allow must give a reason")
	}
	// minute window slides -> allowed again
	if ok, _ := l.Allow("c", now.Add(61*time.Second)); !ok {
		t.Fatal("after the minute slides, must allow again")
	}
}

func TestLimiterHourlyWindows(t *testing.T) {
	now := time.Date(2026, 6, 11, 12, 0, 0, 0, time.UTC)
	l := NewLimiter(2, 3, 99, 99) // 2/hr/chat, 3/hr global, per-min + consecutive off
	if ok, _ := l.Allow("a", now); !ok {
		t.Fatal("first must pass")
	}
	l.RecordBotReply("a", now)
	l.RecordBotReply("a", now.Add(time.Minute))
	if ok, reason := l.Allow("a", now.Add(2*time.Minute)); ok {
		t.Fatal("per-chat cap must block, got allow")
	} else if reason == "" {
		t.Fatal("blocked Allow must give a reason")
	}
	// other chat still fine until global cap
	if ok, _ := l.Allow("b", now.Add(2*time.Minute)); !ok {
		t.Fatal("chat b must pass (global 2/3 used)")
	}
	l.RecordBotReply("b", now.Add(2*time.Minute))
	if ok, _ := l.Allow("c", now.Add(3*time.Minute)); ok {
		t.Fatal("global cap must block chat c")
	}
	// window slides: an hour later everything resets
	if ok, _ := l.Allow("a", now.Add(62*time.Minute)); !ok {
		t.Fatal("after window slides, must allow again")
	}
}

func TestLimiterConsecutiveCap(t *testing.T) {
	now := time.Date(2026, 6, 11, 12, 0, 0, 0, time.UTC)
	l := NewLimiter(99, 99, 99, 2)
	l.RecordBotReply("a", now)
	l.RecordBotReply("a", now.Add(time.Second))
	if ok, _ := l.Allow("a", now.Add(2*time.Second)); ok {
		t.Fatal("2 consecutive bot replies must block the 3rd")
	}
	l.RecordHuman("a") // a human spoke
	if ok, _ := l.Allow("a", now.Add(3*time.Second)); !ok {
		t.Fatal("human message must reset consecutive counter")
	}
	// other chats unaffected
	if ok, _ := l.Allow("b", now); !ok {
		t.Fatal("chat b unaffected by chat a counter")
	}
}
