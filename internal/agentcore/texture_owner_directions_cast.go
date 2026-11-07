package agentcore

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/deskmanifest"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
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
	// castOwnerDirectionsOmittedKey lists the trajectory's owner revisions
	// the runtime left off a Cast to stay inside the bounds (fix 5).
	castOwnerDirectionsOmittedKey = "owner_directions_omitted"
	// textureCastPacketMax bounds a Texture Cast's whole packet, owner
	// directions included, and is the bound engineering reads it to
	// (engineeringRequestBodyMax): one bound for the whole carry, so a
	// request Texture commits is never cut one hop later (fix 9,
	// docs/problems/selfdev-round-blockers-cluster-2026-11-03.md). It leaves
	// 32 KiB of packet beside the owner's words.
	textureCastPacketMax = castOwnerTextMax + 32<<10
	// castOwnerOmittedMax bounds the ids owner_directions_omitted names.
	castOwnerOmittedMax = 32
)

// castOwnerDirection is one owner revision's words on a Cast's packet.
type castOwnerDirection struct {
	RevisionID string `json:"revision_id"`
	// Kind is "request" for the document's first revision, "direction" for
	// an owner revise.
	Kind string `json:"kind"`
	At   string `json:"at,omitempty"`
	Text string `json:"text"`
	// Current marks the newest entry: the operative direction. The list is
	// oldest first, so a desk that indexes it by a round number gets an
	// older round's words (pool r4 took round 3's for round 4,
	// docs/problems/owner-words-restated-away-before-engineering-2026-11-03.md).
	Current bool `json:"current,omitempty"`
}

// orderOwnerDirections sorts directions oldest first by their time, the
// request before a direction of the same time and otherwise in the order
// given, whatever order a Cast named them in, and marks the newest current.
func orderOwnerDirections(directions []castOwnerDirection) {
	at := func(d castOwnerDirection) time.Time {
		t, _ := time.Parse(time.RFC3339Nano, d.At)
		return t
	}
	sort.SliceStable(directions, func(i, j int) bool {
		ti, tj := at(directions[i]), at(directions[j])
		if !ti.Equal(tj) {
			return ti.Before(tj)
		}
		return directions[i].Kind == "request" && directions[j].Kind != "request"
	})
	for i := range directions {
		directions[i].Current = i == len(directions)-1
	}
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
	return attachCastOwnerHistory(intents, scope, load, nil)
}

// attachCastOwnerHistory is attachCastOwnerDirections that also carries the
// owner's earlier words on the same trajectory (fix 5): list returns the
// document's revisions, and every owner revision of this trajectory that
// carries owner words and is no newer than the newest named one is attached
// too, newest first while the bounds allow, the rest named under
// owner_directions_omitted. The named ones are required, as before. The
// cut-off is the named revisions' own time, so a replayed cell attaches the
// same set. A nil list attaches only what is named.
func attachCastOwnerHistory(intents []yaegikernel.StagedIntent, scope ownerRevisionScope, load func(string) (types.Revision, error), list func() ([]types.Revision, error)) ([]yaegikernel.StagedIntent, error) {
	return attachCastOwnerHistoryAt(intents, scope, load, list, time.Time{})
}

