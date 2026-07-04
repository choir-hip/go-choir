package computerversion

import (
	"context"
	"fmt"

	"github.com/yusefmosiah/go-choir/internal/base/blob"
	"github.com/yusefmosiah/go-choir/internal/base/journal"
)

// BaseCurrentStatePaths names existing Base persistence roots for read-only
// observation loading. Both paths must already exist; this boundary does not
// create product state.
type BaseCurrentStatePaths struct {
	JournalPath string `json:"journal_path"`
	BlobRoot    string `json:"blob_root"`
}

// BaseCurrentStateSource owns read-only handles for a scoped Base current-state
// observation source.
type BaseCurrentStateSource struct {
	journal *journal.SQLiteJournal
	blobs   *blob.Store
}

// OpenBaseCurrentStateSource opens existing Base persistence paths for
// read-only observation. It does not apply journal schema migrations or create a
// missing blob root.
func OpenBaseCurrentStateSource(paths BaseCurrentStatePaths) (*BaseCurrentStateSource, error) {
	if paths.JournalPath == "" {
		return nil, fmt.Errorf("base current state source: journal path is required")
	}
	if paths.BlobRoot == "" {
		return nil, fmt.Errorf("base current state source: blob root is required")
	}
	jr, err := journal.OpenSQLiteJournalReadOnly(paths.JournalPath)
	if err != nil {
		return nil, fmt.Errorf("base current state source: open journal: %w", err)
	}
	blobs, err := blob.OpenStore(paths.BlobRoot)
	if err != nil {
		_ = jr.Close()
		return nil, fmt.Errorf("base current state source: open blob store: %w", err)
	}
	return &BaseCurrentStateSource{journal: jr, blobs: blobs}, nil
}

// Close releases the read-only journal handle.
func (s *BaseCurrentStateSource) Close() error {
	if s == nil || s.journal == nil {
		return nil
	}
	return s.journal.Close()
}

// ObservationSet loads a scoped Base current-state ObservationSet from the
// opened read-only source.
func (s *BaseCurrentStateSource) ObservationSet(ctx context.Context, name string, version ComputerVersion) (ObservationSet, error) {
	if s == nil || s.journal == nil || s.blobs == nil {
		return ObservationSet{}, fmt.Errorf("base current state source: not open")
	}
	return BaseCurrentStateObservationSet(ctx, name, version, s.journal, s.blobs)
}
