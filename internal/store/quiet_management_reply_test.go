package store

import (
	"testing"

	"github.com/yusefmosiah/go-choir/internal/deskmanifest"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// A management reply with no objective does not wake a research desk
// (research-desks-woken-by-management-replies-loop-2026-11-20). Ways it can
// fail, written before the code:
//  1. the reply wakes the desk, and a full run starts for nothing;
//  2. the predicate is wide enough to hide a record the desk should act on:
//     a Cast, an Ask, a directive, a Report body, a final Report, a system
//     record (a rebase notice, an owner act), or a reply from anyone but
//     management;
//  3. the rule leaks past research: a reply to a Texture or to management
//     stops waking.
func managementReply(id, research string) types.CommitmentRecord {
	rec := inboxRecord(id, deskmanifest.AdmitterInbox("owner"), research)
	rec.Kind = types.CommitmentKindReport
	rec.RelatedIDs = []string{"ask-or-escalation"}
	rec.Prediction = types.CommitmentPrediction{Hypothesis: "stop retrying; deliver your round's findings"}
	return rec
}

func TestManagementReplyWithNoObjectiveHasTheQuietShape(t *testing.T) {
	research := deskmanifest.ResearchInbox("doc-1")
	reply := managementReply("reply-1", research)
	// The shape is quiet; the wake decision also needs the related record
	// (agentcore.staleReplyJudge), so the store still mints the wake row.
	if !IsQuietManagementReply(reply) || !RecordWakesReader(reply) {
		t.Fatal("a management reply with no objective has the quiet shape; the store leaves the wake to the runtime")
	}
}

func TestRecordsThatCarryNewInformationStillWakeAResearchDesk(t *testing.T) {
	research := deskmanifest.ResearchInbox("doc-1")
	mgmt := deskmanifest.AdmitterInbox("owner")
	withObjective := inboxCast("cast-1", mgmt, research, types.CommitmentCast{})
	ask := inboxAsk("ask-1", mgmt, research)
	notice := inboxNote("note-1", mgmt, research)
	final := managementReply("final-1", research)
	final.Final = true
	body := managementReply("body-1", research)
	body.Report = &types.CommitmentReport{}
	directive := managementReply("dir-1", research)
	directive.Directive = &types.CommitmentDirective{Subtype: types.CommitmentDirectiveCast, Objective: "check the filing"}
	fromTexture := managementReply("tex-1", research)
	fromTexture.SenderInbox = "texture:doc-1"
	toTexture := managementReply("to-tex-1", "texture:doc-1")
	toManagement := managementReply("to-mgmt-1", mgmt)
	toManagement.SenderInbox = research
	noRelated := managementReply("free-1", research)
	noRelated.RelatedIDs = nil
	system := managementReply("sys-1", research)
	system.EmitterKind = types.CommitmentEmitterSystem
	for name, rec := range map[string]types.CommitmentRecord{
		"a Cast with an objective": withObjective, "an Ask": ask, "a directive notice": notice,
		"a final report": final, "a report with a body": body, "a directive with an objective": directive,
		"a reply from a Texture": fromTexture, "a reply to a Texture": toTexture,
		"a reply to management": toManagement, "a report that answers nothing": noRelated,
	} {
		if IsQuietManagementReply(rec) || !RecordWakesReader(rec) {
			t.Errorf("%s must not have the quiet shape and must wake its reader", name)
		}
	}
	// System records are never quiet; the owner's direction and a rebase
	// notice reach a Texture (their predicates are the system branch).
	if IsQuietManagementReply(system) {
		t.Error("a system record is never a quiet reply")
	}
	rebase, ok := types.NewStackRebaseNotice(types.StackRebaseNotice{TextureInbox: "texture:doc-1", OperationID: "op1", BootedBase: "b2", StackBase: "b1", Attempt: 1})
	if !ok || IsQuietManagementReply(rebase) || !RecordWakesReader(rebase) {
		t.Error("a rebase notice must still wake")
	}
	act, ok := types.NewSelfDevelopmentOwnerAct(types.SelfDevelopmentOwnerAct{TextureInbox: "texture:doc-1", OperationID: "op1", Act: types.OwnerActApprove, Body: "approved"})
	if !ok || IsQuietManagementReply(act) || !RecordWakesReader(act) {
		t.Error("an owner act must still wake")
	}
}
