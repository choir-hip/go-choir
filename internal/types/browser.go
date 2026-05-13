package types

import "time"

const (
	BrowserSessionIdle        = "idle"
	BrowserSessionReady       = "ready"
	BrowserSessionUnavailable = "unavailable"
	BrowserSessionError       = "error"
	BrowserSessionClosed      = "closed"
)

// BrowserLink is a link extracted from a backend browser snapshot.
type BrowserLink struct {
	URL  string `json:"url"`
	Text string `json:"text,omitempty"`
}

// BrowserSessionRecord is an owner-scoped backend browser session.
// The first implementations store text, HTML source, and link snapshots from a
// backend provider; later slices can attach screenshots, input events, and VM
// identity.
type BrowserSessionRecord struct {
	SessionID        string        `json:"session_id"`
	OwnerID          string        `json:"owner_id"`
	Provider         string        `json:"provider"`
	Mode             string        `json:"mode"`
	ExecutionScope   string        `json:"execution_scope,omitempty"`
	BackendSessionID string        `json:"backend_session_id,omitempty"`
	WorldKind        string        `json:"world_kind,omitempty"`
	CandidateID      string        `json:"promotion_candidate_id,omitempty"`
	VMID             string        `json:"vm_id,omitempty"`
	SnapshotID       string        `json:"snapshot_id,omitempty"`
	SourceRunID      string        `json:"source_loop_id,omitempty"`
	CandidateTraceID string        `json:"candidate_trace_id,omitempty"`
	State            string        `json:"state"`
	CurrentURL       string        `json:"current_url,omitempty"`
	Title            string        `json:"title,omitempty"`
	TextSnapshot     string        `json:"text_snapshot,omitempty"`
	HTMLSnapshot     string        `json:"html_snapshot,omitempty"`
	Links            []BrowserLink `json:"links,omitempty"`
	ScreenshotPNG    string        `json:"screenshot_png_base64,omitempty"`
	Error            string        `json:"error,omitempty"`
	CreatedAt        time.Time     `json:"created_at"`
	UpdatedAt        time.Time     `json:"updated_at"`
}
