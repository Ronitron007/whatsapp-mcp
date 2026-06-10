package main

import (
	"sync"
	"time"
)

// Debouncer fires once per chat after d of quiet. Each Schedule for the
// same chat resets that chat's timer, so a message burst yields one fire.
type Debouncer struct {
	mu      sync.Mutex
	d       time.Duration
	fire    func(chatJID string)
	timers  map[string]*time.Timer
	stopped bool
}

func NewDebouncer(d time.Duration, fire func(chatJID string)) *Debouncer {
	return &Debouncer{d: d, fire: fire, timers: map[string]*time.Timer{}}
}

// Schedule (re)arms the timer for chatJID.
func (db *Debouncer) Schedule(chatJID string) {
	db.mu.Lock()
	defer db.mu.Unlock()
	if db.stopped {
		return
	}
	if t, ok := db.timers[chatJID]; ok {
		t.Stop()
	}
	db.timers[chatJID] = time.AfterFunc(db.d, func() {
		db.mu.Lock()
		if db.stopped {
			db.mu.Unlock()
			return
		}
		delete(db.timers, chatJID)
		db.mu.Unlock()
		db.fire(chatJID)
	})
}

// Stop cancels all pending timers; the Debouncer is unusable afterwards.
func (db *Debouncer) Stop() {
	db.mu.Lock()
	defer db.mu.Unlock()
	db.stopped = true
	for _, t := range db.timers {
		t.Stop()
	}
	db.timers = map[string]*time.Timer{}
}
