package agentcore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// Records as work state, Cut 1 landing L3a
// (docs/definitions/choir-reeng-cut1-authority-2026-10-12.md, "Cursor and
// wakes"): the Texture-research conversation runs on addressed records
// alone. A desk's inbox id is its desk agent id (texture:<doc>,
// research:<doc>); each cell shows the records in (cursor, head] and
// advances the cursor to that head in the same transaction as its acts.
// Management and engineering routes stay on packets until L3b.

// recordsDesk reports whether a desk's conversation with its counterpart is
// on records.
func recordsDesk(profile string) bool {
	switch strings.TrimSpace(profile) {
	case agentprofile.Texture, agentprofile.Research:
		return true
	}
	return false
}

// recordsCounterpartInbox is the inbox a records desk addresses for a desk
// name or agent id it staged: Texture talks to research:<doc>, research to
// texture:<doc>. ok is false for any other route (it stays on its packet
// path).
func recordsCounterpartInbox(fromAgentID, toDesk string) (string, bool) {
	fromDesk, doc, found := strings.Cut(strings.TrimSpace(fromAgentID), ":")
	if !found || strings.TrimSpace(doc) == "" {
		return "", false
	}
	to := strings.TrimSpace(toDesk)
	toName, toDoc, hasDoc := strings.Cut(to, ":")
	if hasDoc && toDoc != doc {
		return "", false
	}
	if !hasDoc {
		toName = to
	}
	switch {
	case fromDesk == agentprofile.Texture && toName == agentprofile.Research:
		return agentprofile.Research + ":" + doc, true
	case fromDesk == agentprofile.Research && toName == agentprofile.Texture:
		return agentprofile.Texture + ":" + doc, true
	}
	return "", false
}

// cellRecords is the range of a desk's inbox one cell reads.
type cellRecords struct {
	inbox   string
	after   int64
	through int64
	view    []yaegikernel.InboxRecord
}

// loadCellRecords reads the records in (cursor, head] of a records desk's
// inbox for one cell.
func (rt *Runtime) loadCellRecords(ctx context.Context, ownerID, computerID, deskAgentID string) (cellRecords, error) {
	inbox := strings.TrimSpace(deskAgentID)
	after, err := rt.store.ReaderCursor(ctx, ownerID, computerID, inbox, inbox)
	if err != nil {
		return cellRecords{}, fmt.Errorf("records: cursor of %s: %w", inbox, err)
	}
	head, err := rt.store.InboxHead(ctx, ownerID, computerID, inbox)
	if err != nil {
		return cellRecords{}, fmt.Errorf("records: head of %s: %w", inbox, err)
	}
	out := cellRecords{inbox: inbox, after: after, through: head}
	if head <= after {
		out.through = after
		return out, nil
	}
	recs, err := rt.store.ListInboxRecords(ctx, ownerID, computerID, inbox, after, head)
	if err != nil {
		return cellRecords{}, fmt.Errorf("records: list %s (%d, %d]: %w", inbox, after, head, err)
	}
	for _, rec := range recs {
		out.view = append(out.view, inboxRecordView(rec))
	}
	return out, nil
}

// cursorAdvance is the compare-and-set that marks this range read, or nil
// when the range is empty.
func (c cellRecords) cursorAdvance() *types.InboxCursorAdvance {
	if c.inbox == "" || c.through <= c.after {
		return nil
	}
	return &types.InboxCursorAdvance{ReaderAgentID: c.inbox, Inbox: c.inbox, ExpectedSeq: c.after, Seq: c.through}
}

// inboxRecordView is what a cell sees of one record.
func inboxRecordView(rec types.CommitmentRecord) yaegikernel.InboxRecord {
	view := yaegikernel.InboxRecord{
		Seq: rec.InboxSeq, RecordID: rec.RecordID, Kind: inboxRecordKind(rec), From: rec.SenderInbox,
		ConversationID: rec.ConversationID, Round: rec.Round, Final: rec.Final,
		Cast: rec.Cast, Report: rec.Report,
	}
	if t, err := time.Parse(time.RFC3339Nano, rec.Provenance.CommittedAt); err == nil {
		view.CreatedAt = t
	}
	switch {
	case rec.Directive != nil:
		view.Objective = rec.Directive.Objective
		view.Body = rec.Directive.Body
	case rec.Precommit != nil:
		view.Body = rec.Precommit.Question
	default:
		view.Body = rec.Prediction.Hypothesis
	}
	return view
}

func inboxRecordKind(rec types.CommitmentRecord) string {
	switch {
	case rec.EmitterKind == types.CommitmentEmitterSystem:
		return "closure"
	case rec.Directive != nil:
		return string(rec.Directive.Subtype)
	case rec.Kind == types.CommitmentKindPrecommit:
		return "ask"
	case rec.Kind == types.CommitmentKindReport && len(rec.RelatedIDs) > 0 && rec.Report == nil:
		return "reply"
	}
	return string(rec.Kind)
}

