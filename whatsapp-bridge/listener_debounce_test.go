package main

import (
	"sync"
	"testing"
	"time"
)

func TestDebouncerCollapsesBursts(t *testing.T) {
	var mu sync.Mutex
	fired := map[string]int{}
	d := NewDebouncer(40*time.Millisecond, func(chat string) {
		mu.Lock()
		fired[chat]++
		mu.Unlock()
	})
	defer d.Stop()

	// burst of 5 in one chat -> exactly one fire
	for i := 0; i < 5; i++ {
		d.Schedule("a")
		time.Sleep(5 * time.Millisecond)
	}
	// parallel chat is independent
	d.Schedule("b")

	time.Sleep(120 * time.Millisecond)
	mu.Lock()
	defer mu.Unlock()
	if fired["a"] != 1 {
		t.Errorf("chat a: want 1 fire, got %d", fired["a"])
	}
	if fired["b"] != 1 {
		t.Errorf("chat b: want 1 fire, got %d", fired["b"])
	}
}

func TestDebouncerResetExtendsWindow(t *testing.T) {
	var mu sync.Mutex
	count := 0
	d := NewDebouncer(50*time.Millisecond, func(string) {
		mu.Lock()
		count++
		mu.Unlock()
	})
	defer d.Stop()

	d.Schedule("a")
	time.Sleep(30 * time.Millisecond) // before expiry
	d.Schedule("a")                   // resets timer
	time.Sleep(30 * time.Millisecond) // 60ms after first, 30ms after reset
	mu.Lock()
	if count != 0 {
		t.Errorf("must not fire yet (timer was reset), got %d", count)
	}
	mu.Unlock()
	time.Sleep(40 * time.Millisecond)
	mu.Lock()
	defer mu.Unlock()
	if count != 1 {
		t.Errorf("want exactly 1 fire after reset window, got %d", count)
	}
}

func TestDebouncerStopPreventsFires(t *testing.T) {
	var mu sync.Mutex
	count := 0
	d := NewDebouncer(30*time.Millisecond, func(string) {
		mu.Lock()
		count++
		mu.Unlock()
	})
	d.Schedule("a")
	d.Stop()
	time.Sleep(60 * time.Millisecond)
	mu.Lock()
	defer mu.Unlock()
	if count != 0 {
		t.Errorf("stopped debouncer must not fire, got %d", count)
	}
}
