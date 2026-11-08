package agentcore

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/deskmanifest"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// The publish-auto admission (enforce flip D4; sections 6.3, 7.4, 8.1 and 9).
//
// A story whose Cast set publish auto is a standing owner intent ("we need
// paper publishing 24/7"). Its captures are public web captures that carry the
// owner_private:capture floor only because they pass through the owner's
// capture store, and its Texture run carries the document and evidence labels
// by construction. The publication sink admits the document custodian's
// publish of such a story for those labels, and only when the story's spec
// traces to an owner-authorized origin on the tape:
//
//  1. the story's trajectory is a publish-auto story opened by the admitter;
//  2. its opening Cast record was sent by the admitter's inbox (a task's
//     outward Cast is stamped with the admitter, outwardSender), or by the
//     owner;
//  3. every capture the Cast names resolves, through the arrival records, to
//     its source: the story is sourced.
//
// Any break in the walk refuses the admission, and the publish asks as before.
// The schedule that polled a source is not checked (owner, 2026-11-08: "The
// point of the paper is to publish sourced stories asap"): the labels are the
// guard, and since S2a an intake task's labels are its captures' own, so
// nothing else management read rides into a publish-auto story.

// publishAutoLabels are the labels a publish-auto Texture run carries by
// construction: its frame shows the story document, its evidence derives from
// the captures, and each capture read carries the capture floor. Every other
// owner-private label still asks.
var publishAutoLabels = map[string]bool{
	types.OwnerPrivateLabel("capture"):  true,
	types.OwnerPrivateLabel("document"): true,
	types.OwnerPrivateLabel("evidence"): true,
	// The story document is the content being published: a Texture run on a
	// story reads the story index and its documents (ActionStories).
	types.OwnerPrivateLabel("story"): true,
}

// publishAutoCaptureScan bounds the admitter-inbox scan that resolves a capture
// the Cast's related ids did not name.
const publishAutoCaptureScan = 2000

// publishAutoAdmission returns the publication sink's admission for one
// attempt: a label is admitted when the actor is the document custodian, the
// label is in the subset and the provenance walk of the story holds. The walk
// runs once per attempt, and only when a label asks for it.
func (rt *Runtime) publishAutoAdmission(ctx context.Context, call disclosureCall) func(label string) bool {
	var (
		walked, holds bool
	)
	return func(label string) bool {
		if !publishAutoLabels[label] || rt == nil || rt.store == nil {
			return false
		}
		if !isDocumentCustodian(call.deskRole) {
			return false
		}
		if !walked {
			walked = true
			ok, why := rt.publishAutoProvenance(ctx, strings.TrimSpace(call.exec.OwnerID), strings.TrimSpace(call.exec.ComputerID), strings.TrimSpace(call.target))
			holds = ok
			if !ok {
				log.Printf("disclosure: publish of %s is not admitted as publish-auto: %s", call.target, why)
			}
		}
		return holds
	}
}

// isDocumentCustodian reports whether the desk is the custodian of the owner's
// documents, resolved through the registry.
func isDocumentCustodian(deskRole string) bool {
	custodian, found := objectgraph.CustodianOf("choir.texture_document")
	w, ok := objectgraph.DeskWriter(canonicalDeskName(strings.TrimSpace(deskRole)))
	return found && ok && w == custodian
}

// castFromAdmitter reports whether a story's opening Cast was sent by the
// owner's admitter inbox, or is the owner's own act.
func castFromAdmitter(cast types.CommitmentRecord, ownerID string) bool {
	if cast.EmitterKind == types.CommitmentEmitterOwner {
		return true
	}
	return strings.TrimSpace(cast.SenderInbox) == deskmanifest.AdmitterInbox(ownerID)
}

