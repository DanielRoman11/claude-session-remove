// Command context-hub lists, deletes, and resumes AI coding CLI sessions
// (Claude Code, OpenCode, Kimi Code) for the current project directory.
//
// It runs entirely outside those tools: no API calls, no tokens spent.
package main

import (
	"bufio"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/DanielRoman11/context-hub/internal/agents"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/mattn/go-isatty"
)

// flags for the non-interactive mode used by the /context-hub command running
// inside Claude Code, where there is no real tty to drive a TUI on.
type flags struct {
	list     bool
	id       string // "<provider-slug>:<session-id>"
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

	cwd, err := os.Getwd()
	if err != nil {
		fatal("could not determine current directory: %v", err)
	}

	sessions := loadAll(cwd)
	if len(sessions) == 0 {
		fmt.Println("No sessions found for this project.")
		os.Exit(1)
	}

	currentID := os.Getenv("CLAUDE_CODE_SESSION_ID")

	// --list: machine-readable dump, no prompts, no TUI. Used by the
	// /context-hub command to resolve a target without driving a program
	// Claude Code can't attach a tty to.
	if f.list {
		for _, s := range sessions {
			current := "0"
			if isCurrent(s, currentID) {
				current = "1"
			}
			fmt.Printf("%s\t%s\t%s\t%s\t%s\t%d\n",
				providerSlug(s.Provider), s.ID, s.Provider.Name(), s.Title, current, s.UpdatedAt.Unix())
		}
		return
	}

	// --id: delete a specific, already-resolved session non-interactively.
	// Used by the /context-hub command after it has picked a target and
	// confirmed with the user itself (via AskUserQuestion).
	if f.id != "" {
		target, ok := findByComposite(sessions, f.id)
		if !ok {
			fatal("session %q not found", f.id)
		}
		if !f.yes && !confirmPlain(target) {
			fmt.Println("Cancelled.")
			return
		}
		deleteAndMaybeRelaunch(target, currentID, f.noResume)
		return
	}

	matches := filterSessions(sessions, f.query)
	if len(matches) == 0 {
		fmt.Printf("No session found matching %q\n", f.query)
		os.Exit(1)
	}

	if len(matches) == 1 && f.query != "" {
		target := matches[0]
		if !confirmPlain(target) {
			fmt.Println("Cancelled.")
			return
		}
		deleteAndMaybeRelaunch(target, currentID, f.noResume)
		return
	}

	var (
		action string
		target agents.Session
		ok     bool
	)
	if isatty.IsTerminal(os.Stdout.Fd()) && isatty.IsTerminal(os.Stdin.Fd()) {
		action, target, ok = runPicker(matches, currentID)
	} else {
		// No attached tty (piped/redirected): fall back to a plain,
		// numbered delete prompt instead of failing to start the TUI.
		action = "delete"
		target, ok = pickPlain(matches, currentID)
	}
	if !ok {
		fmt.Println("Cancelled.")
		return
	}

	switch action {
	case "resume":
		if err := target.Provider.Resume(target); err != nil {
			fatal("could not resume session: %v", err)
		}
	default:
		deleteAndMaybeRelaunch(target, currentID, f.noResume)
	}
}

func loadAll(cwd string) []agents.Session {
	var all []agents.Session
	for _, p := range agents.All() {
		sessions, err := p.List(cwd)
		if err != nil {
			continue
		}
		all = append(all, sessions...)
	}
	sort.Slice(all, func(i, j int) bool {
		return all[i].UpdatedAt.After(all[j].UpdatedAt)
	})
	return all
}

func isCurrent(s agents.Session, currentID string) bool {
	return currentID != "" && s.Provider.Name() == "Claude Code" && s.ID == currentID
}

func providerSlug(p agents.Provider) string {
	return strings.ToLower(strings.ReplaceAll(p.Name(), " ", "-"))
}

func findByComposite(sessions []agents.Session, composite string) (agents.Session, bool) {
	slug, id, found := strings.Cut(composite, ":")
	if !found {
		return agents.Session{}, false
	}
	for _, s := range sessions {
		if providerSlug(s.Provider) == slug && s.ID == id {
			return s, true
		}
	}
	return agents.Session{}, false
}

// deleteAndMaybeRelaunch removes target and, only when it was NOT the
// currently active Claude Code session, hands off into that provider's own
// picker/continuation UI.
//
// Chaining into a resume after deleting the *active* Claude Code session is
// unsafe: `claude --resume` with no explicit id can fall back to the most
// recently active session, which is exactly the one just deleted, and
// Claude Code will happily recreate a blank transcript under that same id.
// So instead of undoing the deletion, we just tell the user to exit and
// start fresh elsewhere.
func deleteAndMaybeRelaunch(target agents.Session, currentID string, noResume bool) {
	wasCurrent := isCurrent(target, currentID)

	if err := target.Provider.Delete(target); err != nil {
		fatal("could not delete session: %v", err)
	}
	fmt.Println(successStyle.Render("✓ Session deleted."))

	if wasCurrent {
		fmt.Println(dimStyle.Render("This was the active session. Exit this terminal (Ctrl-D) and start a fresh session elsewhere — resuming right now could recreate it under the same id."))
		return
	}
	if noResume {
		return
	}

	if err := target.Provider.Relaunch(target.Directory); err != nil {
		fmt.Println(dimStyle.Render(err.Error() + ", skipping relaunch."))
	}
}

func filterSessions(sessions []agents.Session, query string) []agents.Session {
	if query == "" {
		return sessions
	}
	lowQuery := strings.ToLower(query)
	var out []agents.Session
	for _, s := range sessions {
		if strings.Contains(strings.ToLower(s.Title), lowQuery) || strings.Contains(s.ID, query) {
			out = append(out, s)
		}
	}
	return out
}

// runPicker drives the Bubble Tea TUI and returns the chosen action/session.
func runPicker(sessions []agents.Session, currentID string) (string, agents.Session, bool) {
	m := newModel(sessions, currentID)
	p := tea.NewProgram(m, tea.WithAltScreen())
	final, err := p.Run()
	if err != nil {
		fatal("could not start interface: %v", err)
	}
	res := final.(model)
	if res.action == "" {
		return "", agents.Session{}, false
	}
	return res.action, res.target, true
}

func confirmPlain(target agents.Session) bool {
	prompt := fmt.Sprintf("Delete session %q [%s]? (y/N) ", target.Title, target.Provider.Name())
	answer, _ := promptLine(prompt)
	return strings.EqualFold(answer, "y") || strings.EqualFold(answer, "yes")
}

func pickPlain(sessions []agents.Session, currentID string) (agents.Session, bool) {
	fmt.Println("Sessions for this project:")
	for i, s := range sessions {
		mark := ""
		if isCurrent(s, currentID) {
			mark = " (current)"
		}
		fmt.Printf("  %d. [%s] %s%s\n", i+1, s.Provider.Name(), s.Title, mark)
	}

	prompt := fmt.Sprintf("Select a session to delete (1-%d, or Enter to cancel): ", len(sessions))
	answer, err := promptLine(prompt)
	if err != nil || answer == "" {
		return agents.Session{}, false
	}

	var choice int
	if _, err := fmt.Sscanf(answer, "%d", &choice); err != nil || choice < 1 || choice > len(sessions) {
		fmt.Println("Invalid selection, aborting.")
		os.Exit(1)
	}
	chosen := sessions[choice-1]
	if !confirmPlain(chosen) {
		return agents.Session{}, false
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
