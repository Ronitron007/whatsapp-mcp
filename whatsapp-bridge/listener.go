package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	waLog "go.mau.fi/whatsmeow/util/log"
)

// SendFunc sends text to a chat JID. Wired to sendWhatsAppMessage in main.
type SendFunc func(chatJID, text string) (bool, string)

// Listener is the auto-reply pipeline. One instance per process.
type Listener struct {
	cfg       ListenerConfig
	whitelist map[string]bool
	store     *MessageStore
	send      SendFunc
	invoker   ClaudeInvoker
	limiter   *Limiter
	debouncer *Debouncer
	log       waLog.Logger
	claudeMu  sync.Mutex // serialize claude invocations globally
}

// NewListener wires the pipeline. logger may be nil (tests).
func NewListener(cfg ListenerConfig, store *MessageStore, send SendFunc, invoker ClaudeInvoker, logger waLog.Logger) *Listener {
	if logger == nil {
		logger = waLog.Noop
	}
	l := &Listener{
		cfg:       cfg,
		whitelist: map[string]bool{},
		store:     store,
		send:      send,
		invoker:   invoker,
		limiter:   NewLimiter(cfg.MaxRepliesPerHourPerChat, cfg.MaxRepliesPerHourGlobal, cfg.MaxRepliesPerMinGlobal, cfg.MaxConsecutiveBotReplies),
		log:       logger,
	}
	for _, j := range cfg.Whitelist {
		l.whitelist[j] = true
	}
	l.debouncer = NewDebouncer(time.Duration(cfg.DebounceSeconds)*time.Second, l.fire)
	return l
}

// setDebounceForTest swaps in a faster debouncer. Tests only.
func (l *Listener) setDebounceForTest(d time.Duration) {
	l.debouncer.Stop()
	l.debouncer = NewDebouncer(d, l.fire)
}

// OnMessage is called from handleMessage for every stored live message.
// Cheap and non-blocking: heavy work happens on debounce fire.
func (l *Listener) OnMessage(m LiveMsg) {
	ok, reason := PassesPrefilter(l.whitelist, m, time.Now(), 2*time.Minute)
	if !ok {
		if l.whitelist[m.ChatJID] {
			l.log.Infof("listener: ignoring message in %s: %s", m.ChatJID, reason)
		}
		return
	}
	// A live human message in a whitelisted chat resets the consecutive
	// counter even if it is media-only.
	l.limiter.RecordHuman(m.ChatJID)
	if m.Content == "" {
		return // media-only: counts as human activity, never triggers a reply
	}
	if l.killSwitchActive() {
		l.log.Warnf("listener: kill switch present, not scheduling")
		return
	}
	l.debouncer.Schedule(m.ChatJID)
}

// fire runs after a chat's debounce window goes quiet.
func (l *Listener) fire(chatJID string) {
	now := time.Now()
	if InQuietHours(l.cfg.QuietHours, now) {
		l.log.Infof("listener: quiet hours, skipping %s", chatJID)
		return
	}
	if ok, reason := l.limiter.Allow(chatJID, now); !ok {
		l.log.Warnf("listener: limited for %s: %s", chatJID, reason)
		return
	}
	persona, err := os.ReadFile(l.cfg.PersonaPath)
	if err != nil {
		l.log.Errorf("listener: persona unreadable: %v", err)
		return
	}
	msgs, err := l.store.GetMessages(chatJID, l.cfg.ContextMessages)
	if err != nil {
		l.log.Errorf("listener: transcript fetch failed for %s: %v", chatJID, err)
		return
	}
	prompt := BuildPrompt(string(persona), BuildTranscript(msgs))

	l.claudeMu.Lock()
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(l.cfg.ClaudeTimeoutSeconds)*time.Second)
	out, err := l.invoker.Invoke(ctx, prompt)
	cancel()
	l.claudeMu.Unlock()
	if err != nil {
		l.log.Errorf("listener: claude failed for %s: %v", chatJID, err)
		return // never retry: at-most-once replies
	}
	if IsSkip(out) {
		l.log.Infof("listener: claude chose [SKIP] for %s", chatJID)
		return
	}
	reply := l.cfg.SignaturePrefix + strings.TrimSpace(out)

	if l.killSwitchActive() {
		l.log.Warnf("listener: kill switch engaged after generation, dropping reply for %s", chatJID)
		return
	}
	if l.cfg.DryRun {
		l.log.Infof("listener DRY RUN would send to %s: %s", chatJID, reply)
		fmt.Printf("[DRY RUN] %s -> %s\n", chatJID, reply)
		return
	}
	success, status := l.send(chatJID, reply)
	if !success {
		l.log.Errorf("listener: send failed for %s: %s", chatJID, status)
		return
	}
	l.limiter.RecordBotReply(chatJID, now)
	// Store our own reply: whatsmeow does not echo API-sent messages back
	// as events, and transcripts must include what the bot already said.
	id := fmt.Sprintf("bot-%d", time.Now().UnixNano())
	if err := l.store.StoreMessage(id, chatJID, "me", reply, time.Now(), true, "", "", "", nil, nil, nil, 0); err != nil {
		l.log.Warnf("listener: failed to store own reply: %v", err)
	}
	l.log.Infof("listener: replied in %s", chatJID)
}

func (l *Listener) killSwitchActive() bool {
	_, err := os.Stat(l.cfg.KillSwitchPath)
	return err == nil
}

// Stop cancels pending debounce timers.
func (l *Listener) Stop() {
	if l.debouncer != nil {
		l.debouncer.Stop()
	}
}
