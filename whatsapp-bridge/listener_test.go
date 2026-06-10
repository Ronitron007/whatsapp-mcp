package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

type fakeInvoker struct {
	mu      sync.Mutex
	reply   string
	err     error
	prompts []string
}

func (f *fakeInvoker) Invoke(_ context.Context, prompt string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.prompts = append(f.prompts, prompt)
	return f.reply, f.err
}

type fakeSender struct {
	mu    sync.Mutex
	sends []string // "chat|text"
}

func (f *fakeSender) send(chat, text string) (bool, string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sends = append(f.sends, chat+"|"+text)
	return true, "sent"
}

func (f *fakeSender) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.sends)
}

// newTestListener gives a Listener wired to a real sqlite store in a temp
// dir (MessageStore uses the relative path store/messages.db, so chdir).
func newTestListener(t *testing.T, cfg ListenerConfig, inv ClaudeInvoker, snd *fakeSender) (*Listener, *MessageStore) {
	t.Helper()
	t.Chdir(t.TempDir())
	os.MkdirAll("configs", 0755)
	if cfg.PersonaPath == "configs/persona.md" {
		os.WriteFile("configs/persona.md", []byte("you are the trip bot"), 0644)
	}
	store, err := NewMessageStore()
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	// messages.chat_jid has a FK to chats(jid); production handleMessage
	// always StoreChats before StoreMessage, so tests must too.
	if err := store.StoreChat("trip@g.us", "Trip Planning", time.Now()); err != nil {
		t.Fatalf("store chat: %v", err)
	}
	l := NewListener(cfg, store, snd.send, inv, nil)
	t.Cleanup(l.Stop)
	return l, store
}

func mustStoreMsg(t *testing.T, store *MessageStore, id string, m LiveMsg) {
	t.Helper()
	if err := store.StoreMessage(id, m.ChatJID, m.Sender, m.Content, m.Timestamp, m.IsFromMe, "", "", "", nil, nil, nil, 0); err != nil {
		t.Fatalf("store msg %s: %v", id, err)
	}
}

func testCfg() ListenerConfig {
	cfg := DefaultListenerConfig()
	cfg.Whitelist = []string{"trip@g.us"}
	cfg.DryRun = false
	cfg.DebounceSeconds = 1 // min allowed; tests override the debouncer below
	return cfg
}

func human(content string, ago time.Duration) LiveMsg {
	return LiveMsg{ChatJID: "trip@g.us", Sender: "919876543210",
		Content: content, Timestamp: time.Now().Add(-ago)}
}

func waitFor(t *testing.T, cond func() bool, msg string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal(msg)
}

func TestListenerEndToEndReply(t *testing.T) {
	inv := &fakeInvoker{reply: "5pm works for me!"}
	snd := &fakeSender{}
	cfg := testCfg()
	cfg.SignaturePrefix = "🤖 "
	l, store := newTestListener(t, cfg, inv, snd)
	l.setDebounceForTest(30 * time.Millisecond)

	// store the message first (mirrors handleMessage order), then notify
	m := human("lets meet at 5?", 0)
	mustStoreMsg(t, store, "m1", m)
	l.OnMessage(m)

	waitFor(t, func() bool { return snd.count() == 1 }, "expected exactly one send")
	snd.mu.Lock()
	got := snd.sends[0]
	snd.mu.Unlock()
	if !strings.HasPrefix(got, "trip@g.us|🤖 5pm works for me!") {
		t.Errorf("bad send: %q", got)
	}
	// prompt must contain persona + the human message
	inv.mu.Lock()
	p := inv.prompts[0]
	inv.mu.Unlock()
	if !strings.Contains(p, "trip bot") || !strings.Contains(p, "lets meet at 5?") {
		t.Errorf("prompt missing parts: %q", p)
	}
	// bot reply must be stored for future transcripts. The store write
	// happens after send() on the timer goroutine, so poll rather than
	// asserting immediately.
	waitFor(t, func() bool {
		msgs, _ := store.GetMessages("trip@g.us", 10)
		for _, m := range msgs {
			if m.IsFromMe && strings.Contains(m.Content, "5pm works") {
				return true
			}
		}
		return false
	}, "bot reply not stored in message store")
}

