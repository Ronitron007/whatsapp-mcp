# whatsapp-mcp (fork) — auto-reply bot

## What this is

Fork of lharries/whatsapp-mcp + a listener that auto-replies in whitelisted WhatsApp groups via headless `claude -p`. Spec: `docs/superpowers/specs/2026-06-11-whatsapp-auto-reply-design.md`.

## Language & Stack

- Go (whatsapp-bridge/): whatsmeow bridge + SQLite + REST :8080 + listener. `gofmt` before committing; `go vet` + `go test ./...` must pass.
- Python 3.11+ via uv (whatsapp-mcp-server/): MCP server, stdio. Do not pip-install globally.
- **Bare metal only. No Docker. Ever.** tmux for dev persistence, launchd for production.

## Run

- Bridge + listener: `cd whatsapp-bridge && go run .` (first run shows QR — pair with the burner number, NOT the main account)
- MCP server: registered in Claude Code config, spawned automatically (uv).
- Listener config: `whatsapp-bridge/configs/listener.json` (gitignored). `dry_run: true` until P2.

## Safety rules (non-negotiable)

- Auto-reply `claude -p` invocations get NO MCP tools — text in/out only (prompt-injection surface).
- Never set `dry_run: false` or edit `whitelist` without explicit user instruction.
- Kill switch: `touch whatsapp-bridge/store/KILL` stops all sends instantly.
- Never commit: `store/` (session + messages), `configs/listener.json`, `configs/persona.md`.

## Dependencies

Resolve version conflicts at the root; no workarounds. Go deps via `go.mod` only.

## Debugging

Most obvious cause first; no shotgun logging. Bridge logs to stdout — check tmux scrollback before instrumenting.

## Git

- Branches: `rohan/<topic>`. Never commit to main. PRs via `gh pr create` against the FORK (Ronitron007/whatsapp-mcp), not upstream.
- Never commit secrets/tokens/session DBs; check history before any push.
