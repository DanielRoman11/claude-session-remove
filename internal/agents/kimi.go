package agents

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

// kimiCode talks to Kimi Code CLI's on-disk session store directly: there is
// no documented `kimi session delete` command, and the exact field names in
// state.json aren't confirmed against a live install (Kimi Code wasn't
// available to test against while writing this), so field lookups below
// are done by substring match rather than an exact key name, and every
// failure mode here falls back to "no sessions" instead of an error.
type kimiCode struct{}

// NewKimiCode returns the Kimi Code CLI provider.
func NewKimiCode() Provider { return kimiCode{} }

func (kimiCode) Name() string { return "Kimi Code" }
func (kimiCode) Icon() string { return "☾" }

func kimiHome() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	if h := os.Getenv("KIMI_CODE_HOME"); h != "" {
		return h, nil
	}
	return filepath.Join(home, ".kimi-code"), nil
}

type kimiIndexEntry struct {
	SessionID  string `json:"sessionId"`
	SessionDir string `json:"sessionDir"`
	WorkDir    string `json:"workDir"`
}

func (kimiCode) List(dir string) ([]Session, error) {
	home, err := kimiHome()
	if err != nil {
		return nil, nil
	}

	f, err := os.Open(filepath.Join(home, "session_index.jsonl"))
	if err != nil {
		return nil, nil
	}
	defer f.Close()

	var sessions []Session
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		var e kimiIndexEntry
		if err := json.Unmarshal(scanner.Bytes(), &e); err != nil {
			continue
		}
		if e.WorkDir != dir || e.SessionID == "" || e.SessionDir == "" {
			continue
		}

		title, created, updated := kimiSessionMeta(e.SessionDir)
		sessions = append(sessions, Session{
			ID:        e.SessionID,
			Title:     title,
			Directory: dir,
			CreatedAt: created,
			UpdatedAt: updated,
			Provider:  kimiCode{},
		})
	}

	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].UpdatedAt.After(sessions[j].UpdatedAt)
	})
	return sessions, nil
}

func kimiSessionMeta(sessionDir string) (title string, created, updated time.Time) {
	statePath := filepath.Join(sessionDir, "state.json")
	data, err := os.ReadFile(statePath)
	if err != nil {
		if info, statErr := os.Stat(sessionDir); statErr == nil {
			return "(untitled)", info.ModTime(), info.ModTime()
		}
		return "(untitled)", time.Time{}, time.Time{}
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		info, _ := os.Stat(statePath)
		if info != nil {
			return "(untitled)", info.ModTime(), info.ModTime()
		}
		return "(untitled)", time.Time{}, time.Time{}
	}

	title = firstStringField(raw, "title")
	if title == "" {
		title = "(untitled)"
	}
	if t, ok := firstTimeField(raw, "creat"); ok {
		created = t
	}
	if t, ok := firstTimeField(raw, "updat"); ok {
		updated = t
	}

	info, _ := os.Stat(statePath)
	if created.IsZero() && info != nil {
		created = info.ModTime()
	}
	if updated.IsZero() && info != nil {
		updated = info.ModTime()
	}
	return title, created, updated
}

// firstStringField returns the value of the first key containing substr
// (case-insensitive) whose value is a non-empty string.
func firstStringField(m map[string]interface{}, substr string) string {
	for k, v := range m {
		if !strings.Contains(strings.ToLower(k), substr) {
			continue
		}
		if s, ok := v.(string); ok && s != "" {
			return s
		}
	}
	return ""
}

// firstTimeField returns the first value under a key containing substr that
// parses as a timestamp, trying RFC3339 strings and unix seconds/millis.
func firstTimeField(m map[string]interface{}, substr string) (time.Time, bool) {
	for k, v := range m {
		if !strings.Contains(strings.ToLower(k), substr) {
			continue
		}
		if t, ok := parseAnyTime(v); ok {
			return t, true
		}
	}
	return time.Time{}, false
}

func parseAnyTime(v interface{}) (time.Time, bool) {
	switch val := v.(type) {
	case string:
		if t, err := time.Parse(time.RFC3339, val); err == nil {
			return t, true
		}
		if n, err := strconv.ParseInt(val, 10, 64); err == nil {
			return unixGuess(n), true
		}
	case float64:
		return unixGuess(int64(val)), true
	}
	return time.Time{}, false
}

// unixGuess treats large values as milliseconds, smaller ones as seconds.
func unixGuess(n int64) time.Time {
	if n > 1e12 {
		return time.UnixMilli(n)
	}
	return time.Unix(n, 0)
}

func (kimiCode) Delete(s Session) error {
	home, err := kimiHome()
	if err != nil {
		return err
	}
	f, err := os.Open(filepath.Join(home, "session_index.jsonl"))
	if err != nil {
		return err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		var e kimiIndexEntry
		if err := json.Unmarshal(scanner.Bytes(), &e); err != nil {
			continue
		}
		if e.SessionID == s.ID {
			return os.RemoveAll(e.SessionDir)
		}
	}
	return fmt.Errorf("session %q not found in %s", s.ID, filepath.Join(home, "session_index.jsonl"))
}

func (kimiCode) Resume(s Session) error {
	bin, err := exec.LookPath("kimi")
	if err != nil {
		return fmt.Errorf("'kimi' not found on PATH")
	}
	if err := os.Chdir(s.Directory); err != nil {
		return err
	}
	return syscall.Exec(bin, []string{"kimi", "--session", s.ID}, os.Environ())
}

func (kimiCode) Relaunch(dir string) error {
	bin, err := exec.LookPath("kimi")
	if err != nil {
		return fmt.Errorf("'kimi' not found on PATH")
	}
	if err := os.Chdir(dir); err != nil {
		return err
	}
	return syscall.Exec(bin, []string{"kimi", "--session"}, os.Environ())
}
