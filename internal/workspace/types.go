package workspace

import "time"

// Workspace is the combined API type returned by all Manager methods.
// It merges store metadata with fields derived live from git and tmux.
type Workspace struct {
	// Stable identity and metadata — persisted in the store.
	ID              string
	Name            string
	CreatedAt       time.Time
	LastCaptureHash string
	LastChangedAt   time.Time
	Meta            map[string]string

	// Derived at query time — not persisted.
	WorktreePath string
	TmuxSession  string
	Branch       string
}
