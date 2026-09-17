package agents

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"syscall"
	"time"
)

type openCode struct{}

// NewOpenCode returns the OpenCode provider. It shells out to opencode's
// own `session list`/`session delete` commands instead of touching its
// sqlite database directly.
func NewOpenCode() Provider { return openCode{} }

func (openCode) Name() string { return "OpenCode" }
func (openCode) Icon() string { return "▦" }

type openCodeSession struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Updated   int64  `json:"updated"`
	Created   int64  `json:"created"`
	Directory string `json:"directory"`
}

func (openCode) List(dir string) ([]Session, error) {
	bin, err := exec.LookPath("opencode")
	if err != nil {
		return nil, nil
	}

	cmd := exec.Command(bin, "session", "list", "--format", "json")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return nil, nil
	}

	var raw []openCodeSession
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, nil
	}

	sessions := make([]Session, 0, len(raw))
	for _, r := range raw {
		if r.Directory != "" && r.Directory != dir {
			continue
		}
		title := r.Title
		if title == "" {
			title = "(untitled)"
		}
		sessions = append(sessions, Session{
			ID:        r.ID,
			Title:     title,
			Directory: dir,
			CreatedAt: msToTime(r.Created),
			UpdatedAt: msToTime(r.Updated),
			Provider:  openCode{},
		})
	}

	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].UpdatedAt.After(sessions[j].UpdatedAt)
	})
	return sessions, nil
}

func msToTime(ms int64) time.Time {
	if ms <= 0 {
		return time.Time{}
	}
	return time.UnixMilli(ms)
}

func (openCode) Delete(s Session) error {
	bin, err := exec.LookPath("opencode")
	if err != nil {
		return fmt.Errorf("'opencode' not found on PATH")
	}
	cmd := exec.Command(bin, "session", "delete", s.ID)
	cmd.Dir = s.Directory
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("opencode session delete: %w: %s", err, out)
	}
	return nil
}

func (openCode) Resume(s Session) error {
	bin, err := exec.LookPath("opencode")
	if err != nil {
		return fmt.Errorf("'opencode' not found on PATH")
	}
	if err := os.Chdir(s.Directory); err != nil {
		return err
	}
	return syscall.Exec(bin, []string{"opencode", "--session", s.ID}, os.Environ())
}

func (openCode) Relaunch(dir string) error {
	bin, err := exec.LookPath("opencode")
	if err != nil {
		return fmt.Errorf("'opencode' not found on PATH")
	}
	if err := os.Chdir(dir); err != nil {
		return err
	}
	return syscall.Exec(bin, []string{"opencode"}, os.Environ())
}
