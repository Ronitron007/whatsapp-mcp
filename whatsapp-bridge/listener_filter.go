package main

import (
	"fmt"
	"strings"
	"time"
)

// LiveMsg is the normalized live message the listener consumes.
// Built in handleMessage from *events.Message.
type LiveMsg struct {
	ChatJID   string
	Sender    string
	Content   string
	Timestamp time.Time
	IsFromMe  bool
}

// PassesPrefilter gates messages before any further processing.
// maxAge guards against whatsmeow's offline/backlog replay on reconnect:
// without it the bot would answer messages from hours ago.
//
// Loop protection is signature-based: when signaturePrefix is set, any
// message whose text starts with it is skipped (the bot's own replies,
// their WhatsApp echoes, or someone impersonating the bot). Crucially,
// the account owner's OWN non-signed messages are NOT skipped, so the bot
// replies to the owner too. When no signature is configured we cannot tell
// the bot apart from the owner, so we fall back to skipping all IsFromMe
// messages to stay loop-safe.
func PassesPrefilter(whitelist map[string]bool, m LiveMsg, now time.Time, maxAge time.Duration, signaturePrefix string) (bool, string) {
	if !whitelist[m.ChatJID] {
		return false, "chat not whitelisted"
	}
	if sig := strings.TrimSpace(signaturePrefix); sig != "" {
		if strings.HasPrefix(strings.TrimSpace(m.Content), sig) {
			return false, "bot-signed message (loop guard)"
		}
	} else if m.IsFromMe {
		return false, "own message (no signature configured)"
	}
	if now.Sub(m.Timestamp) > maxAge {
		return false, fmt.Sprintf("stale (%s old)", now.Sub(m.Timestamp).Round(time.Second))
	}
	return true, ""
}

// parseQuietHours parses "HH:MM-HH:MM" into minutes-since-midnight.
func parseQuietHours(spec string) (startMin, endMin int, err error) {
	parts := strings.Split(spec, "-")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("quiet_hours must be HH:MM-HH:MM, got %q", spec)
	}
	parse := func(s string) (int, error) {
		var h, m int
		if _, err := fmt.Sscanf(strings.TrimSpace(s), "%d:%d", &h, &m); err != nil {
			return 0, fmt.Errorf("bad time %q in quiet_hours", s)
		}
		if h < 0 || h > 23 || m < 0 || m > 59 {
			return 0, fmt.Errorf("out-of-range time %q in quiet_hours", s)
		}
		return h*60 + m, nil
	}
	if startMin, err = parse(parts[0]); err != nil {
		return 0, 0, err
	}
	if endMin, err = parse(parts[1]); err != nil {
		return 0, 0, err
	}
	return startMin, endMin, nil
}

// InQuietHours reports whether now falls inside the configured window.
// Empty spec means no quiet hours. Window may wrap midnight.
func InQuietHours(spec string, now time.Time) bool {
	if spec == "" {
		return false
	}
	start, end, err := parseQuietHours(spec)
	if err != nil {
		return false // validated at load; defensive only
	}
	cur := now.Hour()*60 + now.Minute()
	if start <= end {
		return cur >= start && cur < end
	}
	return cur >= start || cur < end // wraps midnight
}
