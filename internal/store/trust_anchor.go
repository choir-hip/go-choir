package store

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/trust"
)

// The anchored sample and the spot-check sampler's seed (MA7.10), kept as
// objects of two kinds the trust service alone writes and reads. No desk
// reads them (the reader gate) and no lease opens them (the lease service and
// the desk manifest refuse them); an engineering capsule's store copy
// withholds them. An anchor is one row with the item whole, so a reader sees
// an anchor with its truth and its scorers' answers, or none.

const (
	ogKindTrustAnchor = objectgraph.ObjectKind("choir.trust_anchor")
	ogKindTrustSeed   = objectgraph.ObjectKind("choir.trust_seed")
)

// trustSeedBytes is the seed length: 256 bits from the system CSPRNG.
const trustSeedBytes = 32

type trustSeed struct {
	ComputerID string    `json:"computer_id"`
	SeedHex    string    `json:"seed_hex"`
	CreatedAt  time.Time `json:"created_at"`
}

// TrustAnchorStore is the store's trust-evidence persistence.
type TrustAnchorStore struct{ s *Store }

// TrustAnchors returns the trust-evidence persistence.
func (s *Store) TrustAnchors() TrustAnchorStore { return TrustAnchorStore{s: s} }

func trustCtx(ctx context.Context) context.Context {
	return objectgraph.WithReader(objectgraph.WithWriter(ctx, objectgraph.WriterTrust), objectgraph.WriterTrust)
}

func trustKey(computerID, id string) string {
	return strings.TrimSpace(computerID) + "\x00" + strings.TrimSpace(id)
}

func (a TrustAnchorStore) get(ctx context.Context, kind objectgraph.ObjectKind, owner, key string) (objectgraph.Object, bool, error) {
	if a.s.og == nil {
		return objectgraph.Object{}, false, fmt.Errorf("store: object graph not initialized")
	}
	id, err := objectgraph.BuildCanonicalID(kind, strings.TrimSpace(owner), objectgraph.StableSuffixFromKey(key))
	if err != nil {
		return objectgraph.Object{}, false, err
	}
	obj, err := a.s.og.GetObject(ctx, id)
	if errors.Is(err, objectgraph.ErrNotFound) {
		return objectgraph.Object{}, false, nil
	}
	return obj, err == nil && !obj.Tombstone, err
}

// Seed returns the computer's sampler seed, creating it on first use. The
// seed is random, held only in this row, and never logged or served.
func (a TrustAnchorStore) Seed(ctx context.Context, ownerID, computerID string) ([]byte, error) {
	ctx = trustCtx(ctx)
	ownerID, computerID = strings.TrimSpace(ownerID), strings.TrimSpace(computerID)
	if ownerID == "" || computerID == "" {
		return nil, fmt.Errorf("trust seed: owner and computer are required")
	}
	key := trustKey(computerID, "seed")
	if obj, ok, err := a.get(ctx, ogKindTrustSeed, ownerID, key); err != nil {
		return nil, err
	} else if ok {
		var row trustSeed
		if err := ogDecode(obj, &row); err != nil {
			return nil, err
		}
		return hex.DecodeString(row.SeedHex)
	}
	raw := make([]byte, trustSeedBytes)
	if _, err := rand.Read(raw); err != nil {
		return nil, fmt.Errorf("trust seed: %w", err)
	}
	now := time.Now().UTC()
	row := trustSeed{ComputerID: computerID, SeedHex: hex.EncodeToString(raw), CreatedAt: now}
	if _, err := a.s.ogPut(ctx, ogKindTrustSeed, ownerID, key, row, map[string]any{"computer_id": computerID}, now); err != nil {
		return nil, err
	}
	return raw, nil
}

// PutAnchor writes one anchored item, whole, in one row. The item id is the
// identity: an item already anchored keeps its first settled truth.
func (a TrustAnchorStore) PutAnchor(ctx context.Context, ownerID, computerID string, item trust.AnchorItem) error {
	ctx = trustCtx(ctx)
	ownerID, computerID = strings.TrimSpace(ownerID), strings.TrimSpace(computerID)
	if ownerID == "" || computerID == "" || strings.TrimSpace(item.ItemID) == "" {
		return fmt.Errorf("trust anchor: owner, computer and item id are required")
	}
	if _, ok := item.Anchor(); !ok {
		return fmt.Errorf("trust anchor: %s has no settled truth", item.ItemID)
	}
	item.ComputerID = computerID
	key := trustKey(computerID, item.ItemID)
	if _, exists, err := a.get(ctx, ogKindTrustAnchor, ownerID, key); err != nil || exists {
		return err
	}
	meta := map[string]any{"computer_id": computerID, "item_id": item.ItemID, "agent": item.Agent, "question_type": item.QuestionType}
	_, err := a.s.ogPut(ctx, ogKindTrustAnchor, ownerID, key, item, meta, item.SettledAt)
	return err
}

// ListAnchors returns every anchored item of the computer.
func (a TrustAnchorStore) ListAnchors(ctx context.Context, ownerID, computerID string) ([]trust.AnchorItem, error) {
	ctx = trustCtx(ctx)
	limit := 500
	for {
		objs, err := a.s.ogListByOwnerAndBody(ctx, ogKindTrustAnchor, strings.TrimSpace(ownerID),
			[]objectgraph.JSONFieldMatch{{JSONPath: "$.computer_id", Value: strings.TrimSpace(computerID)}}, limit)
		if err != nil {
			return nil, err
		}
		if len(objs) >= limit {
			limit *= 4
			continue
		}
		out := make([]trust.AnchorItem, 0, len(objs))
		for _, obj := range objs {
			if obj.Tombstone {
				continue
			}
			var it trust.AnchorItem
			if err := ogDecode(obj, &it); err != nil {
				return nil, err
			}
			out = append(out, it)
		}
		return out, nil
	}
}
