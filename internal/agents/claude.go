package agents

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
)

type claudeCode struct{}

// NewClaudeCode returns the Claude Code provider. Sessions are read from
// the .jsonl transcripts Claude Code itself writes under
// ~/.claude/projects/<encoded-cwd>/.
func NewClaudeCode() Provider { return claudeCode{} }

func (claudeCode) Name() string { return "Claude Code" }
func (claudeCode) Icon() string { return "✳" }

type claudeEntry struct {
	Type      string `json:"type"`
	AiTitle   string `json:"aiTitle"`
	Timestamp string `json:"timestamp"`
	Message   *struct {
		Content json.RawMessage `json:"content"`
	} `json:"message"`
}

func (claudeCode) List(dir string) ([]Session, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, nil
	}
	encoded := strings.ReplaceAll(dir, "/", "-")
	projectDir := filepath.Join(home, ".claude", "projects", encoded)

	entries, err := os.ReadDir(projectDir)
	if err != nil {
		return nil, nil
	}

	var sessions []Session
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".jsonl") {
			continue
		}
		path := filepath.Join(projectDir, entry.Name())
		info, err := entry.Info()
		if err != nil {
			continue
		}
		title, created, updated := claudeSessionMeta(path)
		if created.IsZero() {
			created = info.ModTime()
		}
		if updated.IsZero() {
			updated = info.ModTime()
		}
		sessions = append(sessions, Session{
			ID:        strings.TrimSuffix(entry.Name(), ".jsonl"),
			Title:     title,
			Directory: dir,
			CreatedAt: created,
			UpdatedAt: updated,
			Provider:  claudeCode{},
		})
	}

	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].UpdatedAt.After(sessions[j].UpdatedAt)
	})
	return sessions, nil
}

func claudeSessionMeta(path string) (title string, created, updated time.Time) {
	f, err := os.Open(path)
	if err != nil {
		return "(untitled)", time.Time{}, time.Time{}
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		var e claudeEntry
		if err := json.Unmarshal(scanner.Bytes(), &e); err != nil {
			continue
		}
		if e.Timestamp != "" {
			if t, err := time.Parse(time.RFC3339, e.Timestamp); err == nil {
				if created.IsZero() {
					created = t
				}
				updated = t
			}
		}
		switch e.Type {
		case "ai-title":
			if e.AiTitle != "" {
				title = e.AiTitle
			}
		case "user":
			if title == "" && e.Message != nil {
				if t := claudeFirstUserText(e.Message.Content); t != "" {
					title = claudeTruncate(t, 60)
				}
			}
		}
	}

	if title == "" {
		title = "(untitled)"
	}
	return title, created, updated
}

func claudeFirstUserText(content json.RawMessage) string {
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

func claudeTruncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

func (claudeCode) sessionPath(s Session) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	encoded := strings.ReplaceAll(s.Directory, "/", "-")
	return filepath.Join(home, ".claude", "projects", encoded, s.ID+".jsonl"), nil
}

func (c claudeCode) Delete(s Session) error {
	path, err := c.sessionPath(s)
	if err != nil {
		return err
	}
	return os.Remove(path)
}

func (claudeCode) Resume(s Session) error {
	claudePath, err := exec.LookPath("claude")
	if err != nil {
		return fmt.Errorf("'claude' not found on PATH")
	}
	if err := os.Chdir(s.Directory); err != nil {
		return err
	}
	return syscall.Exec(claudePath, []string{"claude", "--resume", s.ID}, os.Environ())
}

func (claudeCode) Relaunch(dir string) error {
	claudePath, err := exec.LookPath("claude")
	if err != nil {
		return fmt.Errorf("'claude' not found on PATH")
	}
	if err := os.Chdir(dir); err != nil {
		return err
	}
	return syscall.Exec(claudePath, []string{"claude", "--resume"}, os.Environ())
}