// attachCastOwnerHistoryAt is attachCastOwnerHistory that also attaches, to a
// Cast to the admission holder that names no revision, the owner's newest
// words written at or before cutoff (X4, MA4.10). commitTray runs it again on
// crash recovery, so the cut-off must be something the replay reproduces: the
// caller passes the run's start. A zero cut-off attaches nothing.
func attachCastOwnerHistoryAt(intents []yaegikernel.StagedIntent, scope ownerRevisionScope, load func(string) (types.Revision, error), list func() ([]types.Revision, error), cutoff time.Time) ([]yaegikernel.StagedIntent, error) {
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
			// X4 (MA4.10): a Cast to the desk that admits assignments, which
			// names no revision (the restore round), still carries the owner's
			// newest words, attached here once so no run context re-reads the
			// tape at every render. Revisions listed now are the ones written
			// at or before the commit.
			if out2, ok, err := attachNewestOwnerWords(out, intents, i, spec, scope, list, cutoff); err != nil {
				return nil, err
			} else if ok {
				out = out2
			}
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
		var omitted []string
		if list != nil {
			earlier, left, err := earlierOwnerDirections(ids, directions, textBytes, scope, load, list)
			if err != nil {
				return nil, fmt.Errorf("reduce: Texture cast %s: %w", in.LocalID, err)
			}
			directions, omitted = append(earlier, directions...), left
		}
		orderOwnerDirections(directions)
		attached, err := marshalVerbatim(directions)
		if err != nil {
			return nil, fmt.Errorf("reduce: Texture cast %s: encode owner_directions: %w", in.LocalID, err)
		}
		spec[castOwnerDirectionsKey] = attached
		// Only the runtime names what it left out: a cell's own value goes.
		delete(spec, castOwnerDirectionsOmittedKey)
		delete(spec, castOwnerDirectionsOmittedKey+"_count")
		if len(omitted) > 0 {
			if len(omitted) > castOwnerOmittedMax {
				// The newest left out are named; the count is all of them.
				if spec[castOwnerDirectionsOmittedKey+"_count"], err = marshalVerbatim(len(omitted)); err != nil {
					return nil, err
				}
				omitted = omitted[len(omitted)-castOwnerOmittedMax:]
			}
			if spec[castOwnerDirectionsOmittedKey], err = marshalVerbatim(omitted); err != nil {
				return nil, fmt.Errorf("reduce: Texture cast %s: encode %s: %w", in.LocalID, castOwnerDirectionsOmittedKey, err)
			}
		}
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

// attachNewestOwnerWords is the X4 attach for a Cast that names no owner
// revision: when the Cast is to the holder of the admission ledger and the
// document has an owner revision of this trajectory that carries words, the
// newest one goes on the packet as the only owner_directions entry, marked
// current. It reports false, and changes nothing, otherwise.
func attachNewestOwnerWords(out, intents []yaegikernel.StagedIntent, i int, spec map[string]json.RawMessage, scope ownerRevisionScope, list func() ([]types.Revision, error), cutoff time.Time) ([]yaegikernel.StagedIntent, bool, error) {
	in := intents[i]
	if list == nil || cutoff.IsZero() {
		return out, false, nil
	}
	// The resolved destination decides: a Cast to the admission holder, or to
	// the assignment desk, which the resolver forwards to the holder's inbox
	// for a sender that does not admit (Texture).
	if !castsToAdmission(in.ToDesk) {
		return out, false, nil
	}
	revisions, err := list()
	if err != nil {
		return nil, false, fmt.Errorf("reduce: Texture cast %s: list the document's revisions: %w", in.LocalID, err)
	}
	var best castOwnerDirection
	var bestAt time.Time
	found := false
	for _, rev := range revisions {
		if rev.AuthorKind != types.AuthorUser || rev.CreatedAt.IsZero() || rev.CreatedAt.After(cutoff) {
			continue
		}
		d, err := ownerDirectionFromRevision(strings.TrimSpace(rev.RevisionID), scope, func(string) (types.Revision, error) { return rev, nil })
		if err != nil || strings.TrimSpace(d.Text) == "" {
			continue // another trajectory's, or an owner edit without words
		}
		if !found || rev.CreatedAt.After(bestAt) || (rev.CreatedAt.Equal(bestAt) && d.RevisionID > best.RevisionID) {
			best, bestAt, found = d, rev.CreatedAt, true
		}
	}
	if !found {
		return out, false, nil
	}
	if len(best.Text) > castOwnerTextMax {
		return nil, false, fmt.Errorf("reduce: Texture cast %s: the owner's newest words (%d bytes) exceed the %d a Cast carries; shorten the owner's revision or point at its files in the source tree", in.LocalID, len(best.Text), castOwnerTextMax)
	}
	best.Current = true
	attached, err := marshalVerbatim([]castOwnerDirection{best})
	if err != nil {
		return nil, false, fmt.Errorf("reduce: Texture cast %s: encode owner_directions: %w", in.LocalID, err)
	}
	spec[castOwnerDirectionsKey] = attached
	statement, err := marshalVerbatim(spec)
	if err != nil {
		return nil, false, fmt.Errorf("reduce: Texture cast %s: encode packet: %w", in.LocalID, err)
	}
	if out == nil {
		out = append([]yaegikernel.StagedIntent(nil), intents...)
	}
	out[i].Statement = string(statement)
	return out, true, nil
}

// earlierOwnerDirections picks the owner's earlier words for a Cast that
// named ids (whose directions and total size are given): every other owner
// revision of the scope's document and trajectory that carries owner words
// and is no newer than the newest named one, newest first while the Cast
// stays within castOwnerRevisionMax revisions and castOwnerTextMax bytes. It
// returns them oldest first, and the ids it left out, oldest first.
func earlierOwnerDirections(ids []string, named []castOwnerDirection, textBytes int, scope ownerRevisionScope, load func(string) (types.Revision, error), list func() ([]types.Revision, error)) ([]castOwnerDirection, []string, error) {
	// The cut-off is the named revisions' own time, read from the same store
	// they were checked against, never from the listing (which may lag or
	// be cut), so a replayed cell computes the same one.
	seen := map[string]bool{}
	var cutoff time.Time
	for _, id := range ids {
		seen[id] = true
		rev, err := load(id)
		if err != nil {
			return nil, nil, fmt.Errorf("owner_revision %s: read it: %w", id, err)
		}
		if rev.CreatedAt.After(cutoff) {
			cutoff = rev.CreatedAt
		}
	}
	if cutoff.IsZero() {
		return nil, nil, nil
	}
	revisions, err := list()
	if err != nil {
		return nil, nil, fmt.Errorf("list the document's revisions: %w", err)
	}
	type candidate struct {
		rev       types.Revision
		direction castOwnerDirection
	}
	var candidates []candidate
	for _, rev := range revisions {
		id := strings.TrimSpace(rev.RevisionID)
		if id == "" || seen[id] || rev.AuthorKind != types.AuthorUser || rev.CreatedAt.After(cutoff) {
			continue
		}
		direction, err := ownerDirectionFromRevision(id, scope, func(string) (types.Revision, error) { return rev, nil })
		if err != nil {
			continue // another trajectory's, or an owner edit without words
		}
		seen[id] = true
		candidates = append(candidates, candidate{rev: rev, direction: direction})
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if !candidates[i].rev.CreatedAt.Equal(candidates[j].rev.CreatedAt) {
			return candidates[i].rev.CreatedAt.After(candidates[j].rev.CreatedAt)
		}
		return candidates[i].rev.RevisionID > candidates[j].rev.RevisionID
	})
	count := len(named)
	var kept []castOwnerDirection
	var omitted []string
	for _, c := range candidates {
		if count < castOwnerRevisionMax && textBytes+len(c.direction.Text) <= castOwnerTextMax {
			kept = append(kept, c.direction)
			count++
			textBytes += len(c.direction.Text)
			continue
		}
		omitted = append(omitted, c.direction.RevisionID)
	}
	for i, j := 0, len(kept)-1; i < j; i, j = i+1, j-1 {
		kept[i], kept[j] = kept[j], kept[i]
	}
	for i, j := 0, len(omitted)-1; i < j; i, j = i+1, j-1 {
		omitted[i], omitted[j] = omitted[j], omitted[i]
	}
	return kept, omitted, nil
}

