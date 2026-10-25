package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
)

// ogKindTextureRoot names the computer's master document (PD3 S1,
// docs/definitions/choir-pd3-texture-root-desk-2026-10-11.md). The master is
// an ordinary Texture document created by the ordinary create path; its own
// desk, texture:<master>, is the Texture root, so no write check changes.
// One marker per owner and computer, written only by compare-and-set: the
// claim is the ensure step's atomic step, so two ensure steps never name two
// masters. Additive kind, no schema migration.
const ogKindTextureRoot = objectgraph.ObjectKind("choir.texture_root")

const textureRootKey = "texture_root"

// TextureRoot is the marker: which document is the master, and its
// generation. A replacement (the previous master settled) advances the
// generation by one and names the document it replaces.
type TextureRoot struct {
	OwnerID               string    `json:"owner_id"`
	ComputerID            string    `json:"computer_id"`
	DocumentID            string    `json:"document_id"`
	Generation            int64     `json:"generation"`
	PredecessorDocumentID string    `json:"predecessor_document_id,omitempty"`
	ClaimedAt             time.Time `json:"claimed_at"`

	contentHash string
}

// GetTextureRoot returns the computer's marker, or ErrNotFound.
func (s *Store) GetTextureRoot(ctx context.Context, ownerID, computerID string) (TextureRoot, error) {
	ownerID, computerID, err := normalizeLifecycleScope(ownerID, computerID)
	if err != nil {
		return TextureRoot{}, err
	}
	if s.ogStore == nil {
		return TextureRoot{}, fmt.Errorf("store: object graph not initialized")
	}
	id, err := lifecycleCanonicalID(ogKindTextureRoot, ownerID, computerID, textureRootKey)
	if err != nil {
		return TextureRoot{}, err
	}
	obj, err := s.ogStore.GetObject(ctx, id)
	if errors.Is(err, objectgraph.ErrNotFound) {
		return TextureRoot{}, ErrNotFound
	}
	if err != nil {
		return TextureRoot{}, err
	}
	var root TextureRoot
	if err := ogDecode(obj, &root); err != nil {
		return TextureRoot{}, err
	}
	root.contentHash = obj.ContentHash
	return root, nil
}

// ClaimTextureRoot writes next only if the marker is still expected: absent
// when expected is nil, else unchanged since expected was read. next must be
// generation 1 for a first claim and expected's generation plus one for a
// replacement. A lost race returns ErrConcurrentStateChange; the caller
// re-reads the marker and uses the winner.
func (s *Store) ClaimTextureRoot(ctx context.Context, expected *TextureRoot, next TextureRoot) (TextureRoot, error) {
	ownerID, computerID, err := normalizeLifecycleScope(next.OwnerID, next.ComputerID)
	if err != nil {
		return TextureRoot{}, err
	}
	next.OwnerID, next.ComputerID = ownerID, computerID
	next.DocumentID = strings.TrimSpace(next.DocumentID)
	next.PredecessorDocumentID = strings.TrimSpace(next.PredecessorDocumentID)
	if next.DocumentID == "" {
		return TextureRoot{}, fmt.Errorf("texture root: document_id is required")
	}
	if s.ogStore == nil {
		return TextureRoot{}, fmt.Errorf("store: object graph not initialized")
	}
	id, err := lifecycleCanonicalID(ogKindTextureRoot, ownerID, computerID, textureRootKey)
	if err != nil {
		return TextureRoot{}, err
	}
	condition := objectgraph.ObjectCondition{CanonicalID: id, Exists: false}
	wantGeneration := int64(1)
	if expected == nil {
		// A claim that expects no marker loses to any marker, whatever
		// generation it names.
		if _, getErr := s.GetTextureRoot(ctx, ownerID, computerID); getErr == nil {
			return TextureRoot{}, ErrConcurrentStateChange
		} else if !errors.Is(getErr, ErrNotFound) {
			return TextureRoot{}, getErr
		}
	} else {
		if expected.OwnerID != ownerID || expected.ComputerID != computerID || expected.contentHash == "" {
			return TextureRoot{}, fmt.Errorf("texture root: expected marker was not read from this computer's store")
		}
		condition = objectgraph.ObjectCondition{CanonicalID: id, Exists: true, ExpectedContentHash: expected.contentHash}
		wantGeneration = expected.Generation + 1
	}
	if next.Generation != wantGeneration {
		return TextureRoot{}, fmt.Errorf("texture root: generation %d does not follow %d", next.Generation, wantGeneration-1)
	}
	if next.ClaimedAt.IsZero() {
		next.ClaimedAt = time.Now().UTC()
	}
	metadata := map[string]any{
		"computer_id": computerID,
		"document_id": next.DocumentID,
		"generation":  next.Generation,
	}
	obj, err := lifecycleObject(ogKindTextureRoot, ownerID, computerID, textureRootKey, next, metadata, next.ClaimedAt, next.ClaimedAt)
	if err != nil {
		return TextureRoot{}, err
	}
	if err := s.ogStore.PutBatchConditional(ctx, []objectgraph.ObjectCondition{condition}, objectgraph.Batch{Objects: []objectgraph.Object{obj}}); err != nil {
		if errors.Is(err, objectgraph.ErrConflict) {
			return TextureRoot{}, ErrConcurrentStateChange
		}
		return TextureRoot{}, err
	}
	next.contentHash = obj.ContentHash
	return next, nil
}
