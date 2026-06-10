package main

import (
	"encoding/json"
	"fmt"
	"os"
)

// ListenerConfig controls the auto-reply listener. JSON file overlays defaults.
type ListenerConfig struct {
	Whitelist                []string `json:"whitelist"`
	DryRun                   bool     `json:"dry_run"`
	DebounceSeconds          int      `json:"debounce_seconds"`
	ContextMessages          int      `json:"context_messages"`
	MaxRepliesPerHourPerChat int      `json:"max_replies_per_hour_per_chat"`
	MaxRepliesPerHourGlobal  int      `json:"max_replies_per_hour_global"`
	MaxRepliesPerMinGlobal   int      `json:"max_replies_per_minute_global"`
	MaxConsecutiveBotReplies int      `json:"max_consecutive_bot_replies"`
	QuietHours               string   `json:"quiet_hours"` // "" or "23:00-08:00"
	Model                    string   `json:"model"`
	AllowedTools             []string `json:"allowed_tools"`
	AllowedDirs              []string `json:"allowed_dirs"`
	SignaturePrefix          string   `json:"signature_prefix"`
	ClaudeBinary             string   `json:"claude_binary"`
	ClaudeTimeoutSeconds     int      `json:"claude_timeout_seconds"`
	KillSwitchPath           string   `json:"kill_switch_path"`
	PersonaPath              string   `json:"persona_path"`
}

func DefaultListenerConfig() ListenerConfig {
	return ListenerConfig{
		Whitelist:                nil,
		DryRun:                   true,
		DebounceSeconds:          15,
		ContextMessages:          30,
		MaxRepliesPerHourPerChat: 6,
		MaxRepliesPerHourGlobal:  20,
		MaxRepliesPerMinGlobal:   2,
		MaxConsecutiveBotReplies: 2,
		AllowedTools:             []string{"Read", "Glob", "Grep", "WebSearch"},
		ClaudeBinary:             "claude",
		ClaudeTimeoutSeconds:     60,
		KillSwitchPath:           "store/KILL",
		PersonaPath:              "configs/persona.md",
	}
}

// Enabled reports whether the listener should run at all.
func (c ListenerConfig) Enabled() bool { return len(c.Whitelist) > 0 }

func (c ListenerConfig) validate() error {
	if c.DebounceSeconds < 1 || c.DebounceSeconds > 300 {
		return fmt.Errorf("debounce_seconds must be 1-300, got %d", c.DebounceSeconds)
	}
	if c.ContextMessages < 1 || c.ContextMessages > 200 {
		return fmt.Errorf("context_messages must be 1-200, got %d", c.ContextMessages)
	}
	if c.ClaudeTimeoutSeconds < 5 {
		return fmt.Errorf("claude_timeout_seconds must be >=5, got %d", c.ClaudeTimeoutSeconds)
	}
	if c.QuietHours != "" {
		if _, _, err := parseQuietHours(c.QuietHours); err != nil {
			return err
		}
	}
	if c.Enabled() {
		if _, err := os.Stat(c.PersonaPath); err != nil {
			return fmt.Errorf("listener enabled but persona file unreadable at %s: %v", c.PersonaPath, err)
		}
	}
	return nil
}

// LoadListenerConfig reads path over defaults. A missing file is not an
// error: it returns a disabled (empty-whitelist) config.
func LoadListenerConfig(path string) (ListenerConfig, error) {
	cfg := DefaultListenerConfig()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, err
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("parse %s: %v", path, err)
	}
	return cfg, cfg.validate()
}
