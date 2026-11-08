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

// ogKindPackageInstallPending is one pending install: an object-graph object
// of the VM store, so it is an event projection like every other piece of
// state and replay stays eligible while a review waits (a SQL table outside
// the declared schema boundary would make the replay probe ineligible).
const ogKindPackageInstallPending = objectgraph.ObjectKind("choir.package_install_pending")

// package_install_pending holds an owner or update install that waits on
// management's install review (docs/design/desk-install-review-2026-11-08.md
// 2.1): the full request, so an accepting verdict re-runs it, keyed on the
// subject digest and the attempt. A row is written once per attempt, closed
// once by a compare-and-set, and pruned a month after it closes.

// States of a pending install row.
const (
	PackageInstallPendingOpen        = "pending"
	PackageInstallPendingAccepted    = "accepted" // an accepting verdict is sealed; the re-run is queued
	PackageInstallPendingInstalled   = "installed"
	PackageInstallPendingFailed      = "install_failed"
	PackageInstallPendingRejected    = "review_rejected"
	PackageInstallPendingTimedOut    = "review_timed_out"
	PackageInstallPendingInterrupted = "review_interrupted"
	PackageInstallPendingSuperseded  = "superseded"
)

// PackageInstallPendingRetention is how long a closed row is kept.
const PackageInstallPendingRetention = 30 * 24 * time.Hour

// PackageInstallPending is one install waiting on its review.
type PackageInstallPending struct {
	OwnerID       string    `json:"owner_id"`
	ComputerID    string    `json:"computer_id"`
	SubjectDigest string    `json:"subject_digest"`
	Attempt       int       `json:"attempt"`
	ReleaseID     string    `json:"release_id"`
	Package       string    `json:"package"`
	Action        string    `json:"action"`
	Record        []byte    `json:"record"`
	TreePatch     []byte    `json:"tree_patch"`
	Change        []byte    `json:"change"`
	RequestedAt   time.Time `json:"requested_at"`
	// BaselineRelease and BaselineAt are the install in force at pend time
	// (the supersession baseline).
	BaselineRelease string    `json:"baseline_release,omitempty"`
	BaselineAt      time.Time `json:"baseline_at,omitempty"`
	OwedRecordID    string    `json:"owed_record_id"`
	State           string    `json:"state"`
	Reason          string    `json:"reason,omitempty"`
	LastError       string    `json:"last_error,omitempty"`
	ClosedAt        time.Time `json:"closed_at,omitempty"`
}

// Open reports whether the row still waits on a verdict.
func (p PackageInstallPending) Open() bool { return p.State == PackageInstallPendingOpen }

func pendingKey(subjectDigest string, attempt int) string {
	return fmt.Sprintf("%s:a%d", strings.TrimSpace(subjectDigest), attempt)
}

func pendingMetadata(p PackageInstallPending) map[string]any {
	return map[string]any{
		"computer_id": p.ComputerID, "subject_digest": p.SubjectDigest, "attempt": p.Attempt, "release_id": p.ReleaseID,
		"package": p.Package, "state": p.State, "requested_at": p.RequestedAt.UTC().Format(time.RFC3339Nano),
	}
}

func pendingObject(p PackageInstallPending, createdAt time.Time) (objectgraph.Object, error) {
	return lifecycleObject(ogKindPackageInstallPending, p.OwnerID, p.ComputerID, pendingKey(p.SubjectDigest, p.Attempt), p, pendingMetadata(p), createdAt, time.Now().UTC())
}

func decodePending(obj objectgraph.Object) (PackageInstallPending, error) {
	return decodeLifecycleObject[PackageInstallPending](obj)
}

