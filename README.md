# context-hub

A TUI to browse, resume, and delete your AI coding sessions — across Claude Code, OpenCode, and Kimi Code — from one place.

## Overview

Every one of these tools keeps its own session history, with its own picker, its own storage format, and no way to see them side by side. `context-hub` lists all of them for the current project directory in a single full-screen picker, sorted by recency, each tagged with its own icon, so you can jump back into any session (whichever tool it belongs to) or clean up old ones without hunting through three different pickers.

```
╭─ context-hub ───────────────────────────────────────────────── 3 sessions ─╮
│ › ✻ Fix the very very long authentication flow bug in login          │
│    Claude Code · Sep 16 · current                          2h ago    │
│                                                                       │
│   ▦ Old experiment                                                   │
│    OpenCode · Sep 15                                        1d ago   │
│                                                                       │
│   ☾ Db migration test                                                │
│    Kimi Code · Sep 13                                       3d ago   │
│                                                                       │
├─────────────────────────────────────────────────────────────────────┤
│ ↑/↓ navigate    enter/o open    d delete    q quit                   │
╰─────────────────────────────────────────────────────────────────────╯
```

The panel fills the whole terminal (this is trimmed to fit here). Each session is a two-line card: title on top, provider + opened date + current tag on the line below, time-since-last-touched right-aligned in a muted color. The theme is [Catppuccin](https://catppuccin.com) (Mocha on dark terminals, Latte on light ones), with each provider getting its own accent.

Two ways to use it:

- **Standalone, in any terminal:** run `context-hub`. No tokens spent, a full-screen picker built with [Bubble Tea](https://github.com/charmbracelet/bubbletea) and [Lipgloss](https://github.com/charmbracelet/lipgloss).
- **`/context-hub` inside Claude Code:** the command runs the same binary in a non-interactive mode and only asks you two things (which session, and to confirm) through Claude Code's own UI.

### Supported providers

| Provider | Icon | List/delete via | Notes |
|---|---|---|---|
| Claude Code | ✻ | reads `~/.claude/projects/<cwd>/*.jsonl` directly | Codicons does define a real Claude mark (`cod-claude`, U+EC82), but it's new enough that no released Nerd Font build ships it yet (checked against a real install), so it'd render as a blank box for effectively everyone — this uses a plain Unicode sunburst instead |
| OpenCode | ▦ | shells out to `opencode session list/delete` | no official glyph exists anywhere for OpenCode, so this is the closest Unicode approximation of its modular pixel-block mark; uses OpenCode's own CLI, never touches its sqlite db directly |
| Kimi Code | ☾ | reads `~/.kimi-code/session_index.jsonl` + `state.json` | no official glyph exists for Kimi Code either, so this leans on Moonshot AI's own moon branding (their Chinese name literally means "the dark side of the moon"). Also best-effort: written from Kimi Code's docs, not verified against a live install. If your sessions don't show up, please open an issue with what `~/.kimi-code/session_index.jsonl` looks like |

A provider whose CLI isn't installed (or that has no sessions for the current directory) is silently skipped — you'll just see the others.

## Install

### Option A: curl (recommended, no Go required)

Downloads a prebuilt binary for your OS/architecture, verifies its checksum, and installs it — nothing to compile.

```bash
curl -fsSL https://raw.githubusercontent.com/DanielRoman11/context-hub/main/install.sh | bash
```

Step by step, this does:

1. Detects your OS and architecture (linux/darwin, amd64/arm64).
2. Downloads the matching `context-hub` binary from the [latest release](https://github.com/DanielRoman11/context-hub/releases/latest).
3. Downloads `checksums.txt` from that same release and verifies the download's `sha256` before touching anything else.
4. Extracts and installs the binary to `~/.local/bin/context-hub` (override with `CONTEXT_HUB_INSTALL_DIR=/some/dir`).
5. If `~/.claude/commands/` already exists, it also drops `context-hub.md` there so `/context-hub` works right away (set `CONTEXT_HUB_INSTALL_COMMAND=1` to force this even without a `~/.claude` directory).
6. Prints a `PATH` reminder if `~/.local/bin` isn't on it yet.

Prefer to read the script before running it? It's right here: [`install.sh`](install.sh).

To install a specific version instead of latest: `CONTEXT_HUB_VERSION=v2.0.0 curl -fsSL .../install.sh | bash`.

### Option B: Go toolchain

```bash
go install github.com/DanielRoman11/context-hub/cmd/context-hub@latest
```

Make sure `$(go env GOPATH)/bin` is on your `PATH`.

### Option C: build from source

```bash
git clone https://github.com/DanielRoman11/context-hub
cd context-hub
go build -o ~/.local/bin/context-hub ./cmd/context-hub
```

### The `/context-hub` command (optional)

The curl installer above already does this for you when `~/.claude/commands/` exists. To do it by hand instead:

```bash
mkdir -p ~/.claude/commands
curl -fsSL https://raw.githubusercontent.com/DanielRoman11/context-hub/main/commands/context-hub.md \
  -o ~/.claude/commands/context-hub.md
```

or install this as a Claude Code plugin (`.claude-plugin/plugin.json` is already set up for that).

## Usage: standalone (terminal)

1. `cd` into any project directory you've used with Claude Code, OpenCode, or Kimi Code.
2. Run:
   ```bash
   context-hub
   ```
3. A full-screen picker opens listing every session for that directory across all installed providers, most recent first — each row shows the provider's icon, the date it was opened (when available), the title, and how long ago it was last touched, right-aligned in a muted color.
4. Move the selection with `↑`/`↓` (or `j`/`k`).
5. Press `enter` or `o` to **open/resume** that exact session in its original tool (execs straight into it, replacing this process).
6. Press `d` to **delete** instead: opens a confirm dialog.
7. On the confirm dialog, press `y` to delete or `n`/`esc` to cancel and go back to the list.
8. On a confirmed delete:
   - if it wasn't the active Claude Code session, `context-hub` hands off into that provider's own continuation UI (e.g. `claude --resume`'s picker) so you land on your remaining sessions;
   - if it *was* the active Claude Code session, it prints a warning instead and does not relaunch (see "How it works").
9. Press `q` at any time to quit without doing anything.

You can also jump straight to a session instead of browsing the full list:

```bash
context-hub db-migration
```

- **One match** for the title/id substring: goes straight to the delete confirm dialog.
- **Multiple matches**: opens the picker, scoped to just those matches.
- **No match**: prints `No session found matching "db-migration"` and exits.

If stdin/stdout isn't a real terminal (piped, redirected, scripted), `context-hub` falls back to a plain numbered delete prompt instead of the TUI — no extra setup needed.

## Usage: `/context-hub` inside Claude Code

1. Inside a Claude Code session, type:
   ```
   /context-hub
   ```
   to target the current session, or `/context-hub <name>` to match a different one by title/id (across any provider).
2. Claude runs `context-hub --list` behind the scenes (no prompts) and resolves your target.
3. If there's more than one match, Claude asks you to pick one via its own question UI.
4. Claude asks you to confirm the deletion (yes/no) the same way.
5. On yes, it runs `context-hub --id <provider>:<id> --yes --no-resume` and reports the result in one line.
6. It does **not** relaunch anything itself (there's no tty inside a tool call to drive that hand-off) — use `/resume` (Claude Code) or reopen the relevant tool afterward if you want to switch sessions.

## Non-interactive flags

For scripting or the `/context-hub` command:

- `--list` — print `provider_slug<TAB>id<TAB>provider_name<TAB>title<TAB>is_current<TAB>updated_unix` for every session, no prompts, no TUI.
- `--id <provider_slug>:<id> --yes [--no-resume]` — delete that exact session without any prompt.

## How it works

Each provider is a small adapter behind a common interface (`List`, `Delete`, `Resume`, `Relaunch` — see `internal/agents/`):

- **Claude Code**: lists the `.jsonl` transcripts under `~/.claude/projects/<encoded-cwd>/`, deriving title and timestamps from each entry's `timestamp` field (falling back to file mtime). Delete removes the file. Resume/relaunch exec `claude --resume [id]`.
- **OpenCode**: shells out to `opencode session list --format json` and `opencode session delete <id>` — no direct database access, so it stays correct across OpenCode's own schema changes. Resume/relaunch exec `opencode --session <id>` / bare `opencode`.
- **Kimi Code**: reads the `session_index.jsonl` index and each session's `state.json`, matching fields by substring rather than exact key name since the schema isn't verified against a live install. Delete removes the session's directory. Resume/relaunch exec `kimi --session [id]`.

Relaunching after a delete is skipped if the deleted session was the currently active Claude Code one: `claude --resume` with no explicit id can fall back to the most recently active session — the one you just deleted — and Claude Code would recreate a blank transcript under that same id. In that case `context-hub` just tells you to exit and start fresh instead.

## Limitations

- Only the session record itself is removed; this doesn't search for or delete other unrelated state from these tools.
- "Current session" detection only works for Claude Code (via `CLAUDE_CODE_SESSION_ID`, set when running inside a Claude Code session) — OpenCode and Kimi Code sessions are never tagged `current`.
- Relaunching requires the corresponding CLI (`claude`, `opencode`, `kimi`) on `PATH`; if it's missing, `context-hub` reports the deletion and exits instead.
- The Kimi Code adapter is unverified against a real installation — please report issues.

## Author

Daniel Roman

## Version

2.1.0
