package main

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// hardDeniedTools are denied regardless of allowed_tools config:
// the replier must never execute, write, or fetch arbitrary URLs.
const hardDeniedTools = "Bash,Write,Edit,NotebookEdit,WebFetch"

// ClaudeInvoker produces a reply (or [SKIP]) for a prompt.
type ClaudeInvoker interface {
	Invoke(ctx context.Context, prompt string) (string, error)
}

// CLIInvoker shells out to the claude CLI in -p (headless print) mode,
// prompt on stdin, reply on stdout.
type CLIInvoker struct {
	cfg          ListenerConfig
	argsOverride []string // tests only
}

func NewCLIInvoker(cfg ListenerConfig) *CLIInvoker {
	return &CLIInvoker{cfg: cfg}
}

func (c *CLIInvoker) buildArgs() []string {
	args := []string{"-p", "--output-format", "text",
		"--allowedTools", strings.Join(c.cfg.AllowedTools, ","),
		"--disallowedTools", hardDeniedTools,
		"--strict-mcp-config", // never load user/project MCP servers
	}
	if c.cfg.Model != "" {
		args = append(args, "--model", c.cfg.Model)
	}
	for _, d := range c.cfg.AllowedDirs {
		args = append(args, "--add-dir", d)
	}
	return args
}

func (c *CLIInvoker) Invoke(ctx context.Context, prompt string) (string, error) {
	args := c.buildArgs()
	if c.argsOverride != nil {
		args = c.argsOverride
	}
	cmd := exec.CommandContext(ctx, c.cfg.ClaudeBinary, args...)
	cmd.Stdin = strings.NewReader(prompt)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("claude invocation failed: %v (stderr: %s)", err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}
