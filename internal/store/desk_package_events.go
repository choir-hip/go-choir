package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
)

// ogKindDeskPackageEvent is one fact of the desk package event log (plan MA4.2
// and MA4.4): an install, update, candidate, supplant, drop or disable that the
// registry of package desks is folded from. It is an object of the graph, so it
// rides the tape and a restore to an earlier head restores the log to that
// head; the boot re-seeds the image's required packages after it
// (appdesk.Boot). The kernel is its writer: the boot reconcile appends it.
const ogKindDeskPackageEvent = objectgraph.ObjectKind("choir.desk_package_event")

// DeskPackageEvent is the stored body: an opaque JSON event (internal/appdesk
// owns its shape) at the sequence the log gave it.
type DeskPackageEvent struct {
	Seq       uint64          `json:"seq"`
	Body      json.RawMessage `json:"body"`
	CreatedAt time.Time       `json:"created_at"`
}

func deskPackageEventKey(seq uint64) string { return fmt.Sprintf("%020d", seq) }

// AppendDeskPackageEvent appends a fact to one computer's log and returns its
// sequence: one past the highest the log holds. Sequences only increase, so a
// sequence is a desk epoch. The log is append-only.
func (s *Store) AppendDeskPackageEvent(ctx context.Context, ownerID string, body json.RawMessage) (uint64, error) {
	ownerID = strings.TrimSpace(ownerID)
	if ownerID == "" || len(body) == 0 || !json.Valid(body) {
		return 0, errors.New("store: a desk package event needs an owner and a JSON body")
	}
	ctx = objectgraph.WithWriter(ctx, objectgraph.WriterKernel)
	existing, err := s.ListDeskPackageEvents(ctx, ownerID)
	if err != nil {
		return 0, err
	}
	seq := uint64(1)
	if n := len(existing); n > 0 {
		seq = existing[n-1].Seq + 1
	}
	now := time.Now().UTC()
	ev := DeskPackageEvent{Seq: seq, Body: body, CreatedAt: now}
	if _, err := s.ogPut(ctx, ogKindDeskPackageEvent, ownerID, deskPackageEventKey(seq), ev, map[string]any{"seq": seq}, now); err != nil {
		return 0, fmt.Errorf("store: append desk package event: %w", err)
	}
	return seq, nil
}

// ListDeskPackageEvents returns the owner's log in sequence order.
func (s *Store) ListDeskPackageEvents(ctx context.Context, ownerID string) ([]DeskPackageEvent, error) {
	ownerID = strings.TrimSpace(ownerID)
	ctx = objectgraph.WithReader(ctx, objectgraph.WriterKernel)
	objs, err := s.ogListAllObjectsByKind(ctx, ogKindDeskPackageEvent)
	if err != nil {
		return nil, fmt.Errorf("store: list desk package events: %w", err)
	}
	var out []DeskPackageEvent
	for _, o := range objs {
		if o.OwnerID != ownerID || o.Tombstone {
			continue
		}
		var ev DeskPackageEvent
		if err := json.Unmarshal(o.Body, &ev); err != nil {
			return nil, fmt.Errorf("store: decode desk package event %s: %w", o.CanonicalID, err)
		}
		out = append(out, ev)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Seq < out[j].Seq })
	return out, nil
}
