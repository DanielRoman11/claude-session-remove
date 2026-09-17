---
allowed-tools: Bash(claude-delete-session:*), AskUserQuestion
argument-hint: [session-name]
description: Delete a Claude Code session (current one, or matched by name) using the claude-delete-session binary
---

## Argument

```
$ARGUMENTS
```

## Task

Do not reason about this beyond what's below. No exploring, no extra commands.

1. Run `claude-delete-session --list` (one Bash call). Each line is `id<TAB>title<TAB>is_current(0/1)<TAB>mtime`.
2. Resolve the target:
   - If the argument above is empty: the row with `is_current` = `1`. If none, tell the user you can't detect the current session and stop.
   - Else: rows whose title or id contains the argument, case-insensitively.
3. Zero matches: reply exactly `No session found matching "$ARGUMENTS"` and stop.
4. Multiple matches: `AskUserQuestion` with one option per match (label = title, description = id), single-select. Stop if they don't pick one.
5. Confirm with `AskUserQuestion` (Yes/No): "Delete session '<title>' (<id>)?". Stop without running anything else if the answer is anything but yes.
6. On yes, run exactly: `claude-delete-session --id <id> --yes --no-resume` (one Bash call).
7. Report the result in one line. If they want to switch sessions now, tell them to run `/resume` — this command does not relaunch anything by itself.
