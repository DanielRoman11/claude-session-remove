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

### Option A: curl (recommended, no Go required)

Downloads a prebuilt binary for your OS/architecture, verifies its checksum, and installs it — nothing to compile.

```bash
curl -fsSL https://raw.githubusercontent.com/DanielRoman11/claude-session-remove/main/install.sh | bash
```

Step by step, this does:

1. Detects your OS and architecture (linux/darwin, amd64/arm64).
2. Downloads the matching `csr` binary from the [latest release](https://github.com/DanielRoman11/claude-session-remove/releases/latest).
3. Downloads `checksums.txt` from that same release and verifies the download's `sha256` before touching anything else.
4. Extracts and installs the binary to `~/.local/bin/csr` (override with `CSR_INSTALL_DIR=/some/dir`).
5. If `~/.claude/commands/` already exists, it also drops `csr.md` there so `/csr` works right away (set `CSR_INSTALL_COMMAND=1` to force this even without a `~/.claude` directory, or skip it and see "The `/csr` command" below).
6. Prints a `PATH` reminder if `~/.local/bin` isn't on it yet.

Prefer to read the script before running it? It's right here: [`install.sh`](install.sh).

To install a specific version instead of latest: `CSR_VERSION=v1.0.0 curl -fsSL .../install.sh | bash`.

### Option B: Go toolchain

```bash
go install github.com/DanielRoman11/claude-session-remove/cmd/csr@latest
```

Make sure `$(go env GOPATH)/bin` is on your `PATH`.

### Option C: build from source

```bash
git clone https://github.com/DanielRoman11/claude-session-remove
cd claude-session-remove
go build -o ~/.local/bin/csr ./cmd/csr
```

### The `/csr` command (optional)

The curl installer above already does this for you when `~/.claude/commands/` exists. To do it by hand instead, copy `commands/csr.md` into `~/.claude/commands/`:

```bash
mkdir -p ~/.claude/commands
curl -fsSL https://raw.githubusercontent.com/DanielRoman11/claude-session-remove/main/commands/csr.md \
  -o ~/.claude/commands/csr.md
```

or install this as a Claude Code plugin (`.claude-plugin/plugin.json` is already set up for that).

## Usage: standalone (terminal)

1. `cd` into any project directory you've used with Claude Code.
2. Run:
   ```bash
   csr
   ```
3. A full-screen picker opens listing that project's sessions, most recent first, with the current one tagged `● current`.
4. Move the selection with `↑`/`↓` (or `j`/`k`).
5. Press `enter` or `d` on a session to open the delete confirmation.
6. Press `y` to delete, or `n`/`esc` to cancel and go back to the list.
7. On confirmed delete:
   - if it wasn't the active session, `csr` hands off straight into `claude --resume` (via `exec`) so you land on the picker for your remaining sessions;
   - if it *was* the active session, it prints a warning instead and does not resume (see "How it works").
8. Press `q` at any time to quit without deleting anything.

You can also jump straight to a session instead of browsing the full list:

```bash
csr db-migration
```

- **One match** for the title/id substring: goes straight to the confirm dialog (step 5 above).
- **Multiple matches**: opens the picker, scoped to just those matches.
- **No match**: prints `No session found matching "db-migration"` and exits.

If stdin/stdout isn't a real terminal (piped, redirected, scripted), `csr` falls back to a plain numbered prompt instead of the TUI — no extra setup needed.

## Usage: `/csr` inside Claude Code

1. Inside a Claude Code session, type:
   ```
   /csr
   ```
   to target the current session, or `/csr <name>` to match a different one by title/id.
2. Claude runs `csr --list` behind the scenes (no prompts) and resolves your target.
3. If there's more than one match, Claude asks you to pick one via its own question UI.
4. Claude asks you to confirm the deletion (yes/no) the same way.
5. On yes, it runs `csr --id <id> --yes --no-resume` and reports the result in one line.
6. It does **not** chain into `claude --resume` itself (there's no tty inside a tool call to drive that hand-off) — run Claude Code's own `/resume` afterward if you want to switch sessions.

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

1.0.0
