// Command claude-delete-session deletes a Claude Code session transcript
// for the current project directory, then hands off to `claude --resume`.
//
// It runs entirely outside Claude Code: no API calls, no tokens spent.
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type session struct {
	ID      string
	Title   string
	Path    string
	ModTime time.Time
}

type rawEntry struct {
	Type    string `json:"type"`
	AiTitle string `json:"aiTitle"`
	Message *struct {
		Content json.RawMessage `json:"content"`
	} `json:"message"`
}

// flags for the non-interactive mode used by the /delete-session command
// running inside Claude Code, where there is no real tty to prompt on.
type flags struct {
	list     bool
	id       string
	yes      bool
	noResume bool
	query    string
}

func parseFlags(args []string) flags {
	var f flags
	var query []string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--list":
			f.list = true
		case "--yes", "-y":
			f.yes = true
		case "--no-resume":
			f.noResume = true
		case "--id":
			i++
			if i < len(args) {
				f.id = args[i]
			}
		default:
			query = append(query, args[i])
		}
	}
	f.query = strings.Join(query, " ")
	return f
}

func main() {
	f := parseFlags(os.Args[1:])

	home, err := os.UserHomeDir()
	if err != nil {
		fatal("could not determine home directory: %v", err)
	}
	cwd, err := os.Getwd()
	if err != nil {
		fatal("could not determine current directory: %v", err)
	}

	encoded := strings.ReplaceAll(cwd, "/", "-")
	projectDir := filepath.Join(home, ".claude", "projects", encoded)

	sessions, err := loadSessions(projectDir)
	if err != nil {
		fmt.Printf("No sessions found for this project (%s does not exist).\n", projectDir)
		os.Exit(1)
	}
	if len(sessions) == 0 {
		fmt.Println("No sessions found for this project.")
		os.Exit(1)
	}

	currentID := os.Getenv("CLAUDE_CODE_SESSION_ID")

	// --list: machine-readable dump, no prompts, no deletion. Used by the
	// /delete-session command to resolve a target without spawning a shell
	// prompt Claude Code can't answer (it has no attached tty).
	if f.list {
		for _, s := range sessions {
			current := "0"
			if currentID != "" && s.ID == currentID {
				current = "1"
			}
			fmt.Printf("%s\t%s\t%s\t%d\n", s.ID, s.Title, current, s.ModTime.Unix())
		}
		return
	}

	// --id: delete a specific, already-resolved session non-interactively.
	// Used by the /delete-session command after it has picked a target and
	// confirmed with the user itself (via AskUserQuestion).
	if f.id != "" {
		var target *session
		for i := range sessions {
			if sessions[i].ID == f.id {
				target = &sessions[i]
				break
			}
		}
		if target == nil {
			fatal("session id %q not found", f.id)
		}
		if !f.yes {
			prompt := fmt.Sprintf("Delete session %q (%s)? (y/N) ", target.Title, target.Path)
			answer, _ := promptLine(prompt)
			if !strings.EqualFold(answer, "y") && !strings.EqualFold(answer, "yes") {
				fmt.Println("Cancelled.")
				return
			}
		}
		deleteAndMaybeResume(*target, f.noResume)
		return
	}

	matches := filterSessions(sessions, f.query)
	if len(matches) == 0 {
		fmt.Printf("No session found matching %q\n", f.query)
		os.Exit(1)
	}

	var chosen session
	if len(matches) == 1 && f.query != "" {
		chosen = matches[0]
	} else {
		idx, ok := pickSession(matches, currentID)
		if !ok {
			fmt.Println("Cancelled.")
			return
		}
		chosen = matches[idx]
	}

	prompt := fmt.Sprintf("Delete session %q (%s)? (y/N) ", chosen.Title, chosen.Path)
	answer, _ := promptLine(prompt)
	if !strings.EqualFold(answer, "y") && !strings.EqualFold(answer, "yes") {
		fmt.Println("Cancelled.")
		return
	}

	deleteAndMaybeResume(chosen, f.noResume)
}

func deleteAndMaybeResume(target session, noResume bool) {
	if err := os.Remove(target.Path); err != nil {
		fatal("could not delete session: %v", err)
	}
	fmt.Println("Session deleted.")

	if noResume {
		return
	}

	claudePath, err := exec.LookPath("claude")
	if err != nil {
		fmt.Println("'claude' not found on PATH, skipping resume.")
		return
	}

	argv := []string{"claude", "--resume"}
	if err := syscall.Exec(claudePath, argv, os.Environ()); err != nil {
		fatal("could not launch claude --resume: %v", err)
	}
}

func loadSessions(dir string) ([]session, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var sessions []session
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".jsonl") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		info, err := entry.Info()
		if err != nil {
			continue
		}
		sessions = append(sessions, session{
			ID:      strings.TrimSuffix(entry.Name(), ".jsonl"),
			Title:   titleFromFile(path),
			Path:    path,
			ModTime: info.ModTime(),
		})
	}

	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].ModTime.After(sessions[j].ModTime)
	})
	return sessions, nil
}

func titleFromFile(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return "(untitled)"
	}
	defer f.Close()

	var title string
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		var e rawEntry
		if err := json.Unmarshal(scanner.Bytes(), &e); err != nil {
			continue
		}
		switch e.Type {
		case "ai-title":
			if e.AiTitle != "" {
				title = e.AiTitle
			}
		case "user":
			if title == "" && e.Message != nil {
				if t := firstUserText(e.Message.Content); t != "" {
					title = truncate(t, 60)
				}
			}
		}
	}

	if title == "" {
		return "(untitled)"
	}
	return title
}

func firstUserText(content json.RawMessage) string {
	if len(content) == 0 {
		return ""
	}

	var asString string
	if err := json.Unmarshal(content, &asString); err == nil {
		return asString
	}

	var asBlocks []struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(content, &asBlocks); err == nil {
		for _, b := range asBlocks {
			if b.Text != "" {
				return b.Text
			}
		}
	}
	return ""
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

func filterSessions(sessions []session, query string) []session {
	if query == "" {
		return sessions
	}
	lowQuery := strings.ToLower(query)
	var out []session
	for _, s := range sessions {
		if strings.Contains(strings.ToLower(s.Title), lowQuery) || strings.Contains(s.ID, query) {
			out = append(out, s)
		}
	}
	return out
}

func pickSession(sessions []session, currentID string) (int, bool) {
	fmt.Println("Sessions for this project:")
	for i, s := range sessions {
		mark := ""
		if currentID != "" && s.ID == currentID {
			mark = " (current)"
		}
		fmt.Printf("  %d. %s [%s]%s\n", i+1, s.Title, s.ID, mark)
	}

	prompt := fmt.Sprintf("Select a session to delete (1-%d, or Enter to cancel): ", len(sessions))
	answer, err := promptLine(prompt)
	if err != nil || answer == "" {
		return 0, false
	}

	choice, err := strconv.Atoi(answer)
	if err != nil || choice < 1 || choice > len(sessions) {
		fmt.Println("Invalid selection, aborting.")
		os.Exit(1)
	}
	return choice - 1, true
}

func promptLine(prompt string) (string, error) {
	fmt.Print(prompt)
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	line = strings.TrimSpace(line)
	if err != nil && line == "" {
		return "", err
	}
	return line, nil
}

func fatal(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
