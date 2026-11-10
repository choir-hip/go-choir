package algorithm2

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/yusefmosiah/go-choir/internal/ownerfiles"
)

// State is where a proposal is.
type State string

const (
	StatePending  State = "pending"
	StateAccepted State = "accepted"
	StateDeclined State = "declined"
	// StateStale is a proposal whose base is no longer the algorithm document's
	// head: it is never applied, and the next round proposes afresh.
	StateStale State = "stale"
)

// MaxProposalBytes bounds a proposed algorithm document.
const MaxProposalBytes = 64 << 10

// ProposalIDPrefix starts every proposal's id. The id is a published-version
// id in shape (prefix plus fixed-width hex) so the revise call's source
// validation can treat a proposal as a source; see textureowner.
const ProposalIDPrefix = "proposal:"

var proposalIDPattern = regexp.MustCompile(`^proposal:[0-9a-f]{32}$`)

// IsProposalID reports whether id has the shape of a proposal id.
func IsProposalID(id string) bool { return proposalIDPattern.MatchString(id) }

// Proposal is one proposed edit to the algorithm document: the whole proposed
// text, pinned by its manifest hash, plus what changed. It is a local object
// shaped like a published version; accepting it is a revise call with the
// proposal as its source.
type Proposal struct {
	ID             string          `json:"id"`
	DocID          string          `json:"doc_id"`
	BaseRevisionID string          `json:"base_revision_id"`
	CreatedAt      time.Time       `json:"created_at"`
	Content        string          `json:"content"`
	ManifestHash   string          `json:"manifest_hash"`
	Changes        []SectionChange `json:"changes"`
	// Rationale is the model's prose. It is shown to the owner and never
	// decides anything.
	Rationale  string     `json:"rationale,omitempty"`
	State      State      `json:"state"`
	DecidedAt  *time.Time `json:"decided_at,omitempty"`
	DecidedBy  string     `json:"decided_by,omitempty"`
	RevisionID string     `json:"revision_id,omitempty"`
}

// ManifestHashOf is the pin of a proposed text.
func ManifestHashOf(content string) string {
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])
}

// NewProposal builds the pending proposal that turns current into proposed on
// the named base revision. The changes are computed here from the two texts.
func NewProposal(docID, baseRevisionID, current, proposed, rationale string, now time.Time) (Proposal, error) {
	if strings.TrimSpace(docID) == "" || strings.TrimSpace(baseRevisionID) == "" {
		return Proposal{}, errors.New("a proposal needs the document and the revision it is based on")
	}
	if strings.TrimSpace(proposed) == "" {
		return Proposal{}, errors.New("a proposal cannot empty the document")
	}
	if len(proposed) > MaxProposalBytes {
		return Proposal{}, fmt.Errorf("proposed text is %d bytes; the bound is %d", len(proposed), MaxProposalBytes)
	}
	changes := DiffSections(current, proposed)
	if len(changes) == 0 {
		return Proposal{}, errors.New("the proposal changes nothing")
	}
	sum := sha256.Sum256([]byte(docID + "\x00" + baseRevisionID + "\x00" + proposed))
	if len(rationale) > 2000 {
		rationale = strings.ToValidUTF8(rationale[:2000], "")
	}
	return Proposal{
		ID: ProposalIDPrefix + hex.EncodeToString(sum[:16]), DocID: docID, BaseRevisionID: baseRevisionID,
		CreatedAt: now.UTC(), Content: proposed, ManifestHash: ManifestHashOf(proposed),
		Changes: changes, Rationale: rationale, State: StatePending,
	}, nil
}

// Store keeps proposals and the time of the last round.
type Store interface {
	Put(Proposal) error
	Get(id string) (Proposal, bool, error)
	List() ([]Proposal, error)
	SetState(id string, to State, by, revisionID string, at time.Time) error
	LastRound() (time.Time, error)
	RecordRound(at time.Time) error
}

// FileStore keeps one JSON file per proposal under dir/proposals and the last
// round time in dir/round.json, on the owner's computer.
type FileStore struct {
	dir string
	mu  sync.Mutex
}

// NewFileStore returns a store rooted at dir.
func NewFileStore(dir string) *FileStore { return &FileStore{dir: dir} }

func (s *FileStore) path(id string) (string, error) {
	if !IsProposalID(id) {
		return "", fmt.Errorf("%q is not a proposal id", id)
	}
	return filepath.Join(s.dir, "proposals", strings.TrimPrefix(id, ProposalIDPrefix)+".json"), nil
}

func (s *FileStore) write(path string, v any) error {
	raw, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	if err := ownerfiles.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return ownerfiles.WriteFile(path, raw, 0o600)
}

// Put stores a proposal; an existing proposal with the same id is kept as it is.
func (s *FileStore) Put(p Proposal) error {
	path, err := s.path(p.ID)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	return s.write(path, p)
}

func (s *FileStore) get(id string) (Proposal, bool, error) {
	path, err := s.path(id)
	if err != nil {
		return Proposal{}, false, err
	}
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Proposal{}, false, nil
	}
	if err != nil {
		return Proposal{}, false, err
	}
	var p Proposal
	if err := json.Unmarshal(raw, &p); err != nil {
		return Proposal{}, false, fmt.Errorf("proposal %s is unreadable: %w", id, err)
	}
	return p, true, nil
}

// Get reads one proposal. A malformed id is an error, an unknown one is not found.
func (s *FileStore) Get(id string) (Proposal, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.get(id)
}

// List returns every proposal, newest first.
func (s *FileStore) List() ([]Proposal, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entries, err := os.ReadDir(filepath.Join(s.dir, "proposals"))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []Proposal
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".json") {
			continue
		}
		p, ok, err := s.get(ProposalIDPrefix + strings.TrimSuffix(name, ".json"))
		if err != nil || !ok {
			continue
		}
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}

// SetState moves a pending proposal to a decided state. A decided proposal
// does not change again.
func (s *FileStore) SetState(id string, to State, by, revisionID string, at time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok, err := s.get(id)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("proposal %s not found", id)
	}
	if p.State != StatePending {
		return fmt.Errorf("proposal %s is already %s", id, p.State)
	}
	at = at.UTC()
	p.State, p.DecidedBy, p.RevisionID, p.DecidedAt = to, by, revisionID, &at
	path, _ := s.path(id)
	return s.write(path, p)
}

type roundFile struct {
	LastRound time.Time `json:"last_round"`
}

// LastRound is when the last round that called the model ran; zero if none.
func (s *FileStore) LastRound() (time.Time, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	raw, err := os.ReadFile(filepath.Join(s.dir, "round.json"))
	if errors.Is(err, os.ErrNotExist) {
		return time.Time{}, nil
	}
	if err != nil {
		return time.Time{}, err
	}
	var f roundFile
	if err := json.Unmarshal(raw, &f); err != nil {
		return time.Time{}, err
	}
	return f.LastRound, nil
}

// RecordRound stores the time of a round that called the model.
func (s *FileStore) RecordRound(at time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.write(filepath.Join(s.dir, "round.json"), roundFile{LastRound: at.UTC()})
}
