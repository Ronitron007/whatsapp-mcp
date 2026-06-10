package main

import (
	"strings"
	"testing"
	"time"
)

func TestBuildTranscriptOrdersOldestFirstAndLabels(t *testing.T) {
	ts := func(min int) time.Time { return time.Date(2026, 6, 11, 12, min, 0, 0, time.UTC) }
	// GetMessages returns newest-first; BuildTranscript must reverse.
	msgs := []Message{
		{Time: ts(2), Sender: "919999999999", Content: "see you there", IsFromMe: false},
		{Time: ts(1), Sender: "me", Content: "noted!", IsFromMe: true},
		{Time: ts(0), Sender: "918888888888", Content: "lets meet at 5", IsFromMe: false},
	}
	got := BuildTranscript(msgs)
	lines := strings.Split(strings.TrimSpace(got), "\n")
	if len(lines) != 3 {
		t.Fatalf("want 3 lines, got %d: %q", len(lines), got)
	}
	if !strings.Contains(lines[0], "lets meet at 5") {
		t.Errorf("oldest first, got line0=%q", lines[0])
	}
	if !strings.Contains(lines[1], "BOT(me)") {
		t.Errorf("own messages labeled BOT(me), got %q", lines[1])
	}
	if !strings.HasPrefix(lines[0], "[2026-06-11 12:00] ") {
		t.Errorf("timestamp prefix missing: %q", lines[0])
	}
}

func TestBuildTranscriptMediaPlaceholder(t *testing.T) {
	msgs := []Message{{Time: time.Now(), Sender: "x", MediaType: "image", Content: ""}}
	if got := BuildTranscript(msgs); !strings.Contains(got, "[media: image]") {
		t.Errorf("media placeholder missing: %q", got)
	}
}

func TestBuildPromptContainsAllParts(t *testing.T) {
	p := BuildPrompt("PERSONA-TEXT", "TRANSCRIPT-TEXT")
	for _, want := range []string{"PERSONA-TEXT", "TRANSCRIPT-TEXT", "[SKIP]"} {
		if !strings.Contains(p, want) {
			t.Errorf("prompt missing %q", want)
		}
	}
	if !strings.Contains(p, "Output ONLY the message text") {
		t.Error("prompt missing output instruction")
	}
}

func TestIsSkip(t *testing.T) {
	cases := map[string]bool{
		"[SKIP]":              true,
		"  [SKIP]  ":          true,
		"[SKIP] nothing here": true, // prefix counts
		"":                    true,
		"   ":                 true,
		"sure, 5pm works":     false,
	}
	for in, want := range cases {
		if got := IsSkip(in); got != want {
			t.Errorf("IsSkip(%q)=%v want %v", in, got, want)
		}
	}
}
