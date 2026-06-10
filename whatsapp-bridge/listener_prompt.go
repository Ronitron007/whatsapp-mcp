package main

import (
	"fmt"
	"strings"
)

const replyInstruction = `---
You are replying in this WhatsApp group chat as the bot account. The transcript above is the recent conversation, oldest first. Lines labeled BOT(me) are your own earlier messages. Lines labeled owner are the human who runs you — also a participant you can and should help.

Rules:
- Output ONLY the message text to send to the group. No preamble, no quotes, no markdown fences.
- Keep it short and chat-appropriate.
- Do not repeat what you already said as BOT(me).
- If no reply is genuinely useful (nothing addressed to you, nothing you can add), output exactly: [SKIP]`

// BuildTranscript renders store messages (newest-first, as GetMessages
// returns them) into an oldest-first transcript for the prompt.
//
// Labeling is signature-based so the bot's own replies are distinguished
// from the account owner's own messages: a signed message is BOT(me); an
// unsigned IsFromMe message is the owner. Without a signature we cannot
// tell them apart, so IsFromMe falls back to BOT(me) (legacy behavior).
func BuildTranscript(msgs []Message, signaturePrefix string) string {
	sig := strings.TrimSpace(signaturePrefix)
	var b strings.Builder
	for i := len(msgs) - 1; i >= 0; i-- {
		m := msgs[i]
		label := m.Sender
		switch {
		case sig != "" && strings.HasPrefix(strings.TrimSpace(m.Content), sig):
			label = "BOT(me)"
		case m.IsFromMe && sig != "":
			label = "owner"
		case m.IsFromMe:
			label = "BOT(me)"
		}
		content := m.Content
		if content == "" && m.MediaType != "" {
			content = fmt.Sprintf("[media: %s]", m.MediaType)
		}
		fmt.Fprintf(&b, "[%s] %s: %s\n", m.Time.Format("2006-01-02 15:04"), label, content)
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
