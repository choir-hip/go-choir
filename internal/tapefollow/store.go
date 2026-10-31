// Package tapefollow keeps a verified, read-only local copy of one computer's
// tape and snapshot: the follower side of sync (the owner's Mac first,
// docs/design/one-tape-durability-2026-10-31.md section 4a; a follower is
// read-only, docs/design/branching-tape-2026-10-31.md).
//
// The local store is a chain of verified bundles (internal/bundle, format 2),
// each one exactly what choir-bundle verify and choir-bundle import accept,
// so the local copy can also restore the computer on another host:
//
//	<dir>/                       one computer (…/Choir/<computer-id>/ on a Mac)
//	  follow.json                the commit point: the chain of applied
//	                             bundles, the synced head, bytes, time
//	  bundles/<start>-<head>/    one verified bundle per pull (start and head
//	                             sequences, zero padded); its files read-only
//	  staging/                   pulls in progress; cleared at Open
//	  lock                       one syncer per store (flock)
//
// Apply is atomic: a pull lands in staging, is verified there, is renamed
// into bundles/ and only then named by follow.json (written by rename). A
// crash at any point leaves follow.json naming the previous verified chain;
// Open removes the staging area and any bundle directory follow.json does not
// name.
//
// Nothing here writes to the computer. The package's only network call is
// the GET of the next bundle (client.go); there is no code path that appends
// to the computer's tape, pins, or moves its head.
package tapefollow

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/bundle"
	"github.com/yusefmosiah/go-choir/internal/computerevent"
)

// Branch is the only branch followed (the writer lease's branch_id).
const Branch = "main"

const (
	StateName   = "follow.json"
	BundlesDir  = "bundles"
	StagingDir  = "staging"
	lockName    = "lock"
	stateFormat = 1
)

// BundleRef is one applied bundle of the chain.
type BundleRef struct {
	Name       string `json:"name"`
	StartSeq   int64  `json:"start_seq"`
	StartHead  string `json:"start_head"`
	HeadSeq    int64  `json:"head_seq"`
	HeadDigest string `json:"head_digest"`
	Bytes      int64  `json:"bytes"`
	Snapshot   string `json:"snapshot_root,omitempty"`
}

// State is follow.json.
type State struct {
	FormatVersion int    `json:"format_version"`
	ComputerID    string `json:"computer_id"`
	Branch        string `json:"branch"`
	// StartSeq and StartHead are where the local copy begins: the snapshot
	// head of the bootstrap bundle, or 0 and the zero head.
	StartSeq   int64  `json:"start_seq"`
	StartHead  string `json:"start_head"`
	HeadSeq    int64  `json:"head_seq"`
	HeadDigest string `json:"head_digest"`
	// SnapshotRoot is the snapshot root the copy was bootstrapped from.
	SnapshotRoot    string      `json:"snapshot_root,omitempty"`
	SnapshotHeadSeq int64       `json:"snapshot_head_seq,omitempty"`
	Bundles         []BundleRef `json:"bundles"`
	Bytes           int64       `json:"bytes"`
	SyncedAt        time.Time   `json:"synced_at"`
}

// ErrNotChained is wrapped when a pulled bundle does not extend the local
// head: the store is left as it was.
var ErrNotChained = errors.New("tapefollow: bundle does not extend the local copy")

// verifyBundle is the one verification of a pulled bundle: bundle.Verify
// with the store's trust set (docs/design/bundle-authenticity-2026-10-31.md).
// Every object must hash to its name and the chain must link, and every
// receipt must be signed by a platform-control key the follower trusts (the
// trust set never comes from the server or the bundle), and a bootstrap's
// snapshot must be named by a snapshot_recorded event of its tape. A forged
// bundle is refused as surely as a corrupted one.
func (s *Store) verifyBundle(dir string) (bundle.Manifest, error) { return bundle.Verify(dir, s.trust) }

// applyHook, set only by tests (SetApplyHookForTesting), runs at the named
// points of Apply; an error there stops Apply as a crash at that point would,
// leaving the files exactly as they are.
var applyHook func(point string) error

// SetApplyHookForTesting installs a hook run at Apply's crash points
// ("verified", "renamed") and returns the restore func. Tests only.
func SetApplyHookForTesting(f func(point string) error) (restore func()) {
	prev := applyHook
	applyHook = f
	return func() { applyHook = prev }
}

func crashPoint(point string) error {
	if applyHook != nil {
		return applyHook(point)
	}
	return nil
}

// Store is one computer's local copy.
type Store struct {
	dir        string
	computerID string
	state      *State
	trust      *bundle.TrustSet
	unlock     func()
}

// DefaultTrust is the follower's trust set when the caller names none: the
// repository-pinned platform-control key (bundle.PinnedTrust), the key that
// signs choir.news's receipts.
func DefaultTrust() (*bundle.TrustSet, error) { return bundle.NewTrustSet(bundle.PinnedTrust()) }

