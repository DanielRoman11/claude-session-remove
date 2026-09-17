# Session Cleanup Plugin

Delete Claude Code session transcripts from a plain terminal, no AI turn involved.

## Overview

Sessions can only be created, resumed, and cleared today — there's no way to permanently remove one. This plugin ships `claude-delete-session`, a self-contained bash script that finds and deletes a session's transcript file for you, with confirmation, so leftover test sessions (or ones containing sensitive data) don't have to be cleaned up by hand in `~/.claude/projects/`.

It is a plain script on purpose: it runs entirely outside Claude, so deleting a session costs zero tokens and works from any terminal, not just from inside a live session.

## Install

```bash
cp plugins/session-cleanup/scripts/delete-session.sh ~/.local/bin/claude-delete-session
chmod +x ~/.local/bin/claude-delete-session
```

Make sure `~/.local/bin` (or wherever you copy it) is on your `PATH`.

## Usage

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

After a confirmed deletion, it hands off straight into `claude --resume` (via `exec`, replacing the script process) so you land on the picker for your remaining sessions instead of being left at a bare shell, and you never see the transcript you just deleted since its file is already gone.

## How it works

Claude Code stores each session as a `.jsonl` transcript under `~/.claude/projects/<encoded-cwd>/<session-id>.jsonl`. The script:

1. Lists the `.jsonl` files for the current project directory.
2. Derives a display title per session from its `ai-title` entries (falling back to the first user message).
3. Resolves your target: every session (no argument), or a name/id match.
4. Asks for confirmation before deleting anything.
5. Removes only the confirmed file with `rm`.
6. Runs `exec claude --resume` so you're immediately back in a picker.

## Limitations

- Only the transcript file is removed; it does not search for or delete other unrelated Claude Code state.
- Requires `claude` on your `PATH` for the resume hand-off; if it's missing, the script just reports the deletion and exits instead.

## Author

Daniel Roman

## Version

1.0.0
