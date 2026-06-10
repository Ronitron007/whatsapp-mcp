# WhatsApp Auto-Reply Bot — Design Spec

**Date:** 2026-06-11
**Base:** Fork of [lharries/whatsapp-mcp](https://github.com/lharries/whatsapp-mcp) (MIT) at [Ronitron007/whatsapp-mcp](https://github.com/Ronitron007/whatsapp-mcp)
**Branch:** `rohan/auto-reply-listener`

## Goal

Claude listens to whitelisted WhatsApp groups and auto-replies as the account owner, instantly, with no approval step. Secondary: keep the existing MCP server working so interactive Claude Code sessions can search/read/send WhatsApp manually.

## Constraints (user-set)

- **Account:** secondary/burner number first; migrate to main number only after stable.
- **Host:** this Mac, **bare metal — no Docker**. Persistence via tmux during experiments, launchd later.
- **Claude:** headless `claude -p` on existing Claude subscription. No API key billing.
- **Privacy:** no third parties (no Beeper, no Matrix homeserver). WhatsApp ⇄ this Mac only.
- **WhatsApp ToS:** whatsmeow is an unofficial client → small ban risk. Burner + guardrails mitigate.

## Architecture

```
WhatsApp servers ⇄ whatsmeow persistent WebSocket (push events, no polling)
                     │
        [Go bridge process: whatsapp-bridge/]
                     ├─ SQLite store/messages.db (chats, messages) ── existing
                     ├─ SQLite store/whatsapp.db (session; QR pair once) ── existing
                     ├─ REST :8080 /api/send etc. ── existing
                     └─ NEW listener: event hook → filters → debounce → claude -p → send
                     
        [Python MCP server: whatsapp-mcp-server/] ── existing, unchanged
                     reads messages.db + calls REST :8080
                     spawned by Claude Code (stdio) for interactive use
```

Two processes total: the Go bridge (always on, owns the listener) and the Python MCP server (only alive while an interactive Claude Code session uses it).

## New component: listener (Go, inside whatsapp-bridge)

New files (`listener.go`, `listener_config.go`, plus tests); hooks into the existing `*events.Message` handler after the store write.

**Filter chain** (all must pass):
1. Chat JID in `listener.json` whitelist.
2. Text message (media-triggered replies out of scope v1).
3. `!IsFromMe` (covers our own sends — loop protection level 1).
4. Fresh: `now - msg.Timestamp < 120s`. whatsmeow replays backlog/history-sync on reconnect; without this the bot answers stale messages.
5. Kill switch file (`store/KILL`) absent.
6. Rate limits not exceeded (below).

**Debounce:** per-chat timer, 15s (configurable). New qualifying message resets the timer; on expiry the batch becomes ONE claude invocation. Groups are bursty; one reply per lull, not per message.

**Claude invocation:**
- Serialize globally (one `claude -p` at a time, FIFO across chats).
- Command: `claude -p --output-format text` (+ `--model` from config, optional).
- Stdin prompt = `configs/persona.md` + last N (default 30) messages of that chat from SQLite as a `[time] sender: text` transcript + fixed instruction: *reply with message text only, or exactly `[SKIP]` if no reply is warranted.*
- Timeout 60s → kill, log, skip. Non-zero exit → log, skip. Never retry a send.
- Output `[SKIP]` or empty → no send.

**Send:** daemon prepends `signature_prefix` (if set), then sends via existing bridge send path, to the originating chat JID only. The model never chooses recipients.

## Security decision: whitelisted tools + folders only

Auto-reply `claude -p` runs with an explicit tool whitelist (user decision: useful replies need reference material, e.g. trip docs). Mechanics: `--allowedTools <list>` + `--add-dir <folders>` flags per invocation, both from config.

- Default allowed: `Read`, `Glob`, `Grep` (scoped to `allowed_dirs`), `WebSearch`.
- Never allowed: `Bash`, `Write`, `Edit`, any MCP server, any send-capable tool. The daemon alone sends, and only to the originating chat — the model never picks recipients.
- `WebFetch` off by default (an injected prompt could exfiltrate folder contents via crafted URLs); enable knowingly via config if needed.
- Rule of thumb: nothing secret in `allowed_dirs` — group messages are untrusted input and anything readable can end up in a reply.

## Guardrails

Config `configs/listener.json` (gitignored; `listener.example.json` committed):

| Field | Default | Purpose |
|---|---|---|
| `whitelist` | `[]` | group JIDs the bot may reply in |
| `dry_run` | `true` | log would-be replies, send nothing |
| `debounce_seconds` | 15 | burst collapsing |
| `context_messages` | 30 | transcript depth per invocation |
| `max_replies_per_hour_per_chat` | 6 | spam brake |
| `max_replies_per_hour_global` | 20 | spam brake |
| `max_replies_per_minute_global` | 2 | hard burst ceiling (main-account safety) |
| `max_consecutive_bot_replies` | 2 | stop until a human speaks again |
| `quiet_hours` | `null` | optional "23:00-08:00" window |
| `model` | `""` (CLI default) | passed to `--model` if set (`"sonnet"` alias OK) |
| `allowed_tools` | `["Read","Glob","Grep","WebSearch"]` | `--allowedTools` for the reply Claude |
| `allowed_dirs` | `[]` | `--add-dir` folders the reply Claude may read |
| `signature_prefix` | `""` | prepended to every outgoing reply (e.g. `"🤖 bot: "`) |

`configs/persona.md` (gitignored; example committed): who the bot is, tone, what to engage with, when to `[SKIP]`.

Defaults are safe: fresh checkout dry-runs with an empty whitelist.

## Failure handling

- WS disconnect → whatsmeow auto-reconnects (built in); freshness filter absorbs the replay.
- `claude -p` timeout/error → skip, log to stdout (tmux scrollback is the v1 log).
- SQLite contention → reads on the same connection pool as existing code; no schema changes needed (rate-limit counters in memory — restart resets them, acceptable).
- Process crash → tmux during experiments; launchd `KeepAlive` plist when promoted (out of scope for P0/P1, in scope P2).

## Rollout

- **P0 — prove the fork:** `go run` bridge, QR-pair burner, confirm messages land in SQLite, hook MCP server into Claude Code and exercise search/send manually. Success: manual send via MCP reaches a phone.
- **P1 — listener, dry run:** implement listener + tests; whitelist one test group (you + burner); watch logged would-be replies for a day. Success: sensible drafts, zero stale-message replies, debounce works.
- **P2 — go live:** `dry_run=false` in the test group, then real groups. launchd plist. Success: a week of replies with no rate-limit trips, no loops, no WA warnings.

## Testing

- Go unit tests: filter chain (whitelist/fresh/IsFromMe), debounce collapse, rate-limit windows, consecutive-reply cap, `[SKIP]` handling. Claude exec faked via interface.
- E2E = P1 dry-run; live test group before real groups.

## Out of scope (v1)

Media/voice replies, DM auto-replies, multi-account, VPS migration, Matrix/Beeper anything, reply threading/quotes, read receipts control.

## Decisions (resolved 2026-06-11)

1. **Identity:** it IS a bot, openly. Burner phase: profile name identifies it, no in-message marker. Main-account phase: `signature_prefix` set so every reply is visibly bot-authored.
2. **First group:** close-friends trip-planning group (JID captured after pairing). `allowed_dirs` can hold trip docs so replies cite real info.
3. **Model:** CLI default; pinning `sonnet` via config is acceptable.
4. **Burner:** in hand — P0 pairing unblocked.