// Open opens (creating) the store for computerID at dir, takes its lock,
// clears what a crash left (staging, unnamed bundles) and reads follow.json.
// Every bundle applied or re-verified is checked against trust (nil:
// DefaultTrust).
func Open(dir, computerID string, trust *bundle.TrustSet) (*Store, error) {
	if trust == nil {
		var err error
		if trust, err = DefaultTrust(); err != nil {
			return nil, err
		}
	}
	if computerID == "" || computerID != strings.TrimSpace(computerID) || strings.ContainsAny(computerID, `/\`) || computerID == "." || computerID == ".." {
		return nil, fmt.Errorf("tapefollow: invalid computer id")
	}
	if err := os.MkdirAll(filepath.Join(dir, BundlesDir), 0o750); err != nil {
		return nil, err
	}
	unlock, err := lockDir(filepath.Join(dir, lockName))
	if err != nil {
		return nil, fmt.Errorf("tapefollow: %s is in use by another syncer: %w", dir, err)
	}
	s := &Store{dir: dir, computerID: computerID, trust: trust, unlock: unlock}
	if err := s.recover(); err != nil {
		unlock()
		return nil, err
	}
	return s, nil
}

// Close releases the store's lock.
func (s *Store) Close() error {
	if s.unlock != nil {
		s.unlock()
		s.unlock = nil
	}
	return nil
}

func (s *Store) Dir() string        { return s.dir }
func (s *Store) ComputerID() string { return s.computerID }

// State returns the synced state; false when nothing is synced yet.
func (s *Store) State() (State, bool) {
	if s.state == nil {
		return State{}, false
	}
	st := *s.state
	st.Bundles = append([]BundleRef(nil), s.state.Bundles...)
	return st, true
}

func (s *Store) recover() error {
	if err := removeAll(filepath.Join(s.dir, StagingDir)); err != nil {
		return err
	}
	raw, err := os.ReadFile(filepath.Join(s.dir, StateName))
	switch {
	case errors.Is(err, os.ErrNotExist):
		s.state = nil
	case err != nil:
		return err
	default:
		var st State
		if err := json.Unmarshal(raw, &st); err != nil {
			return fmt.Errorf("tapefollow: %s unreadable: %w", StateName, err)
		}
		if st.FormatVersion != stateFormat || st.ComputerID != s.computerID || st.Branch != Branch {
			return fmt.Errorf("tapefollow: %s is not a %s store of %s", StateName, Branch, s.computerID)
		}
		s.state = &st
	}
	named := map[string]bool{}
	if s.state != nil {
		for _, b := range s.state.Bundles {
			named[b.Name] = true
			if _, err := os.Stat(filepath.Join(s.dir, BundlesDir, b.Name, bundle.ManifestName)); err != nil {
				return fmt.Errorf("tapefollow: bundle %s named by %s is missing: %w", b.Name, StateName, err)
			}
		}
	}
	entries, err := os.ReadDir(filepath.Join(s.dir, BundlesDir))
	if err != nil {
		return err
	}
	for _, e := range entries {
		if !named[e.Name()] {
			// Renamed in by an apply that crashed before its commit.
			if err := removeAll(filepath.Join(s.dir, BundlesDir, e.Name())); err != nil {
				return err
			}
		}
	}
	return nil
}

// NewStaging returns an empty directory for one pull.
func (s *Store) NewStaging() (string, error) {
	root := filepath.Join(s.dir, StagingDir)
	if err := os.MkdirAll(root, 0o750); err != nil {
		return "", err
	}
	return os.MkdirTemp(root, "pull-")
}

// DiscardStaging removes a staging directory.
func (s *Store) DiscardStaging(dir string) { _ = removeAll(dir) }

// Apply verifies the bundle in the staging directory stage and, when it
// extends the local copy (any bundle of this computer when the store is
// empty; a delta from the local head otherwise), moves it into the chain and
// commits follow.json. A refused bundle leaves the store as it was.
func (s *Store) Apply(stage string, now time.Time) (State, error) {
	m, err := s.verifyBundle(stage)
	if err != nil {
		return State{}, fmt.Errorf("tapefollow: bundle refused: %w", err)
	}
	if m.ComputerID != s.computerID {
		return State{}, fmt.Errorf("%w: it is of computer %s, not %s", ErrNotChained, m.ComputerID, s.computerID)
	}
	var next State
	if s.state == nil {
		next = State{FormatVersion: stateFormat, ComputerID: s.computerID, Branch: Branch, StartSeq: m.Tape.StartSeq, StartHead: m.Tape.StartHead}
		if m.SnapshotRoot != nil {
			next.SnapshotRoot, next.SnapshotHeadSeq = m.SnapshotRoot.Root, m.SnapshotRoot.HeadSequence
		} else if m.Tape.StartSeq != 0 || m.Tape.StartHead != computerevent.ZeroHead {
			return State{}, fmt.Errorf("%w: a first bundle without a snapshot must start at genesis", ErrNotChained)
		}
	} else {
		next, _ = s.State()
		if !m.Delta || m.SnapshotRoot != nil || m.Tape.StartSeq != next.HeadSeq || m.Tape.StartHead != next.HeadDigest {
			return State{}, fmt.Errorf("%w: it starts at %d (%s), the local head is %d (%s)", ErrNotChained, m.Tape.StartSeq, m.Tape.StartHead, next.HeadSeq, next.HeadDigest)
		}
	}
	size, err := dirBytes(stage)
	if err != nil {
		return State{}, err
	}
	ref := BundleRef{
		Name: fmt.Sprintf("%020d-%020d", m.Tape.StartSeq, m.Tape.HeadSeq), StartSeq: m.Tape.StartSeq, StartHead: m.Tape.StartHead,
		HeadSeq: m.Tape.HeadSeq, HeadDigest: m.Tape.HeadDigest, Bytes: size,
	}
	if m.SnapshotRoot != nil {
		ref.Snapshot = m.SnapshotRoot.Root
	}
	if err := makeReadOnly(stage); err != nil {
		return State{}, err
	}
	if err := crashPoint("verified"); err != nil {
		return State{}, err
	}
	final := filepath.Join(s.dir, BundlesDir, ref.Name)
	if err := removeAll(final); err != nil { // an unnamed leftover of the same range
		return State{}, err
	}
	if err := os.Rename(stage, final); err != nil {
		return State{}, err
	}
	if err := syncDir(filepath.Join(s.dir, BundlesDir)); err != nil {
		return State{}, err
	}
	if err := crashPoint("renamed"); err != nil {
		return State{}, err
	}
	next.HeadSeq, next.HeadDigest = m.Tape.HeadSeq, m.Tape.HeadDigest
	next.Bundles = append(next.Bundles, ref)
	next.Bytes += size
	next.SyncedAt = now.UTC()
	if err := writeState(s.dir, next); err != nil {
		// follow.json still names the previous chain; the renamed bundle is
		// removed at the next Open.
		return State{}, err
	}
	s.state = &next
	return next, nil
}

// VerifyAll re-verifies every bundle of the chain and that each one starts
// at the head the one before it ends at: the whole local copy, from its start
// to its head, checked as a single tape.
func (s *Store) VerifyAll() (State, error) {
	st, ok := s.State()
	if !ok {
		return State{}, fmt.Errorf("tapefollow: nothing synced")
	}
	seq, head := st.StartSeq, st.StartHead
	for i, b := range st.Bundles {
		m, err := s.verifyBundle(filepath.Join(s.dir, BundlesDir, b.Name))
		if err != nil {
			return st, fmt.Errorf("tapefollow: bundle %s: %w", b.Name, err)
		}
		if m.ComputerID != s.computerID || m.Tape.StartSeq != seq || m.Tape.StartHead != head || m.Tape.HeadSeq != b.HeadSeq || m.Tape.HeadDigest != b.HeadDigest || (i > 0 && !m.Delta) {
			return st, fmt.Errorf("tapefollow: bundle %s does not continue the chain at %d", b.Name, seq)
		}
		seq, head = m.Tape.HeadSeq, m.Tape.HeadDigest
	}
	if seq != st.HeadSeq || head != st.HeadDigest {
		return st, fmt.Errorf("tapefollow: the chain ends at %d, %s says %d", seq, StateName, st.HeadSeq)
	}
	return st, nil
}

// BundleDirs lists the chain's bundle directories in order.
func (s *Store) BundleDirs() []string {
	st, _ := s.State()
	out := make([]string, 0, len(st.Bundles))
	for _, b := range st.Bundles {
		out = append(out, filepath.Join(s.dir, BundlesDir, b.Name))
	}
	return out
}

func writeState(dir string, st State) error {
	raw, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".follow-*.json")
	if err != nil {
		return err
	}
	name := tmp.Name()
	_, err = tmp.Write(append(raw, '\n'))
	if err == nil {
		err = tmp.Sync()
	}
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Rename(name, filepath.Join(dir, StateName))
	}
	if err != nil {
		_ = os.Remove(name)
		return err
	}
	return syncDir(dir)
}

func syncDir(dir string) error {
	f, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer f.Close()
	return f.Sync()
}

func dirBytes(dir string) (int64, error) {
	var n int64
	err := filepath.WalkDir(dir, func(_ string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Type().IsRegular() {
			info, err := d.Info()
			if err != nil {
				return err
			}
			n += info.Size()
		}
		return nil
	})
	return n, err
}

// makeReadOnly drops write permission from a bundle's files: the local copy
// is a follower's, never edited in place.
func makeReadOnly(dir string) error {
	return filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Type().IsRegular() {
			return os.Chmod(p, 0o440)
		}
		return nil
	})
}

// removeAll removes a tree whose files may be read-only.
func removeAll(p string) error {
	err := os.RemoveAll(p)
	if err == nil || errors.Is(err, os.ErrNotExist) {
		return nil
	}
	_ = filepath.WalkDir(p, func(q string, d fs.DirEntry, err error) error {
		if err == nil && d.IsDir() {
			_ = os.Chmod(q, 0o750)
		}
		return nil
	})
	return os.RemoveAll(p)
}

// DefaultStoreDir is the app's store for computerID: on macOS
// ~/Library/Application Support/Choir/<computer-id>.
func DefaultStoreDir(computerID string) (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "Choir", computerID), nil
}