// recordsIntent reports whether a staged act is one a records desk sends as
// a record: the conversational acts addressed to its counterpart.
func recordsIntent(in yaegikernel.StagedIntent) bool {
	switch in.Kind {
	case yaegikernel.IntentCast, yaegikernel.IntentAsk, yaegikernel.IntentNote,
		yaegikernel.IntentReport, yaegikernel.IntentReply:
		return true
	}
	return false
}

// inboxRecordForIntent turns a staged act on a records route into its
// addressed record. hasOpenRound says whether the sender is the recipient
// of an open round in this conversation: only then can a Report be final.
func inboxRecordForIntent(scope ReductionScope, in yaegikernel.StagedIntent, toInbox string, hasOpenRound bool) (types.CommitmentRecord, error) {
	rec := commitmentRecordForIntent(scope, in)
	rec.Inbox = toInbox
	rec.SenderInbox = strings.TrimSpace(scope.DeskAgentID)
	rec.Addressee = toInbox
	rec.EmitterKind = types.CommitmentEmitterAgent
	rec.ComputerID = scope.ComputerID
	switch in.Kind {
	case yaegikernel.IntentCast:
		// Texture's Cast to research carries a packet as its third
		// argument; hypotheses in it become the Cast's typed hypotheses.
		cast, err := castBodyFromPacket(in.Statement)
		if err != nil {
			return types.CommitmentRecord{}, fmt.Errorf("reduce: %s: %w", in.LocalID, err)
		}
		rec.Cast = cast
	case yaegikernel.IntentReport:
		report, final := reportBodyFromPacket(in.Packet, in.Claim)
		rec.Report = report
		rec.Final = final && hasOpenRound
	}
	return rec, nil
}

// castBodyFromPacket lifts hypotheses a Cast's packet names into the typed
// Cast body. A packet without hypotheses is an empty Cast body; the packet
// itself stays the record's directive body.
func castBodyFromPacket(raw string) (*types.CommitmentCast, error) {
	raw = strings.TrimSpace(raw)
	cast := &types.CommitmentCast{}
	if raw == "" || raw == "null" {
		return cast, nil
	}
	var probe struct {
		Hypotheses    []types.CastHypothesis  `json:"hypotheses"`
		Uncertainties []types.CastUncertainty `json:"uncertainties"`
		Revisions     []types.CastRevision    `json:"revisions"`
		Withdrawals   []types.CastWithdrawal  `json:"withdrawals"`
	}
	if err := json.Unmarshal([]byte(raw), &probe); err != nil {
		return nil, fmt.Errorf("a Cast's packet is not JSON: %w", err)
	}
	cast.Hypotheses, cast.Uncertainties, cast.Revisions, cast.Withdrawals = probe.Hypotheses, probe.Uncertainties, probe.Revisions, probe.Withdrawals
	return cast, nil
}

// reportBodyFromPacket maps a report packet onto the typed Report body. The
// packet's work disposition marks the end of the round: completed or empty
// is a final Report, with that as its stop reason. Settled and dropped
// hypotheses ride the packet when the desk names them.
func reportBodyFromPacket(rawPacket, claim string) (*types.CommitmentReport, bool) {
	report := &types.CommitmentReport{}
	rawPacket = strings.TrimSpace(rawPacket)
	if rawPacket == "" {
		if c := strings.TrimSpace(claim); c != "" {
			report.Claims = []types.ReportClaim{{Text: c}}
		}
		return report, false
	}
	var packet struct {
		types.CoagentSourcePacketPayload
		Settled []types.HypothesisSettlement `json:"settled"`
		Dropped []types.HypothesisDrop       `json:"dropped"`
		Final   bool                         `json:"final"`
	}
	if err := json.Unmarshal([]byte(rawPacket), &packet); err != nil {
		return report, false
	}
	for _, c := range packet.Claims {
		report.Claims = append(report.Claims, types.ReportClaim{Text: c.Text, Sources: c.SourceIDs})
	}
	report.Settled, report.Dropped = packet.Settled, packet.Dropped
	final := packet.Final
	switch strings.TrimSpace(string(packet.WorkDisposition)) {
	case string(types.WorkItemCompleted), "empty":
		final = true
	}
	if final {
		report.StopReason = firstNonEmpty(string(packet.WorkDisposition), "final")
	}
	return report, final
}

