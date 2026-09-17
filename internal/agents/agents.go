// Package agents defines the provider interface context-hub uses to list,
// delete, and resume sessions across different AI coding CLIs.
package agents

import "time"

// Session is one saved conversation from any provider.
type Session struct {
	ID        string
	Title     string
	Directory string
	CreatedAt time.Time
	UpdatedAt time.Time
	Provider  Provider
}

// Provider knows how to find, remove, and jump back into a given AI CLI's
// sessions for a specific working directory.
type Provider interface {
	// Name is the display name, e.g. "Claude Code".
	Name() string
	// Icon is a short glyph shown next to each of this provider's sessions.
	Icon() string
	// List returns this provider's sessions scoped to dir, most recent
	// first. A provider whose CLI isn't installed, or that has no
	// sessions for dir, returns (nil, nil) rather than an error.
	List(dir string) ([]Session, error)
	// Delete permanently removes a session.
	Delete(s Session) error
	// Resume execs into this exact session, replacing the current
	// process. It does not return on success.
	Resume(s Session) error
	// Relaunch execs a bare instance of the underlying tool in dir,
	// replacing the current process. Used after a deletion to hand off
	// into whatever picker/continuation UI that tool offers on its own.
	Relaunch(dir string) error
}

// All returns every provider context-hub knows about.
func All() []Provider {
	return []Provider{
		NewClaudeCode(),
		NewOpenCode(),
		NewKimiCode(),
	}
}
