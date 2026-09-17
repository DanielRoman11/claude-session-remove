---
allowed-tools: Bash(context-hub:*), AskUserQuestion
argument-hint: [session-name]
description: Delete a Claude Code session (current one, or matched by name) using the context-hub binary
---

## Argument

```
$ARGUMENTS
```

## Task

Do not reason about this beyond what's below. No exploring, no extra commands.

1. Run `context-hub --list` (one Bash call). Each line is `provider_slug<TAB>id<TAB>provider_name<TAB>title<TAB>is_current(0/1)<TAB>updated_unix`.
2. Resolve the target:
   - If the argument above is empty: the row with `is_current` = `1`. If none, tell the user you can't detect the current session and stop.
   - Else: rows whose title or id contains the argument, case-insensitively.
3. Zero matches: reply exactly `No session found matching "$ARGUMENTS"` and stop.
4. Multiple matches: `AskUserQuestion` with one option per match (label = `title`, description = `provider_name (id)`), single-select. Stop if they don't pick one.
5. Confirm with `AskUserQuestion` (Yes/No): "Delete session '<title>' [<provider_name>]?". Stop without running anything else if the answer is anything but yes.
6. On yes, run exactly: `context-hub --id <provider_slug>:<id> --yes --no-resume` (one Bash call).
7. Report the result in one line. If they want to switch sessions now, tell them to run `/resume` (for Claude Code) or reopen the relevant tool — this command does not relaunch anything by itself.
