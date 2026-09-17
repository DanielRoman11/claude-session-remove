# Session Cleanup Plugin

Delete Claude Code session transcripts, either from a plain terminal (zero tokens) or with a `/delete-session` command inside Claude Code (minimal, deterministic).

## Overview

Sessions can only be created, resumed, and cleared today — there's no way to permanently remove one. This plugin ships `claude-delete-session`, a small Go binary that finds and deletes a session's transcript file for you, with confirmation, so leftover test sessions (or ones containing sensitive data) don't have to be cleaned up by hand in `~/.claude/projects/`.

Two ways to use it:

- **Standalone, in any terminal:** run the binary directly. No Claude process involved, no tokens spent, full interactive picker.
- **`/delete-session` inside Claude Code:** the command runs the same binary in a non-interactive mode and only asks you two things (which session, and to confirm) through Claude Code's own UI — it does not reason about parsing sessions itself, that logic lives entirely in the binary.

## Install

```bash
cd plugins/session-cleanup
go build -o ~/.local/bin/claude-delete-session .
```

Make sure `~/.local/bin` (or wherever you put the binary) is on your `PATH`. For the `/delete-session` command, also copy `commands/delete-session.md` into `~/.claude/commands/` (or install the plugin through Claude Code's plugin system).

## Usage: standalone (terminal)

### `claude-delete-session`

Lists every session for the current project directory, most recent first, like `claude --resume` does, and marks the current one (if `CLAUDE_CODE_SESSION_ID` is set) so you can pick which to delete:

```
$ claude-delete-session
Sessions for this project:
  1. Fix auth flow [f3a1...b2]
  2. Old experiment [9c02...ee] (current)
  3. Db migration test [11ab...44]
Select a session to delete (1-3, or Enter to cancel): 3
Delete session "Db migration test" (~/.claude/projects/.../11ab....jsonl)? (y/N) y
Session deleted.
```

### `claude-delete-session <name>`

Filters first by title or session id substring before confirming:

```
$ claude-delete-session db-migration
Delete session "Db migration test" (~/.claude/projects/.../11ab....jsonl)? (y/N) y
Session deleted.
```

- **One match:** confirms and deletes that session.
- **Multiple matches:** shows the same numbered picker, scoped to the matches.
- **No match:** prints `No session found matching "<name>"` and exits.

After a confirmed deletion, it hands off straight into `claude --resume` (via `exec`, replacing the process) so you land on the picker for your remaining sessions, and you never see the transcript you just deleted since its file is already gone.

## Usage: `/delete-session` inside Claude Code

```
> /delete-session
> /delete-session db-migration
```

Behind the scenes this runs `claude-delete-session --list` to read the sessions (no prompts), resolves your target, asks you to pick/confirm through Claude Code's own question UI (a real shell `y/n` prompt can't be answered from inside a tool call — there's no attached tty), then runs `claude-delete-session --id <id> --yes --no-resume` to actually delete. It does not chain into `claude --resume` itself; use Claude Code's own `/resume` afterward if you want to switch sessions.

## Non-interactive flags

For scripting or the `/delete-session` command:

- `--list` — print `id<TAB>title<TAB>is_current<TAB>mtime` for every session, no prompts.
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
