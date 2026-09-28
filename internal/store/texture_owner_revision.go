package store

import (
	"encoding/json"
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
// document head: the head is an owner-side revision that no Texture turn has
// consumed. Owner-side means a user edit (AuthorUser) or an agent-principal
// revision (AuthorAppAgent) that did not come from this desk's own authoring
// turn — a desk-authored head must never wake the desk, or every apply commit
// would respawn it. Returns the head revision and the reducer sequence of the
// artifact_head_advanced event that committed it (0 when the head predates
// event history, e.g. the initial revision).
func PendingTextureOwnerRevision(snapshot types.LifecycleSnapshot) (types.Revision, int64, bool) {
	head := snapshot.HeadRevision
	if snapshot.Trajectory.Status != types.TrajectoryLive ||
		strings.TrimSpace(head.RevisionID) == "" ||
		head.RevisionID != snapshot.Document.CurrentRevisionID ||
		!textureRevisionIsOwnerInput(head) {
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

// textureRevisionIsOwnerInput reports whether a head revision is owner-side
// input to the desk: user-authored, or agent-authored by anyone other than
// the bound desk itself. Two exclusions keep the widening honest:
//   - desk-authored revisions (metadata source = a desk write tool): waking on
//     the desk's own commit would self-respawn it forever;
//   - the trajectory's initial revision (ParentRevisionID empty): the seed is
//     already armed by the initial-work wake, and counting it as owner input
//     double-dispatches trajectory start.
func textureRevisionIsOwnerInput(rev types.Revision) bool {
	if rev.AuthorKind == types.AuthorUser {
		return true
	}
	if rev.AuthorKind != types.AuthorAppAgent {
		return false
	}
	if strings.TrimSpace(rev.ParentRevisionID) == "" {
		return false
	}
	source := ""
	if len(rev.Metadata) > 0 {
		var meta map[string]any
		if err := json.Unmarshal(rev.Metadata, &meta); err == nil {
			source, _ = meta["source"].(string)
		}
	}
	switch strings.TrimSpace(source) {
	case "texture_cell", "edit_texture", "apply_texture":
		return false
	default:
		return true
	}
}