// PutPackageInstallPending writes a new pending row. It never replaces one: a
// subject digest and attempt are written once.
func (s *Store) PutPackageInstallPending(ctx context.Context, p PackageInstallPending) error {
	if strings.TrimSpace(p.OwnerID) == "" || strings.TrimSpace(p.ComputerID) == "" || strings.TrimSpace(p.SubjectDigest) == "" ||
		strings.TrimSpace(p.ReleaseID) == "" || p.Attempt < 1 {
		return fmt.Errorf("package install pending: owner, computer, subject, release and attempt are required")
	}
	if int64(len(p.TreePatch)) > pendingMaxBlob {
		return fmt.Errorf("package install pending: the tree patch is %d bytes, over %d", len(p.TreePatch), pendingMaxBlob)
	}
	if s.ogStore == nil {
		return fmt.Errorf("package install pending: object graph not initialized")
	}
	if p.State == "" {
		p.State = PackageInstallPendingOpen
	}
	if p.RequestedAt.IsZero() {
		p.RequestedAt = time.Now().UTC()
	}
	obj, err := pendingObject(p, p.RequestedAt)
	if err != nil {
		return err
	}
	err = s.ogStore.PutBatchConditional(ctx, []objectgraph.ObjectCondition{{CanonicalID: obj.CanonicalID, Exists: false}}, objectgraph.Batch{Objects: []objectgraph.Object{obj}})
	if errors.Is(err, objectgraph.ErrConflict) {
		return fmt.Errorf("package install pending %s already written", pendingKey(p.SubjectDigest, p.Attempt))
	}
	return err
}

// GetPackageInstallPending reads one row, or ErrNotFound.
func (s *Store) GetPackageInstallPending(ctx context.Context, ownerID, computerID, subjectDigest string, attempt int) (PackageInstallPending, error) {
	obj, err := s.lifecycleGetObject(ctx, ogKindPackageInstallPending, ownerID, computerID, pendingKey(subjectDigest, attempt))
	if err != nil {
		return PackageInstallPending{}, err
	}
	if obj.Tombstone {
		return PackageInstallPending{}, ErrNotFound
	}
	return decodePending(obj)
}

func (s *Store) pendingRowsBy(ctx context.Context, field, value, ownerID, computerID string) ([]PackageInstallPending, error) {
	objs, err := s.ogListAllByMetadata(ctx, ogKindPackageInstallPending, field, value)
	if err != nil {
		return nil, err
	}
	var out []PackageInstallPending
	for _, obj := range objs {
		if obj.Tombstone || (ownerID != "" && obj.OwnerID != ownerID) || (computerID != "" && obj.ComputerID != computerID) {
			continue
		}
		p, err := decodePending(obj)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, nil
}

// LatestPackageInstallPending reads the newest attempt of a subject, or
// ErrNotFound.
func (s *Store) LatestPackageInstallPending(ctx context.Context, ownerID, computerID, subjectDigest string) (PackageInstallPending, error) {
	rows, err := s.pendingRowsBy(ctx, "subject_digest", subjectDigest, ownerID, computerID)
	if err != nil {
		return PackageInstallPending{}, err
	}
	if len(rows) == 0 {
		return PackageInstallPending{}, ErrNotFound
	}
	best := rows[0]
	for _, r := range rows[1:] {
		if r.Attempt > best.Attempt {
			best = r
		}
	}
	return best, nil
}

// ListPackageInstallPending lists this computer's rows, newest first;
// openOnly keeps the rows still waiting.
func (s *Store) ListPackageInstallPending(ctx context.Context, ownerID, computerID string, openOnly bool) ([]PackageInstallPending, error) {
	rows, err := s.pendingRowsBy(ctx, "computer_id", computerID, ownerID, computerID)
	if err != nil {
		return nil, err
	}
	var out []PackageInstallPending
	for _, r := range rows {
		if !openOnly || r.Open() {
			out = append(out, r)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].RequestedAt.After(out[j].RequestedAt) })
	return out, nil
}

// ClosePackageInstallPending moves a row from one state to another in one
// compare-and-set (against the stored content); it reports whether this call
// made the move. Closing states stamp the close time.
func (s *Store) ClosePackageInstallPending(ctx context.Context, ownerID, computerID, subjectDigest string, attempt int, from, to, reason string) (bool, error) {
	if s.ogStore == nil {
		return false, fmt.Errorf("package install pending: object graph not initialized")
	}
	obj, err := s.lifecycleGetObject(ctx, ogKindPackageInstallPending, ownerID, computerID, pendingKey(subjectDigest, attempt))
	if err != nil {
		return false, err
	}
	p, err := decodePending(obj)
	if err != nil {
		return false, err
	}
	if obj.Tombstone || p.State != from {
		return false, nil
	}
	p.State, p.Reason = to, reason
	if to != PackageInstallPendingOpen && to != PackageInstallPendingAccepted {
		p.ClosedAt = time.Now().UTC()
	}
	return s.casPending(ctx, obj, p)
}

