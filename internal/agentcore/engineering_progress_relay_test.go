package agentcore

// Engineering's progress Reports reach the owner's Texture
// (docs/problems/engineering-reports-never-reach-texture-2026-10-24.md).
// Each test names the failure mode it pins (numbers are the problem
// record's "Failure modes of the fix").
//
//  1. a Report stops at management            -> ...MintsOneTextureRecordWhenOn
//  2. the switch is off and behavior changes  -> ...MintsNothingWhenOff
//  3. authority widened (open/close a round)  -> ...MintsOneTextureRecordWhenOn
//  4. Texture does not wake                   -> ...MintsOneTextureRecordWhenOn,
//     ...OnlyTheProgressRecordWakes
//  5. a crash boot wakes Texture              -> ...FinalReportIsNotRelayed
//  6. replay mints a second record            -> ...ReplayMintsOnce
//  8. the body is lost or altered             -> ...MintsOneTextureRecordWhenOn
//  9. Texture reads a "closure"               -> ...MintsOneTextureRecordWhenOn,
//     ...RendersInTextureFrame
// 7. no parent Texture / nothing to relay      -> ...RendersInTextureFrame

import (
	"context"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/texturewrite"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

const relayTestPacket = `{"schema_version":"coagent_source_packet.v1","kind":"evidence_update","summary":"Files hub built","claims":[{"text":"The Files hub compiles and routes by kind."}]}`

// relayEngineeringReduction is the engineering desk's records turn on a
// bound assignment.
func relayEngineeringReduction(rt *Runtime, seed store.EngineeringAssignmentSeed, run types.RunRecord, cell string) *rlmCallReduction {
	return &rlmCallReduction{active: true, mb: rt, st: rt.store, ledger: rt.store, rec: &run, scope: ReductionScope{
		FromAgentID: run.AgentID, DeskAgentID: run.AgentID, FromRole: agentprofile.Engineering, ChannelID: run.AgentID,
		RunID: run.RunID, OwnerID: seed.OwnerID, ComputerID: seed.ComputerID, CellID: cell,
	}}
}

func relayReport(local string) []yaegikernel.StagedIntent {
	return []yaegikernel.StagedIntent{{LocalID: local, Kind: yaegikernel.IntentReport, ToDesk: "management", Packet: relayTestPacket}}
}

func relaySwitch(t *testing.T, s *store.Store, seed store.EngineeringAssignmentSeed, on bool) {
	t.Helper()
	if err := s.PutComputerSetting(context.Background(), store.ComputerSetting{
		OwnerID: seed.OwnerID, ComputerID: seed.ComputerID, Key: store.SettingEngineeringProgressToTexture, Value: on,
	}); err != nil {
		t.Fatal(err)
	}
}

func relayTextureRecords(t *testing.T, s *store.Store, seed store.EngineeringAssignmentSeed) []types.CommitmentRecord {
	t.Helper()
	all, err := s.ListInboxRecords(context.Background(), seed.OwnerID, seed.ComputerID, seed.TextureInbox, 0, 1<<40)
	if err != nil {
		t.Fatal(err)
	}
	var out []types.CommitmentRecord
	for _, rec := range all {
		if strings.HasPrefix(rec.SenderInbox, "progress:") {
			out = append(out, rec)
		}
	}
	return out
}

func TestEngineeringProgressRelayMintsOneTextureRecordWhenOn(t *testing.T) {
	rt, s, _ := c1eRuntime(t)
	ctx := context.Background()
	seed, err := store.SeedEngineeringAssignmentAuthority(s, "owner-relay-on", rt.TextureComputerID(), 1)
	if err != nil {
		t.Fatal(err)
	}
	assignment, run := c1eOpenBound(t, s, seed, 0)
	relaySwitch(t, s, seed, true)
	parentBefore, err := s.ConversationState(ctx, seed.OwnerID, seed.ComputerID, seed.ParentConversationID)
	if err != nil {
		t.Fatal(err)
	}

	if err := relayEngineeringReduction(rt, seed, run, "cell-relay-1").commitTray(ctx, relayReport("rep"), 1); err != nil {
		t.Fatalf("engineering Report: %v", err)
	}

	// Management is still the addressee and still has the Report.
	toManagement := c1eFromEngineering(t, s, assignment)
	if len(toManagement) != 1 || toManagement[0].Final {
		t.Fatalf("management inbox: %+v; want the one progress Report", toManagement)
	}
	// Texture's inbox holds exactly one platform-authored progress record.
	got := relayTextureRecords(t, s, seed)
	if len(got) != 1 {
		t.Fatalf("texture inbox holds %d progress records, want 1", len(got))
	}
	rec := got[0]
	if rec.EmitterKind != types.CommitmentEmitterSystem || rec.Final || rec.Round != 0 || rec.Inbox != seed.TextureInbox {
		t.Fatalf("progress record is %+v; want a non-final round-0 system record to %s", rec, seed.TextureInbox)
	}
	if strings.HasPrefix(rec.SenderInbox, agentprofile.Engineering+":") || strings.HasPrefix(rec.SenderInbox, agentprofile.Management+":") {
		t.Fatalf("progress record is sent as a desk: %s", rec.SenderInbox)
	}
	// A Note directive: operational, so it joins no open-claim projection
	// (ResolveCommitments skips directives), and it carries the Report's
	// own packet, the bytes management reads.
	if rec.Directive == nil || rec.Directive.Subtype != types.CommitmentDirectiveNote || rec.Directive.Body != relayTestPacket || rec.Report != nil {
		t.Fatalf("progress record lost the Report's body: directive=%+v report=%+v", rec.Directive, rec.Report)
	}
	if view := inboxRecordView(rec); view.Kind != "progress" || view.Body != relayTestPacket {
		t.Fatalf("Texture's view is kind %q body %q; want a progress record with the packet", view.Kind, view.Body)
	}
	if !store.RecordWakesReader(rec) {
		t.Fatal("the progress record does not wake Texture")
	}
	// No round opened or closed: management's conversation with the Texture
	// is as it was.
	parentAfter, err := s.ConversationState(ctx, seed.OwnerID, seed.ComputerID, seed.ParentConversationID)
	if err != nil {
		t.Fatal(err)
	}
	if parentAfter.Open != parentBefore.Open || parentAfter.Round != parentBefore.Round {
		t.Fatalf("management-Texture conversation moved: %+v -> %+v", parentBefore, parentAfter)
	}
}

func TestEngineeringProgressRelayMintsNothingWhenOff(t *testing.T) {
	rt, s, _ := c1eRuntime(t)
	ctx := context.Background()
	seed, err := store.SeedEngineeringAssignmentAuthority(s, "owner-relay-off", rt.TextureComputerID(), 1)
	if err != nil {
		t.Fatal(err)
	}
	assignment, run := c1eOpenBound(t, s, seed, 0)
	// No setting stored: the default.
	head := c1eInboxHead(t, s, seed.OwnerID, seed.ComputerID, seed.TextureInbox)
	if err := relayEngineeringReduction(rt, seed, run, "cell-relay-off").commitTray(ctx, relayReport("rep"), 1); err != nil {
		t.Fatalf("engineering Report: %v", err)
	}
	if got := c1eFromEngineering(t, s, assignment); len(got) != 1 {
		t.Fatalf("management inbox: %d records, want the Report", len(got))
	}
	if got := c1eInboxHead(t, s, seed.OwnerID, seed.ComputerID, seed.TextureInbox); got != head {
		t.Fatalf("texture inbox moved %d -> %d with the switch off", head, got)
	}
	// Explicitly off is the same.
	relaySwitch(t, s, seed, false)
	if err := relayEngineeringReduction(rt, seed, run, "cell-relay-off-2").commitTray(ctx, relayReport("rep"), 1); err != nil {
		t.Fatalf("second engineering Report: %v", err)
	}
	if got := c1eInboxHead(t, s, seed.OwnerID, seed.ComputerID, seed.TextureInbox); got != head {
		t.Fatalf("texture inbox moved %d -> %d with the switch set off", head, got)
	}
}

func TestEngineeringProgressRelayReplayMintsOnce(t *testing.T) {
	rt, s, _ := c1eRuntime(t)
	ctx := context.Background()
	seed, err := store.SeedEngineeringAssignmentAuthority(s, "owner-relay-replay", rt.TextureComputerID(), 1)
	if err != nil {
		t.Fatal(err)
	}
	_, run := c1eOpenBound(t, s, seed, 0)
	relaySwitch(t, s, seed, true)
	reduce := relayEngineeringReduction(rt, seed, run, "cell-relay-replay")
	if err := reduce.commitTray(ctx, relayReport("rep"), 1); err != nil {
		t.Fatal(err)
	}
	// The same cell reduced again (crash recovery re-runs commitTray).
	if err := relayEngineeringReduction(rt, seed, run, "cell-relay-replay").commitTray(ctx, relayReport("rep"), 1); err != nil {
		t.Fatalf("replayed cell: %v", err)
	}
	if got := relayTextureRecords(t, s, seed); len(got) != 1 {
		t.Fatalf("a replayed cell left %d progress records, want 1", len(got))
	}
	// A different cell is a different Report and relays on its own.
	if err := relayEngineeringReduction(rt, seed, run, "cell-relay-second").commitTray(ctx, relayReport("rep"), 1); err != nil {
		t.Fatal(err)
	}
	if got := relayTextureRecords(t, s, seed); len(got) != 2 {
		t.Fatalf("a second Report left %d progress records, want 2", len(got))
	}
}

// The relay is for staged progress Reports. The final Report an assignment
// ends with (Complete, or a cancel) is management's to answer the Texture
// with, behind the D6 gate, and is not relayed.
func TestEngineeringProgressRelayFinalReportIsNotRelayed(t *testing.T) {
	rt, s, _ := c1eRuntime(t)
	seed, err := store.SeedEngineeringAssignmentAuthority(s, "owner-relay-final", rt.TextureComputerID(), 1)
	if err != nil {
		t.Fatal(err)
	}
	assignment, _ := c1eOpenBound(t, s, seed, 0)
	relaySwitch(t, s, seed, true)
	c1eCancel(t, rt, assignment, "ended", false)
	if got := relayTextureRecords(t, s, seed); len(got) != 0 {
		t.Fatalf("the final Report was relayed: %+v", got)
	}
}

// Only the progress record is a system record that wakes; the closure, job
// and clean-task system records still do not (D5: nothing resumes work
// after a crash).
func TestEngineeringProgressRelayOnlyTheProgressRecordWakes(t *testing.T) {
	closure := types.NewRoundClosureRecord("management:o", "engineering:a", 1, types.ClosureReasonInterruptedByRestart, "2026-10-24T00:00:00Z")
	if store.RecordWakesReader(closure) {
		t.Fatal("a crash round closure wakes its reader")
	}
	job := types.CommitmentRecord{EmitterKind: types.CommitmentEmitterSystem, Inbox: "engineering:a", SenderInbox: types.JobSenderInbox("j"), Job: &types.CommitmentJob{JobID: "j"}}
	if store.RecordWakesReader(job) {
		t.Fatal("a job end record wakes outside the engineering inbox's own path")
	}
	// A system Note that is not from the progress sender does not wake.
	other := types.CommitmentRecord{EmitterKind: types.CommitmentEmitterSystem, Inbox: "texture:d", SenderInbox: "other:x",
		Directive: &types.CommitmentDirective{Subtype: types.CommitmentDirectiveNote, Body: "x"}}
	if store.RecordWakesReader(other) {
		t.Fatal("an arbitrary system Note wakes its reader")
	}
}

// Texture's frame renders a progress record as the engineering claims it
// carries, labelled as a progress record, so the document's writer reads
// what engineering reported with no frame change.
func TestEngineeringProgressRelayRendersInTextureFrame(t *testing.T) {
	var source types.CommitmentRecord
	source.RecordID, source.SenderInbox, source.ComputerID = "cell-x:Report:rep", "engineering:a1", "c1"
	source.Prediction.Hypothesis = relayTestPacket
	relay, ok := types.NewEngineeringProgressRelay(source, "texture:doc-1")
	if !ok {
		t.Fatal("no relay built for a Report with a packet")
	}
	relay.InboxSeq = 1
	frame := texturewrite.RecordsFrom(textureRecordInputs([]yaegikernel.InboxRecord{inboxRecordView(relay)}))
	if len(frame) != 1 || frame[0].Kind != "progress" || frame[0].Summary != "Files hub built" ||
		len(frame[0].Claims) != 1 || frame[0].Claims[0].Text != "The Files hub compiles and routes by kind." {
		t.Fatalf("frame record = %+v", frame)
	}
	// A Report with nothing to relay, a non-engineering sender and a
	// non-Texture target build nothing.
	empty := source
	empty.Prediction.Hypothesis = ""
	if _, ok := types.NewEngineeringProgressRelay(empty, "texture:doc-1"); ok {
		t.Fatal("an empty Report was relayed")
	}
	if _, ok := types.NewEngineeringProgressRelay(source, "research:doc-1"); ok {
		t.Fatal("a relay was addressed to a non-Texture inbox")
	}
	fromManagement := source
	fromManagement.SenderInbox = "management:o"
	if _, ok := types.NewEngineeringProgressRelay(fromManagement, "texture:doc-1"); ok {
		t.Fatal("a management record was relayed")
	}
}