// castsToAdmission reports whether a Texture Cast's address resolves to the
// admission holder's inbox: it names the holder, or the assignment desk, which
// the resolver forwards to the holder for a sender that does not admit. The
// attach and the packet bound share it so they cannot drift.
func castsToAdmission(toDesk string) bool {
	holder, ok := deskmanifest.GrantHolder(deskmanifest.GrantAdmitAssignment)
	target := castTargetDesk(toDesk)
	return ok && (target == holder.Name || deskmanifest.IsAssignmentPerformer(target))
}

// refuseOversizedTextureCasts refuses a Texture Cast to management whose
// packet, with what the runtime attached, is over textureCastPacketMax:
// engineering, which reads the Texture Cast its assignment serves, reads
// only that much (fix 9). Casts to other desks keep their own bounds.
func refuseOversizedTextureCasts(intents []yaegikernel.StagedIntent) error {
	for _, in := range intents {
		if in.Kind == yaegikernel.IntentCast && castsToAdmission(in.ToDesk) && len(in.Statement) > textureCastPacketMax {
			return fmt.Errorf("reduce: Texture cast %s: the packet is %d bytes, with any owner words the runtime attached; a Cast to management carries at most %d, which is what engineering reads: shorten the packet, name fewer owner revisions, or point at the request's files in the source tree", in.LocalID, len(in.Statement), textureCastPacketMax)
		}
	}
	return nil
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
		// To the nanosecond, so two revisions of one second still order
		// (orderOwnerDirections); a whole second prints as before.
		direction.At = rev.CreatedAt.UTC().Format(time.RFC3339Nano)
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
	// X5 (MA3.7b, E3): the reducer acts as Texture's agent for this one read.
	// The document and its revisions are Texture's, the directions are the
	// owner's words carried on the Cast, and no cell reads them here, so the
	// read is declared as the custodian and is no crossing.
	ctx = objectgraph.WithCustodianReader(ctx, "choir.texture_revision")
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
	var cutoff time.Time
	if r.rec != nil {
		cutoff = r.rec.CreatedAt
	}
	out, err := attachCastOwnerHistoryAt(intents, scope, func(id string) (types.Revision, error) {
		if r.ledger == nil {
			return types.Revision{}, fmt.Errorf("no store to read it from")
		}
		return r.ledger.GetLifecycleRevision(ctx, scope.OwnerID, scope.ComputerID, id)
	}, func() ([]types.Revision, error) {
		if r.ledger == nil || strings.TrimSpace(scope.DocID) == "" {
			return nil, nil
		}
		// No cut: the listing is read whole, so a long document's oldest
		// owner words are not silently lost (the default keeps 1000).
		return r.ledger.ListTextureRevisionsByScopeOG(ctx, scope.OwnerID, scope.ComputerID, scope.DocID, 1<<30)
	}, cutoff)
	if err != nil {
		return nil, err
	}
	if err := refuseOversizedTextureCasts(out); err != nil {
		return nil, err
	}
	return out, nil
}
