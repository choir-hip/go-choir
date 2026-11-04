package store

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
)

// ogKindEngineeringLineage is an app's engineering lineage: the working
// context the app's last engineer left, one object per (computer, app)
// (docs/design/engineer-reuse-2026-11-04.md, phase B). A later request about
// the app starts its new engineer from it. It is advice, read into a new
// assignment's run context; nothing resumes work from it.
const ogKindEngineeringLineage = objectgraph.ObjectKind("choir.engineering_lineage")

// EngineeringLineage is the handoff one engineering assignment left for
// one app it changed. The operation's state is not copied: a reader asks
// the operation store.
type EngineeringLineage struct {
	OwnerID      string `json:"owner_id"`
	ComputerID   string `json:"computer_id"`
	App          string `json:"app"`
	AssignmentID string `json:"assignment_id"`
	Attempt      uint64 `json:"attempt"`
	// RunID is the engineer's run: its trace on the tape is the full
	// transcript.
	RunID        string `json:"run_id"`
	TrajectoryID string `json:"trajectory_id"`
	OperationID  string `json:"operation_id,omitempty"`
	BundleDigest string `json:"bundle_digest,omitempty"`
	BaseCommit   string `json:"base_commit,omitempty"`
	// Result is the assignment's reported result (completed, failed...).
	Result string `json:"result"`
	// Files are the paths under the app the candidate changed.
	Files []string `json:"files"`
	// Summary is the engineer's own Complete summary, bounded; it ends with
	// the engineer's Handoff paragraph when the engineer wrote one.
	Summary    string    `json:"summary"`
	RecordedAt time.Time `json:"recorded_at"`
}

func engineeringLineageKey(computerID, app string) string {
	return strings.TrimSpace(computerID) + "\x00" + strings.ToLower(strings.TrimSpace(app))
}

// PutEngineeringLineage records the app's lineage, replacing the one an
// earlier assignment left.
func (s *Store) PutEngineeringLineage(ctx context.Context, lineage EngineeringLineage) error {
	lineage.OwnerID, lineage.ComputerID = strings.TrimSpace(lineage.OwnerID), strings.TrimSpace(lineage.ComputerID)
	lineage.App = strings.ToLower(strings.TrimSpace(lineage.App))
	if lineage.OwnerID == "" || lineage.ComputerID == "" || lineage.App == "" || strings.TrimSpace(lineage.AssignmentID) == "" {
		return fmt.Errorf("engineering lineage: owner_id, computer_id, app and assignment_id are required")
	}
	if lineage.RecordedAt.IsZero() {
		lineage.RecordedAt = time.Now().UTC()
	}
	metadata := map[string]any{
		"computer_id":   lineage.ComputerID,
		"app":           lineage.App,
		"assignment_id": strings.TrimSpace(lineage.AssignmentID),
		"recorded_at":   lineage.RecordedAt.UTC().Format(time.RFC3339Nano),
	}
	_, err := s.ogPut(ctx, ogKindEngineeringLineage, lineage.OwnerID, engineeringLineageKey(lineage.ComputerID, lineage.App), lineage, metadata, lineage.RecordedAt)
	return err
}

// GetEngineeringLineage returns one app's lineage, or ErrNotFound.
func (s *Store) GetEngineeringLineage(ctx context.Context, ownerID, computerID, app string) (EngineeringLineage, error) {
	if s.og == nil {
		return EngineeringLineage{}, fmt.Errorf("store: object graph not initialized")
	}
	id, err := objectgraph.BuildCanonicalID(ogKindEngineeringLineage, strings.TrimSpace(ownerID), objectgraph.StableSuffixFromKey(engineeringLineageKey(computerID, app)))
	if err != nil {
		return EngineeringLineage{}, err
	}
	obj, err := s.og.GetObject(ctx, id)
	if errors.Is(err, objectgraph.ErrNotFound) {
		return EngineeringLineage{}, ErrNotFound
	}
	if err != nil {
		return EngineeringLineage{}, err
	}
	var lineage EngineeringLineage
	if err := ogDecode(obj, &lineage); err != nil {
		return EngineeringLineage{}, err
	}
	return lineage, nil
}

// ListEngineeringLineageApps returns the apps of one computer that have a
// lineage, sorted.
func (s *Store) ListEngineeringLineageApps(ctx context.Context, ownerID, computerID string) ([]string, error) {
	objs, err := s.ogListAllByMetadata(ctx, ogKindEngineeringLineage, "computer_id", strings.TrimSpace(computerID))
	if err != nil {
		return nil, err
	}
	var apps []string
	for _, obj := range objs {
		if obj.OwnerID != strings.TrimSpace(ownerID) {
			continue
		}
		var lineage EngineeringLineage
		if ogDecode(obj, &lineage) != nil || lineage.App == "" {
			continue
		}
		apps = append(apps, lineage.App)
	}
	sort.Strings(apps)
	return apps, nil
}
