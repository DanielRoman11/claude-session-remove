// Command csr (claude-session-remove) deletes a Claude Code session
// transcript for the current project directory, then hands off to
// `claude --resume`.
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
	"strings"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mattn/go-isatty"
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

// flags for the non-interactive mode used by the /csr command running
// inside Claude Code, where there is no real tty to drive a TUI on.
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

	// --list: machine-readable dump, no prompts, no TUI. Used by the /csr
	// command to resolve a target without driving a program Claude Code
	// can't attach a tty to.
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
	// Used by the /csr command after it has picked a target and confirmed
	// with the user itself (via AskUserQuestion).
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
		if !f.yes && !confirmPlain(*target) {
			fmt.Println("Cancelled.")
			return
		}
		deleteAndMaybeResume(*target, currentID, f.noResume)
		return
	}

	matches := filterSessions(sessions, f.query)
	if len(matches) == 0 {
		fmt.Printf("No session found matching %q\n", f.query)
		os.Exit(1)
	}

	var chosen session
	var ok bool
	if len(matches) == 1 && f.query != "" {
		chosen, ok = matches[0], true
		if !confirmPlain(chosen) {
			ok = false
		}
	} else if isatty.IsTerminal(os.Stdout.Fd()) && isatty.IsTerminal(os.Stdin.Fd()) {
		chosen, ok = runPicker(matches, currentID)
	} else {
		// No attached tty (piped/redirected): fall back to a plain,
		// numbered prompt instead of failing to start the TUI.
		chosen, ok = pickPlain(matches, currentID)
	}
	if !ok {
		fmt.Println("Cancelled.")
		return
	}

	deleteAndMaybeResume(chosen, currentID, f.noResume)
}

// deleteAndMaybeResume removes target's transcript and, only when it was
// NOT the currently active session, hands off to `claude --resume`.
//
// Chaining into --resume after deleting the *active* session is unsafe:
// `claude --resume` with no explicit id can fall back to the most recently
// active session, which is exactly the one just deleted, and Claude Code
// will happily recreate a blank transcript under that same id. So instead
// of undoing the deletion, we just tell the user to exit and start a fresh
// (non-resumed) `claude` themselves.
func deleteAndMaybeResume(target session, currentID string, noResume bool) {
	wasCurrent := currentID != "" && target.ID == currentID

	if err := os.Remove(target.Path); err != nil {
		fatal("could not delete session: %v", err)
	}
	fmt.Println(successStyle.Render("✓ Session deleted."))

	if wasCurrent {
		fmt.Println(dimStyle.Render("This was the active session. Exit this terminal (Ctrl-D) and start a plain 'claude' (not --resume) elsewhere — resuming right now could recreate it under the same id."))
		return
	}

	if noResume {
		return
	}

	claudePath, err := exec.LookPath("claude")
	if err != nil {
		fmt.Println(dimStyle.Render("'claude' not found on PATH, skipping resume."))
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

func relTime(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	case d < 30*24*time.Hour:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	default:
		return t.Format("Jan 2")
	}
}

// runPicker drives the Bubble Tea TUI and returns the chosen session, if any.
func runPicker(sessions []session, currentID string) (session, bool) {
	m := newModel(sessions, currentID)
	p := tea.NewProgram(m, tea.WithAltScreen())
	final, err := p.Run()
	if err != nil {
		fatal("could not start interface: %v", err)
	}
	res := final.(model)
	if !res.confirmed {
		return session{}, false
	}
	return res.target, true
}

func confirmPlain(target session) bool {
	prompt := fmt.Sprintf("Delete session %q (%s)? (y/N) ", target.Title, target.Path)
	answer, _ := promptLine(prompt)
	return strings.EqualFold(answer, "y") || strings.EqualFold(answer, "yes")
}

func pickPlain(sessions []session, currentID string) (session, bool) {
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
		return session{}, false
	}

	var choice int
	if _, err := fmt.Sscanf(answer, "%d", &choice); err != nil || choice < 1 || choice > len(sessions) {
		fmt.Println("Invalid selection, aborting.")
		os.Exit(1)
	}
	chosen := sessions[choice-1]
	if !confirmPlain(chosen) {
		return session{}, false
	}
	return chosen, true
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
