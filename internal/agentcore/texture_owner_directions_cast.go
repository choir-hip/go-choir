package agentcore

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// The owner's words reach management verbatim (docs/problems/texture-
// retypes-owner-directions-2026-10-25.md, chosen fix). Texture used to
// retype an owner direction into a Go string to cast it: a backtick broke
// the cell and the retry paraphrased. A Texture Cast's packet now names the
// owner's revisions (owner_revision: an id or a list of ids) and the
// reducer attaches each one's words, read from the revision, as
// owner_directions. Management reads them on the Cast record; engineering
// reads the same body as the request its assignment is bound to
// (engineeringRequestView).
const (
	castOwnerRevisionKey   = "owner_revision"
	castOwnerDirectionsKey = "owner_directions"
	// castOwnerRevisionMax bounds the revisions one Cast names: the request
	// and the newest directions the request turn prints
	// (textureOwnerDirectionKeep there is 8).
	castOwnerRevisionMax = 8
	// castOwnerTextMax bounds the owner's words one Cast carries, well under
	// the tray's 256 KiB, so too much is refused by name and not by a size
	// error later (review 272 F4). The request turn prints a request to 16k
	// characters and each direction to 4k.
	castOwnerTextMax = 64 << 10
	// castPacketScanDepth bounds the walk for a forged owner_directions.
	castPacketScanDepth = 32
)

// castOwnerDirection is one owner revision's words on a Cast's packet.
type castOwnerDirection struct {
	RevisionID string `json:"revision_id"`
	// Kind is "request" for the document's first revision, "direction" for
	// an owner revise.
	Kind string `json:"kind"`
	At   string `json:"at,omitempty"`
	Text string `json:"text"`
}

// ownerRevisionScope is the document whose owner revisions a Texture cell
// may cast: the run's owner, computer, document and trajectory.
type ownerRevisionScope struct {
	OwnerID, ComputerID, DocID, TrajectoryID string
}

// castPacketObject decodes a Cast's packet when it is a JSON object.
func castPacketObject(raw string) (map[string]json.RawMessage, bool) {
	raw = strings.TrimSpace(raw)
	if !strings.HasPrefix(raw, "{") {
		return nil, false
	}
	var spec map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &spec); err != nil || spec == nil {
		return nil, false
	}
	return spec, true
}

// refuseCellOwnerDirections refuses a Cast whose packet sets
// owner_directions, at any depth and in any letter case: only the reducer
// writes them, so no cell, on any desk, labels its own text as the owner's.
func refuseCellOwnerDirections(in yaegikernel.StagedIntent) error {
	if in.Kind != yaegikernel.IntentCast || strings.TrimSpace(in.Statement) == "" {
		return nil
	}
	var packet any
	if err := json.Unmarshal([]byte(in.Statement), &packet); err != nil {
		return nil
	}
	if jsonHasKeyFold(packet, castOwnerDirectionsKey, castPacketScanDepth) {
		return fmt.Errorf("reduce: cast %s: owner_directions is the runtime's, written only on a Texture Cast that names the owner's revisions as owner_revision; never set or copy it (an engineer reads the Texture request its assignment serves)", in.LocalID)
	}
	return nil
}

// jsonHasKeyFold reports whether a decoded JSON value has an object key equal
// to key in any letter case, within depth levels. A value nested deeper is
// treated as having it, so the bound never lets a forged key through.
func jsonHasKeyFold(v any, key string, depth int) bool {
	switch t := v.(type) {
	case map[string]any:
		if depth <= 0 {
			return true
		}
		for k, child := range t {
			if strings.EqualFold(strings.TrimSpace(k), key) || jsonHasKeyFold(child, key, depth-1) {
				return true
			}
		}
	case []any:
		if depth <= 0 {
			return true
		}
		for _, child := range t {
			if jsonHasKeyFold(child, key, depth-1) {
				return true
			}
		}
	}
	return false
}

