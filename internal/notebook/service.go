package notebook

import (
	"context"
	"time"
)

// Revision is one revision of the notebook as the desk sees it.
type Revision struct {
	RevisionID  string    `json:"revision_id"`
	ParentID    string    `json:"parent_revision_id,omitempty"`
	AuthorKind  string    `json:"author_kind"`  // user or appagent
	AuthorLabel string    `json:"author_label"` // app:autoradio for the desk's own
	At          time.Time `json:"at"`
	Text        string    `json:"text"`
}

// DeskLabel is the AuthorLabel of a revision the desk's patch wrote.
const DeskLabel = "app:autoradio"

// IsDesk reports whether a revision is the desk's own write; any other is the
// listener's (design 2.3).
func (r Revision) IsDesk() bool { return r.AuthorKind == "appagent" && r.AuthorLabel == DeskLabel }

// State is what Texture answers to a read: the head and the revisions from the
// newest desk revision to the head, newest first (the desk revision last).
type State struct {
	Found bool `json:"found"`
	// Archived: the listener archived their notebook. Nothing is read from it
	// or written to it; archiving is the owner's act and so is undoing it.
	Archived bool       `json:"archived,omitempty"`
	DocID    string     `json:"doc_id,omitempty"`
	Head     Revision   `json:"head"`
	Chain    []Revision `json:"chain,omitempty"`
	// ChainCut: the chain was read to its cap without reaching a desk
	// revision, so the listener's edits cannot be compared with the desk's.
	ChainCut bool `json:"chain_cut,omitempty"`
}

// Result is Texture's answer to a patch: applied, stale, refused with a
// reason, or noop (the patch changed nothing). It is the settlement of the
// patch record's forecast by rule.
type Result struct {
	Status     string `json:"status"`
	Reason     string `json:"reason,omitempty"`
	DocID      string `json:"doc_id,omitempty"`
	RevisionID string `json:"revision_id,omitempty"`
	Words      int    `json:"words,omitempty"`
}

// Service is Texture's side of the two grants. The caller names the grant it
// holds; Texture refuses a caller the grant table does not name.
type Service interface {
	NotebookRead(ctx context.Context, ownerID, caller string) (State, error)
	NotebookApply(ctx context.Context, ownerID, caller string, p Patch) (Result, error)
}
