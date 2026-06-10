package main

import (
	"fmt"
	"sync"
	"time"
)

// Limiter enforces hourly reply caps and the consecutive-bot-reply cap.
// In-memory only: restart resets counters (accepted in spec).
type Limiter struct {
	mu           sync.Mutex
	perChat      int
	global       int
	globalPerMin int
	maxConsec    int
	chatTimes    map[string][]time.Time
	globalTimes  []time.Time
	consecutive  map[string]int
}

func NewLimiter(perChatPerHour, globalPerHour, globalPerMinute, maxConsecutive int) *Limiter {
	return &Limiter{
		perChat:      perChatPerHour,
		global:       globalPerHour,
		globalPerMin: globalPerMinute,
		maxConsec:    maxConsecutive,
		chatTimes:    map[string][]time.Time{},
		consecutive:  map[string]int{},
	}
}

func pruneOld(ts []time.Time, cutoff time.Time) []time.Time {
	out := ts[:0]
	for _, t := range ts {
		if t.After(cutoff) {
			out = append(out, t)
		}
	}
	return out
}

// Allow reports whether a bot reply in chat is currently permitted.
func (l *Limiter) Allow(chat string, now time.Time) (bool, string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	cutoff := now.Add(-time.Hour)
	l.chatTimes[chat] = pruneOld(l.chatTimes[chat], cutoff)
	l.globalTimes = pruneOld(l.globalTimes, cutoff)

	if l.globalPerMin > 0 {
		minuteCutoff := now.Add(-time.Minute)
		recent := 0
		for _, t := range l.globalTimes {
			if t.After(minuteCutoff) {
				recent++
			}
		}
		if recent >= l.globalPerMin {
			return false, fmt.Sprintf("global cap %d/min reached", l.globalPerMin)
		}
	}
	if len(l.chatTimes[chat]) >= l.perChat {
		return false, fmt.Sprintf("per-chat cap %d/hr reached", l.perChat)
	}
	if len(l.globalTimes) >= l.global {
		return false, fmt.Sprintf("global cap %d/hr reached", l.global)
	}
	if l.consecutive[chat] >= l.maxConsec {
		return false, fmt.Sprintf("%d consecutive bot replies, waiting for a human", l.consecutive[chat])
	}
	return true, ""
}

// RecordBotReply registers a sent (or dry-run-logged) reply.
func (l *Limiter) RecordBotReply(chat string, now time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.chatTimes[chat] = append(l.chatTimes[chat], now)
	l.globalTimes = append(l.globalTimes, now)
	l.consecutive[chat]++
}

// RecordHuman resets the consecutive counter when a human speaks in chat.
func (l *Limiter) RecordHuman(chat string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.consecutive[chat] = 0
}
