# Session Cleanup Plugin

Delete Claude Code session transcripts directly from the CLI, without leaving your terminal.

## Overview

Sessions can only be created, resumed, and cleared today — there's no way to permanently remove one. This plugin adds a `/delete-session` command that finds and deletes a session's transcript file for you, with confirmation, so leftover test sessions (or ones containing sensitive data) don't have to be cleaned up by hand in `~/.claude/projects/`.

## Commands

### `/delete-session`

Deletes the **current** session after confirmation.

```
> /delete-session
```

### `/delete-session <name>`

Deletes a session matched by title or session id, similar to how `/resume` lists sessions.

```
> /delete-session my-old-session
```

- **Exact/substring match:** confirms and deletes that session.
- **Multiple matches:** shows a numbered list to pick from.
- **No match:** reports `No session found matching "<name>"`.

## Example

```
> /delete-session test
Multiple sessions found:
  1. test-auth-flow
  2. test-database-migration
  3. test-ui-components
[pick one]
Delete session "test-database-migration" (~/.claude/projects/.../<id>.jsonl)?
[confirm]
Session deleted.
```

## How it works

Claude Code stores each session as a `.jsonl` transcript under `~/.claude/projects/<encoded-cwd>/<session-id>.jsonl`. The command:

1. Lists the `.jsonl` files for the current project directory.
2. Derives a display title per session from its `ai-title` entries (falling back to the first user message).
3. Resolves your target (current session, or a name/id match).
4. Asks for confirmation before deleting anything.
5. Removes only the confirmed file with `rm`.

## Limitations

- Deleting the *current* session removes its transcript file, but a running session can't relaunch itself — exit (`/exit` or Ctrl-D) and start a new `claude` process afterward to fully leave it.
- Only the transcript file is removed; it does not search for or delete other unrelated Claude Code state.

## Requirements

- Must be run from within a Claude Code session (for the "current session" mode).

## Author

Daniel Roman

## Version

1.0.0
