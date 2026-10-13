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

// recordsDesk reports whether a desk's conversations are on records:
// Texture with research (L3a) and with management (L3b), research with
// Texture, management with Texture. Engineering stays on packets until its
// landing.
func recordsDesk(profile string) bool {
	switch strings.TrimSpace(profile) {
	case agentprofile.Texture, agentprofile.Research, agentprofile.Management:
		return true
	}
	return false
}

// recordsRoute is the inbox a records desk addresses for the desk name or
// agent id it staged. ok is false for a route that stays on its packet
// path. Texture talks to research:<doc> and management:<owner>; research to
// texture:<doc>; management to texture:<doc>, which it must name (it holds
// conversations with every Texture of its owner).
func recordsRoute(fromAgentID, ownerID, toDesk string) (string, bool, error) {
	fromDesk, fromSubject, _ := strings.Cut(strings.TrimSpace(fromAgentID), ":")
	to := strings.TrimSpace(toDesk)
	toName, toSubject, hasSubject := strings.Cut(to, ":")
	if !hasSubject {
		toName = to
	}
	switch fromDesk {
	case agentprofile.Texture:
		if fromSubject == "" {
			return "", false, nil
		}
		switch toName {
		case agentprofile.Research:
			if hasSubject && toSubject != fromSubject {
				return "", false, nil
			}
			return agentprofile.Research + ":" + fromSubject, true, nil
		case agentprofile.Management:
			if strings.TrimSpace(ownerID) == "" || (hasSubject && toSubject != ownerID) {
				return "", false, nil
			}
			return agentprofile.Management + ":" + ownerID, true, nil
		}
	case agentprofile.Research:
		if fromSubject != "" && toName == agentprofile.Texture && (!hasSubject || toSubject == fromSubject) {
			return agentprofile.Texture + ":" + fromSubject, true, nil
		}
	case agentprofile.Management:
		if toName == agentprofile.Texture {
			if !hasSubject || strings.TrimSpace(toSubject) == "" {
				return "", false, fmt.Errorf("management addresses a Texture by its inbox: pass the record's from (texture:<doc>), not \"texture\"")
			}
			return agentprofile.Texture + ":" + strings.TrimSpace(toSubject), true, nil
		}
	}
	return "", false, nil
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
	if rt == nil || r.ledger == nil || r.rec == nil {
		return intents, nil
	}
	// The desk's inbox is its agent id; a run whose agent id names another
	// desk than its profile is not on a records route.
	if deskOfAgent, _, _ := strings.Cut(strings.TrimSpace(r.scope.DeskAgentID), ":"); deskOfAgent != strings.TrimSpace(r.rec.AgentProfile) {
		return intents, nil
	}
	var rest []yaegikernel.StagedIntent
	var records []types.CommitmentRecord
	if strings.TrimSpace(r.rec.AgentProfile) == agentprofile.Management && metadataStringValue(r.rec.Metadata, "request_source") == inboxRecordRequestSource {
		for _, in := range intents {
			if in.Kind == yaegikernel.IntentCast && strings.HasPrefix(strings.TrimSpace(in.ToDesk), agentprofile.Engineering) {
				// Cut 1 L3b moves management's conversations with Texture onto
				// records; its conversation with engineering moves in the
				// next landing, and until then an engineering assignment
				// needs the control path this run did not come from.
				return nil, fmt.Errorf("reduce: %s: engineering casts from a records turn are not available on this build (Cut 1 landing L3b-2 moves them onto records); report the execution request back to the Texture as blocked", in.LocalID)
			}
		}
	}
	var localIDs []yaegikernel.StagedIntent
	openRound := map[string]bool{}
	for _, in := range intents {
		toInbox, onRoute, routeErr := recordsRoute(r.scope.DeskAgentID, r.scope.OwnerID, in.ToDesk)
		if routeErr != nil && recordsIntent(in) {
			return nil, fmt.Errorf("reduce: %s: %w", in.LocalID, routeErr)
		}
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

// WakeResearchInbox starts a research turn for unread records; see
// WakeRecordsInbox.
func (rt *Runtime) WakeResearchInbox(ctx context.Context, ownerID, computerID, deskAgentID string) (*types.RunRecord, error) {
	return rt.WakeRecordsInbox(ctx, ownerID, computerID, deskAgentID)
}

// recordsInboxPrompts is the first user turn of a run a records desk starts
// for its inbox.
var recordsInboxPrompts = map[string]string{
	agentprofile.Research: "Your inbox has new records from Texture: read them with choir.Records() (each has seq, kind, from, conversation_id, round, objective and body). Answer in the same conversation with choir.ReportPacket(\"texture\", packet, \"\"); a packet whose work_disposition is completed or empty is your final Report and closes the round.",
	agentprofile.Management: "Your inbox has new records: read them with choir.Records() (each has seq, kind, from, conversation_id, round, objective and body; a Texture's Cast carries its packet in body). Answer each Texture in its own conversation with choir.ReportPacket(r.From, packet, \"\") where r.From is the Texture's inbox (texture:<doc>); a packet whose work_disposition is completed or empty is your final Report and closes the round that Texture's Cast opened.",
}

// WakeRecordsInbox starts a turn for a records desk (research or
// management) when it has unread records and no active run; an active run
// reads them in its next cell. A research run is its desk's lifecycle
// activation on its document's trajectory; a management run is the owner's
// persistent management desk, which spans documents and has no trajectory.
func (rt *Runtime) WakeRecordsInbox(ctx context.Context, ownerID, computerID, deskAgentID string) (*types.RunRecord, error) {
	if rt == nil || rt.store == nil {
		return nil, nil
	}
	profile, _, _ := strings.Cut(strings.TrimSpace(deskAgentID), ":")
	prompt, ok := recordsInboxPrompts[profile]
	if !ok {
		return nil, nil
	}
	var agent types.AgentRecord
	var err error
	if profile == agentprofile.Management {
		if deskAgentID != persistentManagementAgentID(ownerID) {
			return nil, nil
		}
		agent, err = rt.EnsurePersistentManagementAgent(ctx, ownerID)
	} else {
		agent, err = rt.store.GetAgentByScope(ctx, ownerID, computerID, deskAgentID)
	}
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("records inbox: load desk %s: %w", deskAgentID, err)
	}
	if _, found, err := rt.activeRunByAgent(ctx, ownerID, deskAgentID); err != nil {
		return nil, fmt.Errorf("records inbox: active run of %s: %w", deskAgentID, err)
	} else if found {
		return nil, nil
	}
	if unread, err := rt.inboxUnread(ctx, ownerID, computerID, deskAgentID); err != nil {
		return nil, fmt.Errorf("records inbox: unread of %s: %w", deskAgentID, err)
	} else if !unread {
		return nil, nil
	}
	metadata := map[string]any{
		runMetadataAgentProfile: profile,
		runMetadataAgentRole:    profile,
		runMetadataAgentID:      deskAgentID,
		runMetadataChannelID:    agent.ChannelID,
		"request_source":        inboxRecordRequestSource,
	}
	if profile == agentprofile.Research {
		doc, err := rt.store.GetLifecycleDocument(ctx, ownerID, computerID, agent.ChannelID)
		if err != nil {
			return nil, fmt.Errorf("records inbox: document %s: %w", agent.ChannelID, err)
		}
		metadata[runMetadataTrajectoryID] = doc.TrajectoryID
	}
	rec, err := rt.StartRunWithMetadata(ctx, prompt, ownerID, metadata)
	if err != nil {
		return nil, fmt.Errorf("records inbox: start run for %s: %w", deskAgentID, err)
	}
	if err := rt.CheckedActorDispatchError("initial_dispatch", rec.RunID); err != nil {
		return nil, fmt.Errorf("records inbox: dispatch run %s: %w", rec.RunID, err)
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

// recheckRecordsInbox is the post-turn re-check (decision D4). A record that
// arrived while the desk's run was active found the desk busy and woke
// nobody. After a completed turn, any unread record starts one more turn.
// After a failed or cancelled turn, only records committed after that turn
// began do: the records it already had stay unread and re-arm nothing, so a
// record that breaks the desk is read again only when a new record arrives,
// never in a loop. A crash closure re-arms nothing at all.
func (rt *Runtime) recheckRecordsInbox(ctx context.Context, rec *types.RunRecord) {
	if rt == nil || rt.store == nil || rec == nil || !rec.State.Terminal() || rec.Error == runFateInterruptedByRestart {
		return
	}
	if !recordsDesk(agentProfileForRun(rec)) {
		return
	}
	cursor, err := rt.store.ReaderCursor(ctx, rec.OwnerID, rec.ComputerID, rec.AgentID, rec.AgentID)
	if err != nil {
		log.Printf("runtime: records re-check after %s: %v", rec.RunID, err)
		return
	}
	head, err := rt.store.InboxHead(ctx, rec.OwnerID, rec.ComputerID, rec.AgentID)
	if err != nil || head <= cursor {
		return
	}
	if rec.State != types.RunCompleted {
		unread, err := rt.store.ListInboxRecords(ctx, rec.OwnerID, rec.ComputerID, rec.AgentID, cursor, head)
		if err != nil {
			return
		}
		late := false
		for _, r := range unread {
			if committed, perr := time.Parse(time.RFC3339Nano, r.Provenance.CommittedAt); perr == nil && committed.After(rec.CreatedAt) {
				late = true
				break
			}
		}
		if !late {
			return
		}
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
