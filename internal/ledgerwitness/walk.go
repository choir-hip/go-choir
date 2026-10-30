// Package ledgerwitness reads where an inbox record sits on a computer's
// event chain, from the host's event artifacts: L3's witness (a)
// (docs/archive/definitions/choir-ledger-l3-hypotheses-settled-2026-10-13.md).
//
// It walks back from a given head along previous_head, reading each event
// file and, for projection batches, its payload, until every asked record
// is found or the cap is reached. It never opens SQL or the guest. Inbox
// records are created under a must-not-exist condition, so a record is in
// exactly one projection batch and the first hit is its position.
// Anything it cannot prove (a record past the cap, a broken or foreign
// chain, a missing payload) is reported missing or stops the walk: the
// caller fails closed.
package ledgerwitness

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/tapestore"
)

// CommitmentRecordKind is the object kind of an inbox record.
const CommitmentRecordKind = "choir.commitment_record"

// Config names the chain to walk.
type Config struct {
	ArtifactsRoot string // the host's platform-artifacts directory
	ComputerID    string
	Head          string // the canonical event head to walk back from
	MaxEvents     int    // at most this many events are read
}

// Position is one record's place on the chain.
type Position struct {
	RecordID     string `json:"record_id"`
	CanonicalID  string `json:"canonical_id"`
	Sequence     uint64 `json:"sequence"`
	EventDigest  string `json:"event_digest"`
	PreviousHead string `json:"previous_head"`
}

// Result is what the walk proved.
type Result struct {
	ComputerID   string              `json:"computer_id"`
	Head         string              `json:"head"`
	HeadSequence uint64              `json:"head_sequence"`
	Walked       int                 `json:"walked"`
	Capped       bool                `json:"capped"`
	Found        map[string]Position `json:"found"`
	Missing      []string            `json:"missing"`
}

// Walk finds recordIDs on the chain ending at cfg.Head.
func Walk(ctx context.Context, cfg Config, recordIDs []string) (Result, error) {
	cfg.ComputerID, cfg.Head = strings.TrimSpace(cfg.ComputerID), strings.TrimSpace(cfg.Head)
	if cfg.ArtifactsRoot == "" || cfg.ComputerID == "" || !computerevent.IsSHA256(cfg.Head) || cfg.MaxEvents <= 0 {
		return Result{}, fmt.Errorf("ledger witness: artifacts root, computer, a sha256 head and a positive cap are required")
	}
	want := map[string]bool{}
	for _, id := range recordIDs {
		if id = strings.TrimSpace(id); id != "" {
			want[id] = true
		}
	}
	res := Result{ComputerID: cfg.ComputerID, Head: cfg.Head, Found: map[string]Position{}}
	tape := tapestore.NewReader(cfg.ArtifactsRoot)
	curr := cfg.Head
	for curr != computerevent.ZeroHead && len(res.Found) < len(want) {
		if err := ctx.Err(); err != nil {
			return res, err
		}
		if res.Walked >= cfg.MaxEvents {
			res.Capped = true
			break
		}
		event, err := readEvent(tape, cfg.ComputerID, curr)
		if err != nil {
			return res, err
		}
		if event.ComputerID != cfg.ComputerID {
			return res, fmt.Errorf("ledger witness: event %s belongs to computer %q, not %q", curr, event.ComputerID, cfg.ComputerID)
		}
		if res.Walked == 0 {
			res.HeadSequence = event.Sequence
		}
		res.Walked++
		if event.EventKind == computerevent.EventProjectionBatchRecorded {
			ids, err := recordsInPayload(tape, cfg.ComputerID, curr, event.PayloadCommitment, want)
			if err != nil {
				return res, err
			}
			for recordID, canonicalID := range ids {
				if _, seen := res.Found[recordID]; !seen {
					res.Found[recordID] = Position{RecordID: recordID, CanonicalID: canonicalID, Sequence: event.Sequence, EventDigest: curr, PreviousHead: event.PreviousHead}
				}
			}
		}
		curr = event.PreviousHead
	}
	for id := range want {
		if _, ok := res.Found[id]; !ok {
			res.Missing = append(res.Missing, id)
		}
	}
	sortStrings(res.Missing)
	return res, nil
}

// readEvent reads an event segment-first, legacy-second (internal/tapestore).
// A corrupt segment is an error; it is never answered from the other layout.
func readEvent(tape *tapestore.Reader, computerID, digest string) (computerevent.Event, error) {
	if !computerevent.IsSHA256(digest) {
		return computerevent.Event{}, fmt.Errorf("ledger witness: previous head %q is not a sha256", digest)
	}
	raw, _, err := tape.Read(computerID, tapestore.NSEvent, digest)
	if err != nil {
		return computerevent.Event{}, fmt.Errorf("ledger witness: read event %s: %w", digest, err)
	}
	if computerevent.DigestBytes(raw) != digest {
		return computerevent.Event{}, fmt.Errorf("ledger witness: event file %s does not hash to its name", digest)
	}
	return computerevent.DecodeHistoricEvent(raw)
}

// recordsInPayload returns the asked records an event's projection batch
// creates, by record id, with their canonical ids.
func recordsInPayload(tape *tapestore.Reader, computerID, eventDigest, payloadDigest string, want map[string]bool) (map[string]string, error) {
	if !computerevent.IsSHA256(payloadDigest) {
		return nil, fmt.Errorf("ledger witness: projection event %s has no payload commitment", eventDigest)
	}
	raw, _, err := tape.Read(computerID, tapestore.NSPayload, payloadDigest)
	if err != nil {
		return nil, fmt.Errorf("ledger witness: projection event %s: read payload %s: %w", eventDigest, payloadDigest, err)
	}
	if computerevent.DigestBytes(raw) != payloadDigest {
		return nil, fmt.Errorf("ledger witness: payload %s does not hash to its name", payloadDigest)
	}
	var batch computerevent.ProjectionBatch
	if err := json.Unmarshal(raw, &batch); err != nil {
		return nil, fmt.Errorf("ledger witness: decode payload %s: %w", payloadDigest, err)
	}
	out := map[string]string{}
	for _, op := range batch.Ops {
		if op.Kind != computerevent.ProjectionOpObject {
			continue
		}
		var obj struct {
			ObjectKind string          `json:"object_kind"`
			Metadata   json.RawMessage `json:"metadata"`
		}
		if json.Unmarshal(op.Body, &obj) != nil || obj.ObjectKind != CommitmentRecordKind {
			continue
		}
		var meta struct {
			RecordID string `json:"record_id"`
		}
		if json.Unmarshal(obj.Metadata, &meta) != nil || !want[meta.RecordID] {
			continue
		}
		out[meta.RecordID] = op.CanonicalID
	}
	return out, nil
}

func sortStrings(s []string) { sort.Strings(s) }
