package agentcore

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/sourceproj"
)

// Capsule overlay S4 (docs/design/capsule-overlay-computer-state-2026-11-03.md
// section 5): reconcile saves what a reset would lose at
// refs/choir/saved/<commit> and, before it resets, this puts each save on the
// tape as a source.projection_saved fact (a recovery_recorded event), so an
// owner edit in Source/platform is never lost silently.

const sourceProjectionSavedMediaType = "application/vnd.choir.source-projection-saved+json"

// sourceProjectionSavedPayload is the tape body of one save.
type sourceProjectionSavedPayload struct {
	Fact   string `json:"fact"`
	Ref    string `json:"ref"`
	Commit string `json:"commit"`
	Kind   string `json:"kind"`
	From   string `json:"from,omitempty"`
	To     string `json:"to,omitempty"`
}

// sourceProjectionSavedIdempotency names one save on the tape: the commit is
// the ref's name, so a retried reconcile appends nothing new.
func sourceProjectionSavedIdempotency(commit string) string {
	return "source-projection-saved:" + strings.ToLower(strings.TrimSpace(commit))
}

// recordSourceProjectionSaved is reconcile's OnSave hook. It appends one
// recovery_recorded event per save not yet on the tape, and a guest-local
// fact. An error stops the reset (sourceproj leaves the tree as it was) and
// the Cast that asked is refused; a runtime without a tape keeps the fact
// only.
func (rt *Runtime) recordSourceProjectionSaved(ctx context.Context, saved []sourceproj.Saved) error {
	if rt == nil || len(saved) == 0 {
		return nil
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	// The guest-local fact names only the refs this reconcile made, so a retry
	// after a failed append does not grow the log.
	var created []sourceproj.Saved
	for _, s := range saved {
		if s.Created {
			created = append(created, s)
		}
	}
	if len(created) > 0 {
		rt.appendSourceProjectionFactRecord(sourceProjectionFact{
			At: now, Kind: sourceproj.FactSaved, Trigger: "reconcile", State: "saved", Saved: created,
		})
	}
	computerID := firstNonEmpty(strings.TrimSpace(rt.selfdevComputerID), strings.TrimSpace(rt.cfg.ComputerID))
	if rt.store == nil || rt.eventAppender == nil || computerID == "" {
		return nil
	}
	for _, s := range saved {
		key := sourceProjectionSavedIdempotency(s.Commit)
		if _, found, err := rt.store.EventByIdempotency(ctx, computerID, key); err != nil {
			return fmt.Errorf("source projection: look up the saved fact for %s: %w", s.Ref, err)
		} else if found {
			continue
		}
		payload, err := computerevent.CanonicalJSON(sourceProjectionSavedPayload{
			Fact: sourceproj.FactSaved, Ref: s.Ref, Commit: s.Commit, Kind: s.Kind, From: s.From, To: s.To,
		})
		if err != nil {
			return err
		}
		eventID, err := computerevent.NewEventID()
		if err != nil {
			return err
		}
		event := computerevent.Event{
			SchemaVersion: computerevent.SchemaVersionV1, EventID: eventID, ComputerID: computerID,
			EventKind: computerevent.EventRecoveryRecorded, OccurredAt: now,
			IdempotencyKey: key,
			ActorProfile:   agentprofile.Management, AuthorityRef: "guest-core:source-projection",
			PrivacyClass:      "owner",
			ProposedEffectRef: s.Commit,
			ReducerVersion:    computerevent.ReducerVersionV1,
		}
		if _, _, err := appendEventPayload(rt, ctx, event, computerevent.TransitionInput{}, payload, sourceProjectionSavedMediaType, "owner"); err != nil {
			return fmt.Errorf("source projection: put the saved %s on the tape: %w", s.Ref, err)
		}
	}
	return nil
}
