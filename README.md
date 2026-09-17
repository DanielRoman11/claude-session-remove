# claude-session-remove

Delete Claude Code session transcripts, either from a plain terminal (zero tokens, full TUI) or with a `/csr` command inside Claude Code (minimal, deterministic).

## Overview

Sessions can only be created, resumed, and cleared today — there's no way to permanently remove one. This project ships `csr`, a small Go binary that finds and deletes a session's transcript file for you, with confirmation, so leftover test sessions (or ones containing sensitive data) don't have to be cleaned up by hand in `~/.claude/projects/`.

Two ways to use it:

- **Standalone, in any terminal:** run `csr`. No Claude process involved, no tokens spent, a full-screen picker built with [Bubble Tea](https://github.com/charmbracelet/bubbletea) and [Lipgloss](https://github.com/charmbracelet/lipgloss), styled after Claude Code's own palette.
- **`/csr` inside Claude Code:** the command runs the same binary in a non-interactive mode and only asks you two things (which session, and to confirm) through Claude Code's own UI — it does not reason about parsing sessions itself, that logic lives entirely in the binary.

```
╭──────────────────────────────────────────────╮
│ Claude Code Sessions  3 found                 │
│                                                │
│ ›  Fix auth flow                    2h ago    │
│    Old experiment    ● current      1d ago    │
│    Db migration test                3d ago    │
│                                                │
│ ↑/↓ navigate    enter/d delete    q quit       │
╰──────────────────────────────────────────────╯
```

## Install

### The binary

```bash
go install github.com/DanielRoman11/claude-session-remove/cmd/csr@latest
```

or build from a local clone:

```bash
git clone https://github.com/DanielRoman11/claude-session-remove
cd claude-session-remove
go build -o ~/.local/bin/csr ./cmd/csr
```

Make sure `~/.local/bin` (or wherever `go install` puts binaries — usually `$(go env GOPATH)/bin`) is on your `PATH`.

### The `/csr` command (optional)

Copy `commands/csr.md` into `~/.claude/commands/`, or install this as a Claude Code plugin (`.claude-plugin/plugin.json` is already set up for that).

## Usage: standalone (terminal)

### `csr`

Opens the picker: every session for the current project directory, most recent first, current one tagged. Arrow keys (or `j`/`k`) move, `enter`/`d` opens a confirm dialog, `y` deletes, `n`/`esc` cancels, `q` quits.

### `csr <name>`

Filters first by title or session id substring:

- **One match:** goes straight to the confirm dialog.
- **Multiple matches:** opens the picker, scoped to the matches.
- **No match:** prints `No session found matching "<name>"` and exits.

After a confirmed deletion, it hands off straight into `claude --resume` (via `exec`, replacing the process) so you land on the picker for your remaining sessions, and you never see the transcript you just deleted since its file is already gone.

If stdin/stdout isn't a real terminal (piped, redirected, scripted), `csr` falls back to a plain numbered prompt instead of the TUI.

## Usage: `/csr` inside Claude Code

```
> /csr
> /csr db-migration
```

Behind the scenes this runs `csr --list` to read the sessions (no prompts), resolves your target, asks you to pick/confirm through Claude Code's own question UI (a real TUI can't be driven from inside a tool call — there's no attached tty), then runs `csr --id <id> --yes --no-resume` to actually delete. It does not chain into `claude --resume` itself; use Claude Code's own `/resume` afterward if you want to switch sessions.

## Non-interactive flags

For scripting or the `/csr` command:

- `--list` — print `id<TAB>title<TAB>is_current<TAB>mtime` for every session, no prompts, no TUI.
- `--id <id> --yes [--no-resume]` — delete that exact session id without any prompt.

## How it works

Claude Code stores each session as a `.jsonl` transcript under `~/.claude/projects/<encoded-cwd>/<session-id>.jsonl`. The binary:

1. Lists the `.jsonl` files for the current project directory.
2. Derives a display title per session from its `ai-title` entries (falling back to the first user message).
3. Resolves your target: every session (no argument), a name/id match, or an explicit `--id`.
4. Asks for confirmation before deleting anything (unless `--yes`).
5. Removes only the confirmed file.
6. In interactive mode, execs `claude --resume` unless `--no-resume` was passed.

## Limitations

- Only the transcript file is removed; it does not search for or delete other unrelated Claude Code state.
- The resume hand-off requires `claude` on `PATH`; if it's missing, it just reports the deletion and exits instead.

## Author

Daniel Roman

## Version

2.0.0