// attachCastOwnerDirections expands every Cast whose packet names
// owner_revision: each named revision must be an owner revision of scope's
// document, and its words go on the packet byte for byte. A Cast that names
// none is returned unchanged. Revisions are immutable, so a replayed cell
// attaches the same bytes. The caller's intents are not modified.
func attachCastOwnerDirections(intents []yaegikernel.StagedIntent, scope ownerRevisionScope, load func(string) (types.Revision, error)) ([]yaegikernel.StagedIntent, error) {
	var out []yaegikernel.StagedIntent
	for i, in := range intents {
		if in.Kind != yaegikernel.IntentCast {
			continue
		}
		spec, ok := castPacketObject(in.Statement)
		if !ok {
			continue
		}
		rawIDs, named := spec[castOwnerRevisionKey]
		if !named {
			continue
		}
		if err := refuseCellOwnerDirections(in); err != nil {
			return nil, err
		}
		ids, err := castOwnerRevisionIDs(rawIDs)
		if err != nil {
			return nil, fmt.Errorf("reduce: Texture cast %s: %w", in.LocalID, err)
		}
		directions := make([]castOwnerDirection, 0, len(ids))
		textBytes := 0
		for _, id := range ids {
			direction, err := ownerDirectionFromRevision(id, scope, load)
			if err != nil {
				return nil, fmt.Errorf("reduce: Texture cast %s: %w", in.LocalID, err)
			}
			directions = append(directions, direction)
			textBytes += len(direction.Text)
		}
		if textBytes > castOwnerTextMax {
			return nil, fmt.Errorf("reduce: Texture cast %s: the owner's words in the revisions named are %d bytes; a Cast carries at most %d: name fewer revisions in this Cast and the rest in another", in.LocalID, textBytes, castOwnerTextMax)
		}
		attached, err := marshalVerbatim(directions)
		if err != nil {
			return nil, fmt.Errorf("reduce: Texture cast %s: encode owner_directions: %w", in.LocalID, err)
		}
		spec[castOwnerDirectionsKey] = attached
		statement, err := marshalVerbatim(spec)
		if err != nil {
			return nil, fmt.Errorf("reduce: Texture cast %s: encode packet: %w", in.LocalID, err)
		}
		if out == nil {
			out = append([]yaegikernel.StagedIntent(nil), intents...)
		}
		out[i].Statement = string(statement)
	}
	if out == nil {
		return intents, nil
	}
	return out, nil
}

// castOwnerRevisionIDs reads owner_revision: one id or a list of ids, each
// named once, at most castOwnerRevisionMax.
func castOwnerRevisionIDs(raw json.RawMessage) ([]string, error) {
	var ids []string
	var one string
	if err := json.Unmarshal(raw, &one); err == nil {
		ids = []string{one}
	} else if err := json.Unmarshal(raw, &ids); err != nil {
		return nil, fmt.Errorf("owner_revision is a revision id or a list of them, not %s", truncateForError(string(raw)))
	}
	if len(ids) > castOwnerRevisionMax {
		return nil, fmt.Errorf("owner_revision names %d revisions; a Cast names at most %d", len(ids), castOwnerRevisionMax)
	}
	seen := map[string]bool{}
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			return nil, fmt.Errorf("owner_revision names an empty revision id")
		}
		if !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("owner_revision names no revision")
	}
	return out, nil
}

