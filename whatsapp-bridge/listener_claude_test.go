package main

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestCLIInvokerBuildArgs(t *testing.T) {
	cfg := DefaultListenerConfig()
	cfg.Model = "sonnet"
	cfg.AllowedDirs = []string{"/tmp/trip", "/tmp/notes"}
	inv := NewCLIInvoker(cfg)
	args := inv.buildArgs()
	joined := strings.Join(args, " ")

	for _, want := range []string{
		"-p",
		"--output-format text",
		"--model sonnet",
		"--allowedTools Read,Glob,Grep,WebSearch",
		"--disallowedTools " + hardDeniedTools,
		"--strict-mcp-config",
		"--add-dir /tmp/trip",
		"--add-dir /tmp/notes",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("args missing %q in %q", want, joined)
		}
	}
}

func TestCLIInvokerNoModelNoDirs(t *testing.T) {
	inv := NewCLIInvoker(DefaultListenerConfig())
	joined := strings.Join(inv.buildArgs(), " ")
	if strings.Contains(joined, "--model") {
		t.Error("empty model must not emit --model")
	}
	if strings.Contains(joined, "--add-dir") {
		t.Error("no dirs must not emit --add-dir")
	}
}

func TestCLIInvokerRunsRealBinary(t *testing.T) {
	// Use /bin/cat as a stand-in "claude": echoes stdin back.
	cfg := DefaultListenerConfig()
	cfg.ClaudeBinary = "/bin/cat"
	inv := NewCLIInvoker(cfg)
	inv.argsOverride = []string{} // cat takes no claude flags
	out, err := inv.Invoke(context.Background(), "hello prompt")
	if err != nil {
		t.Fatalf("invoke: %v", err)
	}
	if strings.TrimSpace(out) != "hello prompt" {
		t.Errorf("want echoed prompt, got %q", out)
	}
}

func TestCLIInvokerTimeout(t *testing.T) {
	cfg := DefaultListenerConfig()
	cfg.ClaudeBinary = "/bin/sleep"
	inv := NewCLIInvoker(cfg)
	inv.argsOverride = []string{"5"}
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	if _, err := inv.Invoke(ctx, ""); err == nil {
		t.Error("timeout must surface as error")
	}
}
