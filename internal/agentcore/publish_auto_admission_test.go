package agentcore

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/disclosure"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Enforce flip S1, D4 (sections 6.3, 7.4, 8.1, 9), written first.
//
// The publication sink admits the document custodian's publish of a
// publish-auto story when the story's spec traces to an owner-authorized
// origin on the tape, for the capture, document and evidence labels only. The
// walk: the story's trajectory, the opening Cast (sent by the admitter), and
// the captures it names, each resolved to its source. Any break asks as before.
//
//	FM5  D4 admits a publish that is not publish-auto, a label outside the
//	     subset, a caster that is not the admitter, or a different actor.

type publishAutoFixture struct {
	*disclosureFixture
	n int
}

func newPublishAutoFixture(t *testing.T, slug string, uid uint32) *publishAutoFixture {
	f := &publishAutoFixture{disclosureFixture: newDisclosureFixture(t, slug, uid)}
	f.escalating()
	return f
}

// putSchedule stores a schedule under the owner's id as the owner route would
// (stamped) or as a desk or an older build did (unstamped).
func (f *publishAutoFixture) putSchedule(t *testing.T, source, stamp string) {
	t.Helper()
	sched := store.SourceSchedule{OwnerID: f.owner, ComputerID: f.computer, SourceID: source, IntervalSeconds: 900, State: store.SourceScheduleActive, Generation: 1, CreatedByOwnerAct: stamp}
	if err := f.rt.store.PutSourceSchedule(objectgraph.WithWriter(context.Background(), objectgraph.WriterOwner), sched); err != nil {
		t.Fatal(err)
	}
}

// openStory opens a story the way management does. sender is the Cast's
// sender inbox; publish is the Cast's publish mode; capture ids are looked up
// in the arrival records of the admitter's inbox.
func (f *publishAutoFixture) openStory(t *testing.T, sender, publish string, captures []string, related []string, emitter types.CommitmentEmitterKind) string {
	t.Helper()
	f.n++
	id := fmt.Sprintf("cell-pa%d:cast:c1", f.n)
	cast := types.CommitmentRecord{
		SchemaID: types.CommitmentRecordSchemaV1, RecordID: id, Kind: types.CommitmentKindDirective,
		Directive: &types.CommitmentDirective{Subtype: types.CommitmentDirectiveCast, Objective: "Harbor reopens"},
		Cast:      &types.CommitmentCast{Publish: publish, Captures: captures}, RelatedIDs: related,
		Inbox: types.StoryOpenInbox, SenderInbox: sender, EmitterKind: emitter,
		Discrepancy: types.DiscrepancyUnresolved, Provenance: types.CommitmentProvenance{AgentID: sender},
	}
	req := types.CommitInboxRecordsRequest{OwnerID: f.owner, ComputerID: f.computer, CommandID: "cell-records:" + id, Records: []types.CommitmentRecord{cast}}
	req.CommandDigest, _ = store.ComputeCommitInboxRecordsDigest(req)
	receipt, err := f.rt.store.CommitInboxRecords(context.Background(), req)
	if err != nil {
		t.Fatalf("open story: %v", err)
	}
	return strings.TrimPrefix(receipt.Records[0].Inbox, "texture:")
}

func (f *publishAutoFixture) arrival(t *testing.T, source string, ids ...string) {
	t.Helper()
	var items []types.CaptureArrival
	for _, id := range ids {
		items = append(items, types.CaptureArrival{CaptureID: id, Title: id, URL: "https://pa.example/" + id})
	}
	seedArrival(t, f.rt.store, f.owner, f.computer, source, items)
}

// publishes runs the publication sink as the given desk with the given labels
// and reports whether it asked.
func (f *publishAutoFixture) asks(t *testing.T, doc, desk string, labels ...string) bool {
	t.Helper()
	f.n++
	exec := f.run(t, fmt.Sprintf("run-pa-%d", f.n), desk, "x", labels...)
	err := f.rt.GateDisclosure(context.Background(), *exec.RunRecord, disclosure.SinkPublication, doc, "publish a revision")
	// A publish-auto story never waits on the owner (owner, 2026-11-08): a
	// label the admission does not take is refused with no owner ask.
	if err != nil && !strings.Contains(err.Error(), "waits on the owner") && !strings.Contains(err.Error(), "never asks the owner") {
		t.Fatalf("unexpected refusal: %v", err)
	}
	return err != nil
}

func TestPublishAutoOnAnOwnerStampedScheduleIsAdmittedForTheStoryLabels(t *testing.T) {
	f := newPublishAutoFixture(t, "pa1", 62861)
	defer disclosure.SetModeForTest(disclosure.ModeEnforce)()
	capture, document, evidence, story := types.OwnerPrivateLabel("capture"), types.OwnerPrivateLabel("document"), types.OwnerPrivateLabel("evidence"), types.OwnerPrivateLabel("story")
	f.putSchedule(t, "rss:pa-owner", "req-owner-1")
	f.arrival(t, "rss:pa-owner", "cap-pa-1")
	arrivalID := types.CaptureArrivalRecordID("rss:pa-owner", []string{"cap-pa-1"})
	manager := agentprofile.Management + ":" + f.owner

	// Through RelatedIDs, and by scanning the admitter's arrivals.
	for name, related := range map[string][]string{"related": {arrivalID}, "scanned": nil} {
		doc := f.openStory(t, manager, types.CastPublishAuto, []string{"cap-pa-1"}, related, types.CommitmentEmitterAgent)
		if f.asks(t, doc, agentprofile.Texture, capture, document, evidence, story, types.LabelWorldUntrusted) {
			t.Errorf("%s: a publish-auto story on an owner-stamped schedule asked", name)
		}
	}
	// A task's outward Cast is stamped with the admitter as its sender, so the
	// delegated path walks the same.
	doc := f.openStory(t, manager, types.CastPublishAuto, []string{"cap-pa-1"}, []string{arrivalID}, types.CommitmentEmitterAgent)

	// FM5: a label outside the subset still asks, with the rest admitted.
	for _, label := range []string{types.OwnerPrivateLabel("mail"), types.OwnerPrivateLabel("run_memory"), types.OwnerPrivateLabel("texture")} {
		if !f.asks(t, doc, agentprofile.Texture, capture, label) {
			t.Errorf("FM5: label %s rode a publish-auto admission", label)
		}
	}
	// FM5: another actor than the document custodian asks.
	if !f.asks(t, doc, agentprofile.Management, capture) {
		t.Error("FM5: a publish by a desk that is not the document custodian was admitted")
	}
	// The sink's other cells are untouched: the same labels at library release
	// still ask.
	f.n++
	exec := f.run(t, fmt.Sprintf("run-pa-lib-%d", f.n), agentprofile.Texture, "x", capture)
	if err := f.rt.GateDisclosure(context.Background(), *exec.RunRecord, disclosure.SinkLibraryRelease, doc, "release"); err == nil {
		t.Error("FM5: the admission reached the library release sink")
	}
}