func (s *Store) casPending(ctx context.Context, old objectgraph.Object, p PackageInstallPending) (bool, error) {
	next, err := pendingObject(p, old.CreatedAt)
	if err != nil {
		return false, err
	}
	err = s.ogStore.PutBatchConditional(ctx, []objectgraph.ObjectCondition{{CanonicalID: old.CanonicalID, Exists: true, ExpectedContentHash: old.ContentHash}}, objectgraph.Batch{Objects: []objectgraph.Object{next}})
	if errors.Is(err, objectgraph.ErrConflict) {
		return false, nil
	}
	return err == nil, err
}

// SetPackageInstallPendingError records the last refusal on a row (shown in
// the installs API).
func (s *Store) SetPackageInstallPendingError(ctx context.Context, ownerID, computerID, subjectDigest string, attempt int, lastError string) error {
	for tries := 0; tries < 3; tries++ {
		obj, err := s.lifecycleGetObject(ctx, ogKindPackageInstallPending, ownerID, computerID, pendingKey(subjectDigest, attempt))
		if err != nil {
			return err
		}
		p, err := decodePending(obj)
		if err != nil {
			return err
		}
		p.LastError = lastError
		if ok, err := s.casPending(ctx, obj, p); err != nil || ok {
			return err
		}
	}
	return nil
}

// ClosePackageInstallPendingAtBoot closes every row still waiting or accepted
// (a verdict sealed but its re-run not finished) as interrupted by a restart,
// and returns the closed rows. It is the crash boot's closer; a planned
// restart keeps its rows and its wakes.
func (s *Store) ClosePackageInstallPendingAtBoot(ctx context.Context, reason string) ([]PackageInstallPending, error) {
	var open []PackageInstallPending
	for _, state := range []string{PackageInstallPendingOpen, PackageInstallPendingAccepted} {
		rows, err := s.pendingRowsBy(ctx, "state", state, "", "")
		if err != nil {
			return nil, err
		}
		open = append(open, rows...)
	}
	var closed []PackageInstallPending
	for _, p := range open {
		ok, err := s.ClosePackageInstallPending(ctx, p.OwnerID, p.ComputerID, p.SubjectDigest, p.Attempt, p.State, PackageInstallPendingInterrupted, reason)
		if err != nil {
			return closed, err
		}
		if ok {
			p.State, p.Reason = PackageInstallPendingInterrupted, reason
			closed = append(closed, p)
		}
	}
	return closed, nil
}

// PrunePackageInstallPending deletes closed rows that closed before cutoff
// and returns how many.
func (s *Store) PrunePackageInstallPending(ctx context.Context, cutoff time.Time) (int64, error) {
	var n int64
	for _, state := range []string{PackageInstallPendingInstalled, PackageInstallPendingFailed, PackageInstallPendingRejected, PackageInstallPendingTimedOut,
		PackageInstallPendingInterrupted, PackageInstallPendingSuperseded} {
		rows, err := s.pendingRowsBy(ctx, "state", state, "", "")
		if err != nil {
			return n, err
		}
		for _, p := range rows {
			if p.ClosedAt.IsZero() || !p.ClosedAt.Before(cutoff) {
				continue
			}
			id, err := lifecycleCanonicalID(ogKindPackageInstallPending, p.OwnerID, p.ComputerID, pendingKey(p.SubjectDigest, p.Attempt))
			if err != nil {
				return n, err
			}
			if err := s.ogDelete(ctx, id); err != nil {
				return n, err
			}
			n++
		}
	}
	return n, nil
}

// pendingMaxBlob bounds a stored tree patch (pkgrelease.MaxBlobBytes,
// restated so the store imports nothing of the release format).
const pendingMaxBlob = 2 << 20
