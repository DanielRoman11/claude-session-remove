---
allowed-tools: Bash(ls:*), Bash(cat:*), Bash(pwd), Bash(rm:*), Bash(python3:*), AskUserQuestion
argument-hint: [session-name]
description: Delete a Claude Code session transcript (the current one, or one matched by name), after confirmation
---

## Context

- Current working directory: !`pwd`
- Project session directory: !`echo ~/.claude/projects/$(pwd | tr '/' '-')`
- Current session id (this conversation): !`echo "${CLAUDE_CODE_SESSION_ID:-unknown}"`
- Session files in this project: !`ls -la ~/.claude/projects/$(pwd | tr '/' '-')/*.jsonl 2>/dev/null || echo "(none found)"`

## Argument

The text the user typed after `/delete-session` (may be empty, meaning "delete the current session"):

```
$ARGUMENTS
```

## Your task

Follow these steps precisely. Do not delete anything without an explicit "yes" from the user.

1. **Resolve the project session directory** using the context above (`~/.claude/projects/<cwd with "/" replaced by "-">`). If it doesn't exist or has no `.jsonl` files, report "No sessions found for this project." and stop.

2. **Build a name → session-id map.** For every `*.jsonl` file in that directory, derive a human-readable title:
   - Read the file and use the `aiTitle` value from the *last* line of type `"ai-title"`, if any.
   - Otherwise, fall back to the text of the first `"user"` message in the file, truncated to ~60 chars.
   - The session id is the file's basename without `.jsonl`.

   A one-liner like this works well:
   ```bash
   python3 - "$DIR" <<'EOF'
   import json, sys, pathlib
   d = pathlib.Path(sys.argv[1])
   for f in sorted(d.glob("*.jsonl"), key=lambda p: p.stat().st_mtime, reverse=True):
       title, sid = None, f.stem
       with f.open() as fh:
           for line in fh:
               try:
                   obj = json.loads(line)
               except ValueError:
                   continue
               if obj.get("type") == "ai-title":
                   title = obj.get("aiTitle")
               elif title is None and obj.get("type") == "user":
                   msg = obj.get("message", {})
                   content = msg.get("content")
                   if isinstance(content, str):
                       title = content[:60]
                   elif isinstance(content, list) and content and isinstance(content[0], dict):
                       title = str(content[0].get("text", ""))[:60]
       print(f"{sid}\t{title or '(untitled)'}\t{f.stat().st_mtime}")
   EOF
   ```

3. **Determine the target session(s):**
   - If `$ARGUMENTS` is empty: the target is the current session id from the context above. If it is `unknown` (e.g. not running inside a live Claude Code session), tell the user you can't determine the current session and ask them to pass a session name instead, then stop.
   - If `$ARGUMENTS` is non-empty: case-insensitively match it as a substring against each session's title, and also against the raw session id (so users can paste a full or partial id). Collect all matches.

4. **Handle the match count:**
   - **Zero matches:** report exactly `No session found matching "$ARGUMENTS"` and stop.
   - **Multiple matches:** use `AskUserQuestion` to show a numbered list (title + session id prefix + last-modified date) and let the user pick exactly one. If they decline/cancel, stop without changes.
   - **Exactly one match:** proceed with it.

5. **Confirm before deleting.** Use `AskUserQuestion` (or a plain yes/no if that tool isn't available) showing the session's title, id, and file path, e.g. "Delete session '<title>' (<file>)?". If the answer is no, reply that the deletion was cancelled and stop — do not touch any files.

6. **Delete on confirmation:** remove only that session's `.jsonl` file with `rm -- "<path>"`. Never use wildcards or delete more than the confirmed file.

7. **Report the outcome:**
   - If the deleted session was the *current* one, tell the user the transcript file is gone and that they should exit this session (e.g. `/exit` or Ctrl-D) and start a new `claude` process to fully leave it — a running session cannot restart itself mid-conversation.
   - Otherwise, confirm the session was deleted and that the current conversation is unaffected.

## Notes

- This only removes the session's transcript (`.jsonl`) file under `~/.claude/projects/<project>/`. It does not touch unrelated Claude Code state.
- Never delete a file you have not shown to and had confirmed by the user in this same invocation.