func TestListenerSkipMeansNoSend(t *testing.T) {
	inv := &fakeInvoker{reply: "[SKIP]"}
	snd := &fakeSender{}
	l, store := newTestListener(t, testCfg(), inv, snd)
	l.setDebounceForTest(30 * time.Millisecond)
	m := human("good morning all", 0)
	mustStoreMsg(t, store, "m1", m)
	l.OnMessage(m)
	waitFor(t, func() bool {
		inv.mu.Lock()
		defer inv.mu.Unlock()
		return len(inv.prompts) == 1
	}, "claude must be invoked")
	time.Sleep(50 * time.Millisecond)
	if snd.count() != 0 {
		t.Errorf("[SKIP] must not send, got %d sends", snd.count())
	}
}

func TestListenerDryRunNoSend(t *testing.T) {
	inv := &fakeInvoker{reply: "I would say this"}
	snd := &fakeSender{}
	cfg := testCfg()
	cfg.DryRun = true
	l, store := newTestListener(t, cfg, inv, snd)
	l.setDebounceForTest(30 * time.Millisecond)
	m := human("anyone booked flights?", 0)
	mustStoreMsg(t, store, "m1", m)
	l.OnMessage(m)
	waitFor(t, func() bool {
		inv.mu.Lock()
		defer inv.mu.Unlock()
		return len(inv.prompts) == 1
	}, "claude must be invoked in dry run")
	time.Sleep(50 * time.Millisecond)
	if snd.count() != 0 {
		t.Errorf("dry run must not send, got %d", snd.count())
	}
}

func TestListenerIgnoresNonWhitelistedAndOwn(t *testing.T) {
	inv := &fakeInvoker{reply: "x"}
	snd := &fakeSender{}
	l, _ := newTestListener(t, testCfg(), inv, snd)
	l.setDebounceForTest(20 * time.Millisecond)
	l.OnMessage(LiveMsg{ChatJID: "other@g.us", Sender: "s", Content: "hi", Timestamp: time.Now()})
	l.OnMessage(LiveMsg{ChatJID: "trip@g.us", Sender: "me", Content: "hi", Timestamp: time.Now(), IsFromMe: true})
	time.Sleep(80 * time.Millisecond)
	inv.mu.Lock()
	defer inv.mu.Unlock()
	if len(inv.prompts) != 0 {
		t.Errorf("no invocations expected, got %d", len(inv.prompts))
	}
}

func TestListenerKillSwitchBlocksSend(t *testing.T) {
	inv := &fakeInvoker{reply: "should never go out"}
	snd := &fakeSender{}
	cfg := testCfg()
	l, store := newTestListener(t, cfg, inv, snd)
	l.setDebounceForTest(30 * time.Millisecond)
	os.MkdirAll(filepath.Dir(cfg.KillSwitchPath), 0755)
	os.WriteFile(cfg.KillSwitchPath, []byte{}, 0644) // engage kill switch
	m := human("urgent: reply pls", 0)
	mustStoreMsg(t, store, "m1", m)
	l.OnMessage(m)
	time.Sleep(100 * time.Millisecond)
	if snd.count() != 0 {
		t.Errorf("kill switch must block, got %d sends", snd.count())
	}
}

func TestListenerConsecutiveCapAcrossRounds(t *testing.T) {
	inv := &fakeInvoker{reply: "another bot msg"}
	snd := &fakeSender{}
	cfg := testCfg()
	cfg.MaxConsecutiveBotReplies = 1
	l, store := newTestListener(t, cfg, inv, snd)
	l.setDebounceForTest(20 * time.Millisecond)

	m1 := human("first", 0)
	mustStoreMsg(t, store, "m1", m1)
	l.OnMessage(m1)
	waitFor(t, func() bool { return snd.count() == 1 }, "first reply expected")

	// no human in between -> second round must be blocked by consecutive cap.
	// (an OnMessage with a human msg would reset the counter, so simulate the
	// debounce firing again directly)
	l.fire("trip@g.us")
	time.Sleep(50 * time.Millisecond)
	if snd.count() != 1 {
		t.Errorf("consecutive cap must block second bot reply, got %d sends", snd.count())
	}
}
