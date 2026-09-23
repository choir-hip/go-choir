package store

import (
	"strings"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// TextureTurnConsumedHead reports whether a texture_turn_committed event on the
// tape ran against head revisionID without advancing it. A turn that advances
// the head consumes its base implicitly (the head no longer equals revisionID),
// so only non-advancing outcomes need this marker. The event tape is the single
// authority; no separate consumed table exists.
func TextureTurnConsumedHead(events []types.LifecycleEvent, revisionID string) bool {
	revisionID = strings.TrimSpace(revisionID)
	if revisionID == "" {
		return false
	}
	for _, event := range events {
		if event.Kind != types.LifecycleTextureTurnCommitted {
			continue
		}
		if len(event.ArtifactRefs) >= 2 && strings.TrimSpace(event.ArtifactRefs[1]) == revisionID {
			return true
		}
	}
	return false
}

// PendingTextureOwnerRevision derives the desk's owner-input trigger from the
// document head: the head is an owner-authored revision that no Texture turn
// has consumed. Returns the head revision and the reducer sequence of the
// artifact_head_advanced event that committed it (0 when the head predates
// event history, e.g. the initial revision).
func PendingTextureOwnerRevision(snapshot types.LifecycleSnapshot) (types.Revision, int64, bool) {
	head := snapshot.HeadRevision
	if snapshot.Trajectory.Status != types.TrajectoryLive ||
		strings.TrimSpace(head.RevisionID) == "" ||
		head.RevisionID != snapshot.Document.CurrentRevisionID ||
		head.AuthorKind != types.AuthorUser {
		return types.Revision{}, 0, false
	}
	if TextureTurnConsumedHead(snapshot.Events, head.RevisionID) {
		return types.Revision{}, 0, false
	}
	seq := int64(0)
	for _, event := range snapshot.Events {
		if event.Kind == types.LifecycleArtifactHeadAdvanced &&
			len(event.ArtifactRefs) >= 2 && strings.TrimSpace(event.ArtifactRefs[1]) == head.RevisionID {
			seq = event.ReducerSeq
		}
	}
	return head, seq, true
}