// ownerDirectionFromRevision is the authority check: the revision loads
// from this computer's store, is the run's owner's, belongs to this
// document and trajectory, was written by the owner, and carries an owner
// direction (its owner_prompt) or, as the document's first revision, the
// owner's request.
func ownerDirectionFromRevision(id string, scope ownerRevisionScope, load func(string) (types.Revision, error)) (castOwnerDirection, error) {
	if strings.TrimSpace(scope.DocID) == "" {
		return castOwnerDirection{}, fmt.Errorf("owner_revision %s: this run names no document", id)
	}
	rev, err := load(id)
	if errors.Is(err, store.ErrNotFound) {
		return castOwnerDirection{}, fmt.Errorf("owner_revision %s is no revision on this computer", id)
	}
	if err != nil {
		return castOwnerDirection{}, fmt.Errorf("owner_revision %s: read it: %w", id, err)
	}
	switch {
	case strings.TrimSpace(rev.RevisionID) != id:
		return castOwnerDirection{}, fmt.Errorf("owner_revision %s read back as %q", id, rev.RevisionID)
	case strings.TrimSpace(rev.OwnerID) != strings.TrimSpace(scope.OwnerID) || strings.TrimSpace(rev.ComputerID) != strings.TrimSpace(scope.ComputerID):
		return castOwnerDirection{}, fmt.Errorf("owner_revision %s is not this computer's owner's revision", id)
	case strings.TrimSpace(rev.DocID) != strings.TrimSpace(scope.DocID):
		return castOwnerDirection{}, fmt.Errorf("owner_revision %s is a revision of another document, not this one", id)
	case strings.TrimSpace(scope.TrajectoryID) == "":
		// An owner revise is lifecycle-bound; a run with no trajectory
		// cannot place one in it (review 272 F3).
		return castOwnerDirection{}, fmt.Errorf("owner_revision %s: this run names no trajectory", id)
	case strings.TrimSpace(rev.TrajectoryID) != strings.TrimSpace(scope.TrajectoryID):
		return castOwnerDirection{}, fmt.Errorf("owner_revision %s belongs to another trajectory, not this one", id)
	case rev.AuthorKind != types.AuthorUser:
		return castOwnerDirection{}, fmt.Errorf("owner_revision %s is not an owner revision (its author is %s)", id, rev.AuthorKind)
	}
	direction := castOwnerDirection{RevisionID: id}
	if !rev.CreatedAt.IsZero() {
		direction.At = rev.CreatedAt.UTC().Format(time.RFC3339)
	}
	var metadata map[string]any
	_ = json.Unmarshal(rev.Metadata, &metadata)
	// The words as stored, not trimmed again: the revise handler stored
	// them, and verbatim is byte for byte.
	ownerText := func(key string) string {
		text, _ := metadata[key].(string)
		if strings.TrimSpace(text) == "" {
			return ""
		}
		return text
	}
	switch {
	case ownerText("owner_prompt") != "":
		direction.Kind, direction.Text = "direction", ownerText("owner_prompt")
	case strings.TrimSpace(rev.ParentRevisionID) == "" && ownerText("seed_prompt") != "":
		direction.Kind, direction.Text = "request", ownerText("seed_prompt")
	case strings.TrimSpace(rev.ParentRevisionID) == "" && strings.TrimSpace(rev.Content) != "":
		direction.Kind, direction.Text = "request", rev.Content
	default:
		return castOwnerDirection{}, fmt.Errorf("owner_revision %s carries no owner direction (an owner edit, not a revise): describe the edit in your own words", id)
	}
	return direction, nil
}

// marshalVerbatim encodes v without HTML escaping, so the owner's <, > and &
// read as written in the packet a desk prints.
func marshalVerbatim(v any) (json.RawMessage, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return json.RawMessage(bytes.TrimRight(buf.Bytes(), "\n")), nil
}

func truncateForError(s string) string {
	if len(s) > 80 {
		return s[:80] + "…"
	}
	return s
}

// attachTextureOwnerDirections is attachCastOwnerDirections for a Texture
// cell's reduction: the scope is the run's, and revisions are read from
// the lifecycle store. The document is the desk's own (texture:<doc>); a
// run whose metadata names another document names none.
func (r *rlmCallReduction) attachTextureOwnerDirections(ctx context.Context, intents []yaegikernel.StagedIntent) ([]yaegikernel.StagedIntent, error) {
	scope := ownerRevisionScope{OwnerID: strings.TrimSpace(r.scope.OwnerID), ComputerID: strings.TrimSpace(r.scope.ComputerID)}
	if desk, subject, ok := strings.Cut(strings.TrimSpace(r.scope.DeskAgentID), ":"); ok && desk == agentprofile.Texture {
		scope.DocID = strings.TrimSpace(subject)
	}
	if r.rec != nil {
		runDoc := strings.TrimSpace(firstNonEmpty(metadataStringValue(r.rec.Metadata, "doc_id"), r.rec.ChannelID))
		switch {
		case scope.DocID == "":
			scope.DocID = runDoc
		case runDoc != "" && runDoc != scope.DocID:
			scope.DocID = ""
		}
		scope.TrajectoryID = strings.TrimSpace(firstNonEmpty(r.rec.TrajectoryID, metadataStringValue(r.rec.Metadata, "trajectory_id")))
	}
	return attachCastOwnerDirections(intents, scope, func(id string) (types.Revision, error) {
		if r.ledger == nil {
			return types.Revision{}, fmt.Errorf("no store to read it from")
		}
		return r.ledger.GetLifecycleRevision(ctx, scope.OwnerID, scope.ComputerID, id)
	})
}