func TestFM5APublishThatIsNotPublishAutoOrNotFromTheAdmitterIsNotAdmitted(t *testing.T) {
	f := newPublishAutoFixture(t, "pa2", 62862)
	defer disclosure.SetModeForTest(disclosure.ModeEnforce)()
	capture := types.OwnerPrivateLabel("capture")
	f.putSchedule(t, "rss:pa-owner2", "req-owner-2")
	f.arrival(t, "rss:pa-owner2", "cap-pb-1")
	manager := agentprofile.Management + ":" + f.owner

	// Not publish-auto: the story's Cast sets no publish mode.
	plain := f.openStory(t, manager, "", []string{"cap-pb-1"}, nil, types.CommitmentEmitterAgent)
	if !f.asks(t, plain, agentprofile.Texture, capture) {
		t.Error("a story that is not publish-auto was admitted")
	}
	// The caster is not the admitter. The store already refuses any other
	// desk's Cast to texture:new, so the walk's own check is pinned directly.
	cast := func(sender string, emitter types.CommitmentEmitterKind) types.CommitmentRecord {
		return types.CommitmentRecord{SenderInbox: sender, EmitterKind: emitter}
	}
	for _, caster := range []string{agentprofile.Research + ":" + f.owner, agentprofile.Engineering + ":assignment-x", agentprofile.Texture + ":doc-x", agentprofile.Management + ":another-owner", ""} {
		if castFromAdmitter(cast(caster, types.CommitmentEmitterAgent), f.owner) {
			t.Errorf("a publish-auto spec set by %q was admitted", caster)
		}
	}
	if !castFromAdmitter(cast(manager, types.CommitmentEmitterAgent), f.owner) || !castFromAdmitter(cast("", types.CommitmentEmitterOwner), f.owner) {
		t.Error("the admitter's own Cast, or the owner's act, was not admitted")
	}
	// A story that names no capture traces to no schedule.
	none := f.openStory(t, manager, types.CastPublishAuto, nil, nil, types.CommitmentEmitterAgent)
	if !f.asks(t, none, agentprofile.Texture, capture) {
		t.Error("a publish-auto story that names no capture was admitted")
	}
	// A capture no arrival record names traces to no source.
	orphan := f.openStory(t, manager, types.CastPublishAuto, []string{"cap-unknown"}, nil, types.CommitmentEmitterAgent)
	if !f.asks(t, orphan, agentprofile.Texture, capture) {
		t.Error("a publish-auto story citing an unknown capture was admitted")
	}
	// A document that is no story.
	if !f.asks(t, "doc-not-a-story", agentprofile.Texture, capture) {
		t.Error("a document that is not a story was admitted")
	}
}

// The owner, 2026-11-08: "The point of the paper is to publish sourced stories
// asap." A story is admitted when every capture it names resolves to a
// source; the schedule that polled the source, its stamp, or its absence is
// not checked. A capture that resolves to no source still asks.
func TestASourcedStoryIsAdmittedWhateverItsSchedule(t *testing.T) {
	f := newPublishAutoFixture(t, "pa3", 62863)
	defer disclosure.SetModeForTest(disclosure.ModeEnforce)()
	capture := types.OwnerPrivateLabel("capture")
	manager := agentprofile.Management + ":" + f.owner
	f.putSchedule(t, "rss:pa-desk", "")
	f.arrival(t, "rss:pa-desk", "cap-pc-1")
	doc := f.openStory(t, manager, types.CastPublishAuto, []string{"cap-pc-1"}, nil, types.CommitmentEmitterAgent)
	if f.asks(t, doc, agentprofile.Texture, capture) {
		t.Fatal("a story on an unstamped schedule asked")
	}
	f.arrival(t, "rss:pa-never", "cap-pc-2")
	doc2 := f.openStory(t, manager, types.CastPublishAuto, []string{"cap-pc-2"}, nil, types.CommitmentEmitterAgent)
	if f.asks(t, doc2, agentprofile.Texture, capture) {
		t.Fatal("a story on a source with no schedule asked")
	}
	// A capture no arrival record names is unsourced, and the publish asks.
	unsourced := f.openStory(t, manager, types.CastPublishAuto, []string{"cap-pc-1", "cap-pc-nowhere"}, nil, types.CommitmentEmitterAgent)
	if !f.asks(t, unsourced, agentprofile.Texture, capture) {
		t.Error("a story with an unsourced capture was admitted")
	}
}