// commitRecordsRoute commits a records desk's cell: its acts on the records
// route become addressed records and commit with the cursor advance over the
// range the cell read, as one transaction. It returns the intents that stay
// on their existing paths (ledger-only acts, other routes, other kinds).
func (r *rlmCallReduction) commitRecordsRoute(ctx context.Context, intents []yaegikernel.StagedIntent, receipt *ReductionReceipt) ([]yaegikernel.StagedIntent, error) {
	rt := r.rt()
	if rt == nil || r.ledger == nil {
		return intents, nil
	}
	var rest []yaegikernel.StagedIntent
	var records []types.CommitmentRecord
	var localIDs []yaegikernel.StagedIntent
	openRound := map[string]bool{}
	for _, in := range intents {
		toInbox, onRoute := recordsCounterpartInbox(r.scope.DeskAgentID, in.ToDesk)
		if !recordsIntent(in) || !onRoute {
			rest = append(rest, in)
			continue
		}
		conversation := types.InboxConversationID(r.scope.DeskAgentID, toInbox)
		open, seen := openRound[conversation]
		if !seen {
			state, err := r.ledger.ConversationState(ctx, r.scope.OwnerID, r.scope.ComputerID, conversation)
			if err != nil {
				return nil, fmt.Errorf("reduce: conversation %s: %w", conversation, err)
			}
			open = state.Open && state.RecipientInbox == r.scope.DeskAgentID
			openRound[conversation] = open
		}
		rec, err := inboxRecordForIntent(r.scope, in, toInbox, open)
		if err != nil {
			return nil, err
		}
		if rec.Final {
			openRound[conversation] = false
		}
		records = append(records, rec)
		localIDs = append(localIDs, in)
	}
	cursor := r.records.cursorAdvance()
	if len(records) == 0 && cursor == nil {
		return rest, nil
	}
	req := types.CommitInboxRecordsRequest{
		OwnerID: r.scope.OwnerID, ComputerID: r.scope.ComputerID,
		CommandID: "cell-records:" + r.scope.CellID, Records: records, Cursor: cursor,
	}
	digest, err := store.ComputeCommitInboxRecordsDigest(req)
	if err != nil {
		return nil, err
	}
	req.CommandDigest = digest
	committed, err := r.ledger.CommitInboxRecords(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("reduce: commit records: %w", err)
	}
	for i, in := range localIDs {
		seq := int64(0)
		if i < len(committed.Records) {
			seq = committed.Records[i].InboxSeq
		}
		receipt.Intents = append(receipt.Intents, ReducedIntent{LocalID: in.LocalID, Seq: uint64(seq), Kind: in.Kind})
	}
	return rest, nil
}

// inboxRecordRequestSource marks a run a records desk started because its
// inbox has unread records (Cut 1 L3a).
const inboxRecordRequestSource = "inbox_record"

// inboxUnread reports whether a desk's inbox head is past its cursor.
func (rt *Runtime) inboxUnread(ctx context.Context, ownerID, computerID, deskAgentID string) (bool, error) {
	cursor, err := rt.store.ReaderCursor(ctx, ownerID, computerID, deskAgentID, deskAgentID)
	if err != nil {
		return false, err
	}
	head, err := rt.store.InboxHead(ctx, ownerID, computerID, deskAgentID)
	if err != nil {
		return false, err
	}
	return head > cursor, nil
}

// WakeResearchInbox starts a research turn when the desk has unread records
// and no active run; an active run reads them in its next cell. The run is
// the desk's lifecycle activation on its document's trajectory.
func (rt *Runtime) WakeResearchInbox(ctx context.Context, ownerID, computerID, deskAgentID string) (*types.RunRecord, error) {
	if rt == nil || rt.store == nil {
		return nil, nil
	}
	agent, err := rt.store.GetAgentByScope(ctx, ownerID, computerID, deskAgentID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("research inbox: load desk %s: %w", deskAgentID, err)
	}
	if profile, _ := agentprofile.Canonical(agent.Profile); profile != agentprofile.Research {
		return nil, nil
	}
	if _, found, err := rt.activeRunByAgent(ctx, ownerID, deskAgentID); err != nil {
		return nil, fmt.Errorf("research inbox: active run of %s: %w", deskAgentID, err)
	} else if found {
		return nil, nil
	}
	if unread, err := rt.inboxUnread(ctx, ownerID, computerID, deskAgentID); err != nil {
		return nil, fmt.Errorf("research inbox: unread of %s: %w", deskAgentID, err)
	} else if !unread {
		return nil, nil
	}
	doc, err := rt.store.GetLifecycleDocument(ctx, ownerID, computerID, agent.ChannelID)
	if err != nil {
		return nil, fmt.Errorf("research inbox: document %s: %w", agent.ChannelID, err)
	}
	metadata := map[string]any{
		runMetadataAgentProfile: agentprofile.Research,
		runMetadataAgentRole:    agentprofile.Research,
		runMetadataAgentID:      deskAgentID,
		runMetadataChannelID:    agent.ChannelID,
		runMetadataTrajectoryID: doc.TrajectoryID,
		"request_source":        inboxRecordRequestSource,
	}
	prompt := "Your inbox has new records from Texture: read them with choir.Records() (each has seq, kind, from, conversation_id, round, objective, body). Answer in the same conversation with choir.ReportPacket(\"texture\", packet, \"\"); a packet whose work_disposition is completed or empty is your final Report and closes the round."
	rec, err := rt.StartRunWithMetadata(ctx, prompt, ownerID, metadata)
	if err != nil {
		return nil, fmt.Errorf("research inbox: start run for %s: %w", deskAgentID, err)
	}
	if err := rt.CheckedActorDispatchError("initial_dispatch", rec.RunID); err != nil {
		return nil, fmt.Errorf("research inbox: dispatch run %s: %w", rec.RunID, err)
	}
	return rec, nil
}