// publishAutoProvenance walks the tape from the story document to the owner's
// schedule. why names the break.
func (rt *Runtime) publishAutoProvenance(ctx context.Context, ownerID, computerID, docID string) (bool, string) {
	if ownerID == "" || docID == "" {
		return false, "no owner or document"
	}
	if computerID == "" {
		computerID = strings.TrimSpace(rt.cfg.ComputerID)
	}
	ctx = withKernelReader(ctx)
	doc, err := rt.store.GetLifecycleDocument(ctx, ownerID, computerID, docID)
	if err != nil || strings.TrimSpace(doc.TrajectoryID) == "" {
		return false, "the document is no story"
	}
	trajectory, err := rt.store.GetLifecycleTrajectory(ctx, ownerID, computerID, doc.TrajectoryID)
	if err != nil {
		return false, "its trajectory could not be read"
	}
	if !types.StoryPublishesAuto(trajectory) {
		return false, "the story is not publish-auto"
	}
	if trajectory.SubjectRefs[types.StoryRefOriginDesk] != deskmanifest.AdmitterName() {
		return false, "the story was not opened by the admitter"
	}
	openingID := strings.TrimSpace(trajectory.SubjectRefs[types.StoryRefOpeningRecord])
	if openingID == "" {
		return false, "the story names no opening record"
	}
	cast, err := rt.store.GetCommitmentRecord(ctx, ownerID, computerID, openingID)
	if err != nil || cast == nil {
		return false, "the opening Cast could not be read"
	}
	if cast.Cast == nil || cast.Cast.Publish != types.CastPublishAuto {
		return false, "the opening Cast did not set publish auto"
	}
	if !castFromAdmitter(*cast, ownerID) {
		return false, "the opening Cast was not sent by the admitter"
	}
	if cast.EmitterKind == types.CommitmentEmitterOwner {
		// An owner act that sets publish-auto directly is the origin itself.
		return true, ""
	}
	if _, err := rt.castCaptureSources(ctx, ownerID, computerID, *cast); err != nil {
		return false, err.Error()
	}
	return true, ""
}

// castCaptureSources resolves every capture the Cast names to its source: from
// the arrival records the Cast relates, else from the admitter's inbox. Every
// capture must resolve, and an event record's member resolves by the member's
// own source.
func (rt *Runtime) castCaptureSources(ctx context.Context, ownerID, computerID string, cast types.CommitmentRecord) (map[string]bool, error) {
	want := map[string]bool{}
	for _, id := range cast.Cast.Captures {
		if id = strings.TrimSpace(id); id != "" {
			want[id] = true
		}
	}
	if len(want) == 0 {
		return nil, fmt.Errorf("the opening Cast names no capture")
	}
	found := map[string]string{}
	take := func(rec *types.CommitmentRecord) {
		if rec == nil || rec.Captures == nil {
			return
		}
		for _, item := range rec.Captures.Items {
			id := strings.TrimSpace(item.CaptureID)
			if !want[id] || found[id] != "" {
				continue
			}
			source := strings.TrimSpace(item.SourceID)
			if source == "" {
				source = strings.TrimSpace(rec.Captures.SourceID)
			}
			if source == "" || strings.HasPrefix(source, "event:") {
				continue
			}
			found[id] = source
		}
	}
	for _, id := range cast.RelatedIDs {
		if rec, err := rt.store.GetCommitmentRecord(ctx, ownerID, computerID, id); err == nil {
			take(rec)
		}
	}
	if len(found) < len(want) {
		recs, err := rt.store.ListCommitmentRecords(ctx, ownerID, computerID, deskmanifest.AdmitterInbox(ownerID), publishAutoCaptureScan)
		if err != nil {
			return nil, fmt.Errorf("the arrival records could not be read")
		}
		for i := range recs {
			take(&recs[i])
		}
	}
	sources := map[string]bool{}
	for id := range want {
		source := found[id]
		if source == "" {
			return nil, fmt.Errorf("capture %s names no source in the arrival records", id)
		}
		sources[source] = true
	}
	return sources, nil
}
