package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultListenerConfig(t *testing.T) {
	c := DefaultListenerConfig()
	if !c.DryRun {
		t.Error("DryRun must default true")
	}
	if c.Enabled() {
		t.Error("empty whitelist must mean disabled")
	}
	if c.DebounceSeconds != 15 || c.ContextMessages != 30 {
		t.Errorf("bad defaults: %+v", c)
	}
	if c.MaxRepliesPerHourPerChat != 6 || c.MaxRepliesPerHourGlobal != 20 || c.MaxConsecutiveBotReplies != 2 {
		t.Errorf("bad limit defaults: %+v", c)
	}
	if len(c.AllowedTools) != 4 || c.AllowedTools[0] != "Read" {
		t.Errorf("bad AllowedTools default: %v", c.AllowedTools)
	}
	if c.ClaudeBinary != "claude" || c.ClaudeTimeoutSeconds != 60 {
		t.Errorf("bad claude defaults: %+v", c)
	}
	if c.KillSwitchPath != "store/KILL" || c.PersonaPath != "configs/persona.md" {
		t.Errorf("bad path defaults: %+v", c)
	}
}

func TestLoadListenerConfigMissingFileDisabled(t *testing.T) {
	c, err := LoadListenerConfig(filepath.Join(t.TempDir(), "nope.json"))
	if err != nil {
		t.Fatalf("missing file must not error: %v", err)
	}
	if c.Enabled() {
		t.Error("missing file must mean disabled")
	}
}

func TestLoadListenerConfigOverlayAndValidate(t *testing.T) {
	dir := t.TempDir()
	persona := filepath.Join(dir, "persona.md")
	os.WriteFile(persona, []byte("you are a bot"), 0644)
	cfgPath := filepath.Join(dir, "listener.json")
	os.WriteFile(cfgPath, []byte(`{
		"whitelist": ["123-456@g.us"],
		"dry_run": false,
		"debounce_seconds": 5,
		"persona_path": "`+persona+`"
	}`), 0644)

	c, err := LoadListenerConfig(cfgPath)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if c.DryRun {
		t.Error("explicit dry_run:false must override default")
	}
	if c.DebounceSeconds != 5 || !c.Enabled() {
		t.Errorf("overlay failed: %+v", c)
	}
	if c.ContextMessages != 30 {
		t.Error("absent fields must keep defaults")
	}
}

func TestLoadListenerConfigEnabledNeedsPersona(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "listener.json")
	os.WriteFile(cfgPath, []byte(`{"whitelist":["x@g.us"],"persona_path":"`+filepath.Join(dir, "missing.md")+`"}`), 0644)
	if _, err := LoadListenerConfig(cfgPath); err == nil {
		t.Error("enabled config with missing persona file must error")
	}
}

func TestLoadListenerConfigRejectsBadValues(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "listener.json")
	os.WriteFile(cfgPath, []byte(`{"debounce_seconds": 0}`), 0644)
	if _, err := LoadListenerConfig(cfgPath); err == nil {
		t.Error("debounce_seconds<1 must error")
	}
}