// InboxUnread reports whether a records desk has unread records, for the
// Texture controller's wake decision.
func (rt *Runtime) InboxUnread(ctx context.Context, ownerID, computerID, deskAgentID string) (bool, error) {
	if rt == nil || rt.store == nil {
		return false, nil
	}
	return rt.inboxUnread(ctx, ownerID, computerID, deskAgentID)
}

// recheckRecordsInbox is the post-turn re-check (decision D4): a record that
// arrived while the desk's run was active found the desk busy and woke
// nobody, so after a turn completes, unread records start one more turn. A
// failed or cancelled run re-arms nothing: a record that breaks the desk is
// read again only when a new record arrives, never in a loop.
func (rt *Runtime) recheckRecordsInbox(ctx context.Context, rec *types.RunRecord) {
	if rt == nil || rt.store == nil || rec == nil || rec.State != types.RunCompleted {
		return
	}
	profile := agentProfileForRun(rec)
	if !recordsDesk(profile) {
		return
	}
	unread, err := rt.inboxUnread(ctx, rec.OwnerID, rec.ComputerID, rec.AgentID)
	if err != nil {
		log.Printf("runtime: records re-check after %s: %v", rec.RunID, err)
		return
	}
	if !unread {
		return
	}
	content, _ := json.Marshal(map[string]string{"inbox": rec.AgentID, "recheck_after_run": rec.RunID})
	if err := rt.DispatchActor(ctx, rec.OwnerID, rec.ComputerID, rec.AgentID, "inbox_record", string(content), "", ""); err != nil {
		log.Printf("runtime: records re-check wake for %s after %s: %v", rec.AgentID, rec.RunID, err)
	}
}

// closeOpenRoundsAfterCrash is the crash boot's closure for records (Cut 1,
// "Restart"): every round open before the boot gets one system closure, a
// final Report addressed to the round's caster with the fate
// interrupted_by_restart. A closure wakes no one; a second boot finds no
// open round and writes nothing.
func (rt *Runtime) closeOpenRoundsAfterCrash(ctx context.Context) {
	ownerID := strings.TrimSpace(rt.selfdevRouteOwnerID)
	if ownerID == "" {
		ownerID = strings.TrimSpace(os.Getenv("CHOIR_OWNER_ID"))
	}
	computerID := strings.TrimSpace(rt.TextureComputerID())
	if ownerID == "" || computerID == "" {
		log.Printf("runtime: crash boot round closure skipped: owner or computer unknown")
		return
	}
	fold, err := rt.store.RebuildInboxHeads(ctx, ownerID, computerID)
	if err != nil {
		log.Printf("runtime: crash boot round closure: fold: %v", err)
		return
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	for _, conversation := range fold.Conversations {
		if !conversation.Open {
			continue
		}
		closure := types.NewRoundClosureRecord(conversation.CasterInbox, conversation.RecipientInbox, conversation.Round, types.ClosureReasonInterruptedByRestart, now)
		closure.ComputerID = computerID
		req := types.CommitInboxRecordsRequest{OwnerID: ownerID, ComputerID: computerID, CommandID: "boot-closure:" + closure.RecordID, Records: []types.CommitmentRecord{closure}}
		if req.CommandDigest, err = store.ComputeCommitInboxRecordsDigest(req); err != nil {
			log.Printf("runtime: crash boot round closure %s: %v", closure.RecordID, err)
			continue
		}
		if _, err := rt.store.CommitInboxRecords(ctx, req); err != nil {
			log.Printf("runtime: crash boot round closure %s: %v", closure.RecordID, err)
			continue
		}
		log.Printf("runtime: closed round %d of %s after a crash restart", conversation.Round, conversation.ConversationID)
	}
}
