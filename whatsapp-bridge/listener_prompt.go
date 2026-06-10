package main

import (
	"fmt"
	"strings"
)

const replyInstruction = `---
You are replying in this WhatsApp group chat as the bot account. The transcript above is the recent conversation, oldest first; lines labeled BOT(me) are your own earlier messages.

Rules:
- Output ONLY the message text to send to the group. No preamble, no quotes, no markdown fences.
- Keep it short and chat-appropriate.
- Do not repeat what you already said as BOT(me).
- If no reply is genuinely useful (nothing addressed to you, nothing you can add), output exactly: [SKIP]`

// BuildTranscript renders store messages (newest-first, as GetMessages
// returns them) into an oldest-first transcript for the prompt.
func BuildTranscript(msgs []Message) string {
	var b strings.Builder
	for i := len(msgs) - 1; i >= 0; i-- {
		m := msgs[i]
		sender := m.Sender
		if m.IsFromMe {
			sender = "BOT(me)"
		}
		content := m.Content
		if content == "" && m.MediaType != "" {
			content = fmt.Sprintf("[media: %s]", m.MediaType)
		}
		fmt.Fprintf(&b, "[%s] %s: %s\n", m.Time.Format("2006-01-02 15:04"), sender, content)
	}
	return b.String()
}

// BuildPrompt assembles the full stdin payload for claude -p.
func BuildPrompt(persona, transcript string) string {
	return persona + "\n\n--- Recent conversation ---\n" + transcript + "\n" + replyInstruction
}

// IsSkip reports whether claude's output means "send nothing".
func IsSkip(output string) bool {
	t := strings.TrimSpace(output)
	return t == "" || strings.HasPrefix(t, "[SKIP]")
}
