package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Inbox records (Cut 1 L3a, definitions/choir-reeng-cut1-authority
// artifacts 1 and 2). Work state is addressed commitment records: each
// takes the next number in its inbox, a conversation id from its inbox
// pair and a round from that conversation, and mints one wake row for the
// recipient. Three objects beside the records are caches of their fold,
// written only here and rebuilt by RebuildInboxHeads:
//
//   - the inbox head (last sequence taken), advanced by compare-and-set
//     like the arrival ordinal (lifecycle.go nextArrivalOrdinal);
//   - the conversation head (round, open flag, caster, hypothesis states);
//   - the reader cursor, the one fact a reader owns.
//
// The cursor is a dedicated object keyed by the reader agent id rather than
// a field on the agent record: the agent record is rewritten under CAS by
// most lifecycle transitions, so a cursor there would make every advance
// race unrelated agent writes and would need the agent to exist. A
// dedicated object commits in the same PutBatchConditional as the reader's
// effects, which is the atomicity D3 asks for.
//
// Wake rows: the recipient actor of an inbox is the desk agent whose id
// equals the inbox id ("texture:<doc>", "research:<doc>",
// "engineering:<doc>", "management:<owner>"), the identity the existing
// desks already use. System records (round closures) wake no one (D5).
//
// planInboxRecords takes no lock and composes into a caller's batch (a
// Texture turn, an RLM cell); CommitInboxRecords wraps it as a standalone
// idempotent command under trajectoryMu.

const (
	ogKindInboxHead        = objectgraph.ObjectKind("choir.inbox_head")
	ogKindConversationHead = objectgraph.ObjectKind("choir.conversation_head")
	ogKindInboxCursor      = objectgraph.ObjectKind("choir.inbox_cursor")
	ogKindInboxCommand     = objectgraph.ObjectKind("choir.inbox_command")

	// InboxRecordWakeKind is the actor wake kind minted per addressed record.
	InboxRecordWakeKind = "inbox_record"
	// inboxRecordPlane marks record objects committed through this path.
	inboxRecordPlane = "inbox"
)

// ErrInboxRefused marks a malformed or out-of-turn inbox record or cursor
// advance. The store validates well-formedness only and scores nothing.
var ErrInboxRefused = errors.New("inbox record refused")

func inboxRefused(format string, args ...any) error {
	return fmt.Errorf("inbox: "+format+": %w", append(args, ErrInboxRefused)...)
}

// inboxWakeContent is the wake row's immutable occurrence content.
type inboxWakeContent struct {
	Inbox          string `json:"inbox"`
	InboxSeq       int64  `json:"inbox_seq"`
	RecordID       string `json:"record_id"`
	ConversationID string `json:"conversation_id"`
	Round          int    `json:"round"`
}

func inboxWakeKey(inbox string, seq int64) string {
	return "inbox-wake:" + inbox + "\x00" + strconv.FormatInt(seq, 10)
}

// ComputeCommitInboxRecordsDigest returns the replay identity of an inbox
// command. Provenance commit time and model are observation metadata, not
// identity: a cell replayed after a model failover replays its receipt.
func ComputeCommitInboxRecordsDigest(req types.CommitInboxRecordsRequest) (string, error) {
	req.OwnerID, req.ComputerID, req.CommandDigest = "", "", ""
	req.CommandID = strings.TrimSpace(req.CommandID)
	records := make([]types.CommitmentRecord, len(req.Records))
	for i, rec := range req.Records {
		rec.Provenance.CommittedAt = ""
		rec.Provenance.ModelID = ""
		records[i] = rec
	}
	req.Records = records
	return lifecycleDigest(req)
}

// inboxCached is one cache object as the planner read it, plus its value.
type inboxCached[T any] struct {
	prior  objectgraph.Object
	exists bool
	value  T
	key    string
}

func loadInboxCached[T any](ctx context.Context, s *Store, kind objectgraph.ObjectKind, ownerID, computerID, key string) (*inboxCached[T], error) {
	obj, err := s.lifecycleGetObject(ctx, kind, ownerID, computerID, key)
	if errors.Is(err, ErrNotFound) {
		return &inboxCached[T]{key: key}, nil
	}
	if err != nil {
		return nil, err
	}
	value, err := decodeLifecycleObject[T](obj)
	if err != nil {
		return nil, err
	}
	return &inboxCached[T]{prior: obj, exists: true, value: value, key: key}, nil
}

// write returns the updated object and its CAS condition: the prior content
// hash when the object existed, absence otherwise.
func (c *inboxCached[T]) write(kind objectgraph.ObjectKind, ownerID, computerID string, metadata map[string]any, now time.Time) (objectgraph.Object, objectgraph.ObjectCondition, error) {
	createdAt := now
	if c.exists {
		createdAt = c.prior.CreatedAt
	}
	metadata["computer_id"] = computerID
	obj, err := lifecycleObject(kind, ownerID, computerID, c.key, c.value, metadata, createdAt, now)
	if err != nil {
		return objectgraph.Object{}, objectgraph.ObjectCondition{}, err
	}
	condition := objectgraph.ObjectCondition{CanonicalID: obj.CanonicalID}
	if c.exists {
		condition.Exists, condition.ExpectedContentHash = true, c.prior.ContentHash
	}
	return obj, condition, nil
}

func validInboxID(inbox string) bool {
	desk, subject, ok := strings.Cut(inbox, ":")
	return ok && strings.TrimSpace(desk) != "" && strings.TrimSpace(subject) != ""
}

func validateInboxRecordShape(rec types.CommitmentRecord) error {
	if rec.RecordID == "" || strings.TrimSpace(rec.Provenance.AgentID) == "" {
		return inboxRefused("record %q requires record_id and provenance agent", rec.RecordID)
	}
	if !validInboxID(rec.Inbox) || !validInboxID(rec.SenderInbox) || rec.Inbox == rec.SenderInbox {
		return inboxRefused("record %s needs distinct desk:subject inbox and sender_inbox (%q, %q)", rec.RecordID, rec.Inbox, rec.SenderInbox)
	}
	if rec.InboxSeq != 0 {
		return inboxRefused("record %s names inbox_seq %d; the store assigns it", rec.RecordID, rec.InboxSeq)
	}
	if rec.ConversationID != "" && rec.ConversationID != types.InboxConversationID(rec.Inbox, rec.SenderInbox) {
		return inboxRefused("record %s names conversation %q, its inboxes derive %q", rec.RecordID, rec.ConversationID, types.InboxConversationID(rec.Inbox, rec.SenderInbox))
	}
	switch rec.EmitterKind {
	case "", types.CommitmentEmitterAgent, types.CommitmentEmitterOwner, types.CommitmentEmitterEdition,
		types.CommitmentEmitterSource, types.CommitmentEmitterSystem:
	default:
		return inboxRefused("record %s emitter kind %q is unknown", rec.RecordID, rec.EmitterKind)
	}
	kind := rec.RecordKind()
	switch kind {
	case types.CommitmentKindPrecommit, types.CommitmentKindReport, types.CommitmentKindResolve, types.CommitmentKindDisagreement:
	case types.CommitmentKindDirective:
		if rec.Directive == nil || !types.IsValidDirectiveSubtype(rec.Directive.Subtype) {
			return inboxRefused("directive record %s requires a valid subtype", rec.RecordID)
		}
		if rec.Directive.Subtype == types.CommitmentDirectiveNote && strings.TrimSpace(rec.Directive.Body) == "" {
			return inboxRefused("note record %s requires a body", rec.RecordID)
		}
	default:
		return inboxRefused("record %s kind %q is not a mintable act", rec.RecordID, rec.Kind)
	}
	if rec.Cast != nil && !rec.IsCast() {
		return inboxRefused("record %s carries a cast body but is not a cast", rec.RecordID)
	}
	if rec.Report != nil && kind != types.CommitmentKindReport {
		return inboxRefused("record %s carries a report body but is not a report", rec.RecordID)
	}
	if rec.Final && (kind != types.CommitmentKindReport || rec.Report == nil || strings.TrimSpace(rec.Report.StopReason) == "") {
		return inboxRefused("final record %s must be a report with a stop_reason", rec.RecordID)
	}
	if rec.Job != nil && (rec.EmitterKind != types.CommitmentEmitterSystem || strings.TrimSpace(rec.Job.JobID) == "" || rec.SenderInbox != types.JobSenderInbox(rec.Job.JobID)) {
		return inboxRefused("job record %s must be a system record from job:<id> naming its job", rec.RecordID)
	}
	if rec.Captures != nil && (rec.EmitterKind != types.CommitmentEmitterSystem || strings.TrimSpace(rec.Captures.SourceID) == "" || len(rec.Captures.Items) == 0) {
		return inboxRefused("capture arrival %s must be a system record naming its source and at least one capture", rec.RecordID)
	}
	if rec.Inbox == types.StoryOpenInbox {
		return inboxRefused("record %s is addressed to %s; only a management Cast committed through CommitInboxRecords opens a story", rec.RecordID, types.StoryOpenInbox)
	}
	if d := rec.Disposition; d != nil {
		isNote := kind == types.CommitmentKindDirective && rec.Directive != nil && rec.Directive.Subtype == types.CommitmentDirectiveNote
		toStory, toSource := strings.HasPrefix(rec.Inbox, "texture:"), strings.HasPrefix(rec.Inbox, "source:")
		if !isNote || len(d.Captures) == 0 || !strings.HasPrefix(rec.SenderInbox, "management:") ||
			(d.Kind == types.DispositionFeed && !toStory) || (d.Kind == types.DispositionDecline && !toSource) ||
			(d.Kind != types.DispositionFeed && d.Kind != types.DispositionDecline) {
			return inboxRefused("disposition %s must be management's Note feeding captures to texture:<doc> or declining them to source:<id>", rec.RecordID)
		}
	}
	return nil
}

func validProbability(p float64) bool { return p >= 0 && p <= 1 }

// openHypothesisState is a newly opened hypothesis as the head holds it.
// Admission (stepConversation) and the fold (foldConversation) both build
// it here, so a rebuilt head matches the live one.
func openHypothesisState(h types.CastHypothesis, openedBy string, round int) types.HypothesisState {
	return types.HypothesisState{ID: h.ID, OpenedBy: openedBy, OpenedRound: round, Statement: h.Statement,
		Probability: h.Probability, General: h.General, Kind: h.Kind, Criterion: h.Criterion, ResolvesBy: h.ResolvesBy,
		Status: types.HypothesisOpen}
}

// checkHypothesisKind is L3-1's shape check at admission. A hypothesis
// without a kind is legacy and passes unchanged. An assertion needs a
// criterion and takes no resolves_by; a forecast needs both. The refusal
// names the hypothesis and the field, so the desk can correct and retry.
func checkHypothesisKind(recordID string, i int, h types.CastHypothesis) error {
	criterion := strings.TrimSpace(h.Criterion) != ""
	resolvesBy := strings.TrimSpace(h.ResolvesBy) != ""
	switch h.Kind {
	case "":
		if criterion || resolvesBy {
			return inboxRefused("cast %s hypothesis %d has a criterion or resolves_by but no kind; kind is %q or %q", recordID, i, types.HypothesisKindAssertion, types.HypothesisKindForecast)
		}
	case types.HypothesisKindAssertion:
		if !criterion {
			return inboxRefused("cast %s hypothesis %d is an assertion with no criterion: name the check that corroborates it", recordID, i)
		}
		if resolvesBy {
			return inboxRefused("cast %s hypothesis %d is an assertion with a resolves_by; only a forecast takes one", recordID, i)
		}
	case types.HypothesisKindForecast:
		if !criterion {
			return inboxRefused("cast %s hypothesis %d is a forecast with no criterion: name the event that settles it", recordID, i)
		}
		if !resolvesBy {
			return inboxRefused("cast %s hypothesis %d is a forecast with no resolves_by (RFC 3339, within 7 days)", recordID, i)
		}
	default:
		return inboxRefused("cast %s hypothesis %d has kind %q; kind is %q or %q", recordID, i, h.Kind, types.HypothesisKindAssertion, types.HypothesisKindForecast)
	}
	return nil
}

// checkHypothesisClock is L3-1's time check, run only when a Cast is first
// admitted, against the store's commit clock. A forecast's resolves_by is
// RFC 3339, after the commit and at most HypothesisForecastHorizon past
// it. The fold, replay and VerifyInboxHeads never run it: a committed
// forecast's resolves_by passes, and its record must still fold unchanged.
func checkHypothesisClock(recordID string, i int, h types.CastHypothesis, now time.Time) error {
	if h.Kind != types.HypothesisKindForecast || strings.TrimSpace(h.ResolvesBy) == "" {
		return nil
	}
	at, err := time.Parse(time.RFC3339, strings.TrimSpace(h.ResolvesBy))
	if err != nil {
		return inboxRefused("cast %s hypothesis %d has resolves_by %q; it must be RFC 3339, such as %s", recordID, i, h.ResolvesBy, now.UTC().Add(72*time.Hour).Format(time.RFC3339))
	}
	if !at.After(now) {
		return inboxRefused("cast %s hypothesis %d has resolves_by %s, not after the commit at %s", recordID, i, at.UTC().Format(time.RFC3339), now.UTC().Format(time.RFC3339))
	}
	if at.Sub(now) > types.HypothesisForecastHorizon {
		return inboxRefused("cast %s hypothesis %d has resolves_by %s, more than 7 days after the commit at %s", recordID, i, at.UTC().Format(time.RFC3339), now.UTC().Format(time.RFC3339))
	}
	return nil
}

// inboxRecordWakes reports whether a record mints a wake row for its
// recipient: RecordWakesReader, the one predicate the runtime's post-turn
// re-check also asks (management delegation R2-3). A desk's record wakes.
// A system record does not (D5), except the live-minted kinds that
// RecordWakesReader lists; that comment is the one list.
func inboxRecordWakes(rec types.CommitmentRecord) bool {
	return RecordWakesReader(rec)
}

// stepConversation validates rec against its conversation head, stamps its
// round and new hypothesis ids, and advances the head. It is the only
// writer of round and hypothesis state; any error refuses the whole plan.
func stepConversation(head *types.ConversationHead, rec *types.CommitmentRecord) (opened, closed bool, ids []string, err error) {
	sender := rec.SenderInbox
	round := 0
	switch {
	case rec.Final:
		if !head.Open {
			return false, false, nil, inboxRefused("final report %s: conversation %s has no open round", rec.RecordID, head.ConversationID)
		}
		if sender != head.RecipientInbox {
			return false, false, nil, inboxRefused("final report %s: only round %d's recipient %s closes it", rec.RecordID, head.Round, head.RecipientInbox)
		}
		round, closed = head.Round, true
	case rec.IsCast():
		if head.Open {
			if sender != head.CasterInbox {
				return false, false, nil, inboxRefused("cast %s: round %d is open and cast by %s", rec.RecordID, head.Round, head.CasterInbox)
			}
			round = head.Round
		} else {
			round, opened = head.Round+1, true
		}
	case head.Open:
		round = head.Round
	case strings.HasPrefix(sender, "engineering:"):
		// C1e D3: an engineering conversation is one assignment's one round.
		// Other desks send round-0 notes; engineering never speaks outside
		// its open round, so a late progress Report cannot land after the
		// round closed.
		return false, false, nil, inboxRefused("record %s: engineering sender %s has no open round in %s", rec.RecordID, sender, head.ConversationID)
	}
	if rec.Round != 0 && rec.Round != round {
		return false, false, nil, inboxRefused("record %s names round %d; the conversation is at round %d", rec.RecordID, rec.Round, round)
	}
	if opened {
		head.Round, head.Open, head.CasterInbox, head.RecipientInbox = round, true, sender, rec.Inbox
	}
	hypothesis := func(id string) (types.HypothesisState, error) {
		state, ok := head.Hypotheses[id]
		if !ok {
			return state, inboxRefused("record %s names unknown hypothesis %q in %s", rec.RecordID, id, head.ConversationID)
		}
		return state, nil
	}
	settleable := func(id string) (types.HypothesisState, error) {
		state, err := hypothesis(id)
		if err == nil && (state.Status != types.HypothesisOpen || state.OpenedBy == sender) {
			err = inboxRefused("record %s may not settle or drop %s (%s, opened by %s)", rec.RecordID, id, state.Status, state.OpenedBy)
		}
		return state, err
	}
	ownOpen := func(id string) (types.HypothesisState, error) {
		state, err := hypothesis(id)
		if err == nil && (state.Status != types.HypothesisOpen || state.OpenedBy != sender) {
			err = inboxRefused("record %s may not revise or withdraw %s (%s, opened by %s)", rec.RecordID, id, state.Status, state.OpenedBy)
		}
		return state, err
	}
	if report := rec.Report; report != nil {
		if round == 0 && len(report.Settled)+len(report.Dropped) > 0 {
			return false, false, nil, inboxRefused("report %s settles hypotheses outside an open round", rec.RecordID)
		}
		for _, claim := range report.Claims {
			if strings.TrimSpace(claim.Text) == "" {
				return false, false, nil, inboxRefused("report %s has an empty claim", rec.RecordID)
			}
		}
		for _, finding := range report.Findings {
			if strings.TrimSpace(finding.Text) == "" || (len(finding.BearsOn) > 0) == finding.Outside {
				return false, false, nil, inboxRefused("report %s finding must have text and bear on ids or lie outside", rec.RecordID)
			}
			for _, id := range finding.BearsOn {
				if _, err := hypothesis(id); err != nil {
					return false, false, nil, err
				}
			}
		}
		for _, settled := range report.Settled {
			state, err := settleable(settled.ID)
			if err == nil && strings.TrimSpace(settled.Outcome) == "" {
				err = inboxRefused("report %s settles %s without an outcome", rec.RecordID, settled.ID)
			}
			if err != nil {
				return false, false, nil, err
			}
			state.Status, state.Outcome = types.HypothesisSettled, settled.Outcome
			head.Hypotheses[settled.ID] = state
		}
		for _, dropped := range report.Dropped {
			state, err := settleable(dropped.ID)
			if err == nil && strings.TrimSpace(dropped.Reason) == "" {
				err = inboxRefused("report %s drops %s without a reason", rec.RecordID, dropped.ID)
			}
			if err != nil {
				return false, false, nil, err
			}
			state.Status, state.Reason = types.HypothesisDropped, dropped.Reason
			head.Hypotheses[dropped.ID] = state
		}
	}
	if rec.Cast != nil {
		cast := *rec.Cast
		for _, revision := range cast.Revisions {
			state, err := ownOpen(revision.ID)
			if err == nil && revision.Probability != nil && !validProbability(*revision.Probability) {
				err = inboxRefused("cast %s revises %s to probability %v", rec.RecordID, revision.ID, *revision.Probability)
			}
			if err != nil {
				return false, false, nil, err
			}
			if revision.Statement != "" {
				state.Statement = revision.Statement
			}
			if revision.Probability != nil {
				state.Probability = *revision.Probability
			}
			head.Hypotheses[revision.ID] = state
		}
		for _, withdrawal := range cast.Withdrawals {
			state, err := ownOpen(withdrawal.ID)
			if err != nil {
				return false, false, nil, err
			}
			state.Status, state.Reason = types.HypothesisWithdrawn, withdrawal.Reason
			head.Hypotheses[withdrawal.ID] = state
		}
		for _, uncertainty := range cast.Uncertainties {
			if strings.TrimSpace(uncertainty.Question) == "" {
				return false, false, nil, inboxRefused("cast %s has an empty uncertainty", rec.RecordID)
			}
		}
		cast.Hypotheses = append([]types.CastHypothesis(nil), cast.Hypotheses...)
		for i, h := range cast.Hypotheses {
			if h.ID != "" || strings.TrimSpace(h.Statement) == "" || !validProbability(h.Probability) {
				return false, false, nil, inboxRefused("cast %s hypothesis %d needs a statement, a probability in [0,1] and no id", rec.RecordID, i)
			}
			if err := checkHypothesisKind(rec.RecordID, i, h); err != nil {
				return false, false, nil, err
			}
			head.HypothesisCount++
			h.ID = "H" + strconv.Itoa(head.HypothesisCount)
			cast.Hypotheses[i] = h
			if head.Hypotheses == nil {
				head.Hypotheses = map[string]types.HypothesisState{}
			}
			head.Hypotheses[h.ID] = openHypothesisState(h, sender, round)
			ids = append(ids, h.ID)
		}
		rec.Cast = &cast
	}
	if closed {
		head.Open = false
	}
	rec.Round = round
	return opened, closed, ids, nil
}

// planInboxRecords reads the current heads and cursor and returns the
// objects (records, inbox heads, conversation heads, wake rows, cursor) and
// CAS conditions that commit records and the cursor advance atomically,
// plus the receipt the commit would produce. It takes no lock, so a caller
// already holding trajectoryMu can append the result to its own batch; one
// plan per batch. A head CAS failure refuses the whole batch.
func (s *Store) planInboxRecords(ctx context.Context, ownerID, computerID string, records []types.CommitmentRecord, cursor *types.InboxCursorAdvance, now time.Time) ([]objectgraph.Object, []objectgraph.ObjectCondition, types.InboxCommitReceipt, error) {
	var objects []objectgraph.Object
	var conditions []objectgraph.ObjectCondition
	receipt := types.InboxCommitReceipt{CreatedAt: now}
	if len(records) == 0 && cursor == nil {
		return nil, nil, receipt, inboxRefused("nothing to commit")
	}
	heads := map[string]*inboxCached[types.InboxHead]{}
	loadHead := func(inbox string) (*inboxCached[types.InboxHead], error) {
		if head, ok := heads[inbox]; ok {
			return head, nil
		}
		head, err := loadInboxCached[types.InboxHead](ctx, s, ogKindInboxHead, ownerID, computerID, inbox)
		if err == nil {
			head.value.Inbox = inbox
			heads[inbox] = head
		}
		return head, err
	}
	conversations := map[string]*inboxCached[types.ConversationHead]{}
	var headOrder, conversationOrder []string
	dirtyHeads, dirtyConversations := map[string]bool{}, map[string]bool{}
	seenRecords := map[string]bool{}

	for _, rec := range records {
		rec.RecordID, rec.Inbox, rec.SenderInbox = strings.TrimSpace(rec.RecordID), strings.TrimSpace(rec.Inbox), strings.TrimSpace(rec.SenderInbox)
		if strings.TrimSpace(rec.Provenance.CommittedAt) == "" {
			// A record folded into a replayable body leaves its time to the
			// commit.
			rec.Provenance.CommittedAt = now.UTC().Format(time.RFC3339Nano)
		}
		if err := validateInboxRecordShape(rec); err != nil {
			return nil, nil, receipt, err
		}
		if seenRecords[rec.RecordID] {
			return nil, nil, receipt, inboxRefused("record %s appears twice", rec.RecordID)
		}
		seenRecords[rec.RecordID] = true
		if exists, err := s.CommitmentRecordExists(ctx, ownerID, computerID, rec.RecordID); err != nil {
			return nil, nil, receipt, err
		} else if exists {
			return nil, nil, receipt, inboxRefused("record %s is already committed", rec.RecordID)
		}

		head, err := loadHead(rec.Inbox)
		if err != nil {
			return nil, nil, receipt, err
		}
		head.value.Seq++
		rec.InboxSeq = head.value.Seq
		if !dirtyHeads[rec.Inbox] {
			dirtyHeads[rec.Inbox] = true
			headOrder = append(headOrder, rec.Inbox)
		}

		rec.ConversationID = types.InboxConversationID(rec.Inbox, rec.SenderInbox)
		conversation, ok := conversations[rec.ConversationID]
		if !ok {
			conversation, err = loadInboxCached[types.ConversationHead](ctx, s, ogKindConversationHead, ownerID, computerID, rec.ConversationID)
			if err != nil {
				return nil, nil, receipt, err
			}
			conversation.value.ConversationID = rec.ConversationID
			conversations[rec.ConversationID] = conversation
		}
		if rec.Cast != nil {
			for i, h := range rec.Cast.Hypotheses {
				if err := checkHypothesisClock(rec.RecordID, i, h, now); err != nil {
					return nil, nil, receipt, err
				}
			}
		}
		before, _ := json.Marshal(conversation.value)
		opened, closed, ids, err := stepConversation(&conversation.value, &rec)
		if err != nil {
			return nil, nil, receipt, err
		}
		if after, _ := json.Marshal(conversation.value); (!conversation.exists || string(before) != string(after)) && !dirtyConversations[rec.ConversationID] {
			dirtyConversations[rec.ConversationID] = true
			conversationOrder = append(conversationOrder, rec.ConversationID)
		}

		stamp := now.UTC().Format(time.RFC3339Nano)
		recordObj, err := lifecycleObject(ogKindCommitmentRecord, ownerID, computerID, rec.RecordID, rec, map[string]any{
			"record_id": rec.RecordID, "agent_id": rec.Provenance.AgentID, "model_id": rec.Provenance.ModelID,
			"discrepancy": string(rec.Discrepancy), "schema_id": rec.SchemaID, "record_kind": string(rec.RecordKind()),
			"addressee": strings.TrimSpace(rec.Addressee), "record_plane": inboxRecordPlane,
			"inbox": rec.Inbox, "inbox_seq": rec.InboxSeq, "sender_inbox": rec.SenderInbox,
			"conversation_id": rec.ConversationID, "round": rec.Round, "final": rec.Final,
			"emitter_kind": string(rec.EmitterKind), "created_at": stamp, "updated_at": stamp,
		}, now, now)
		if err != nil {
			return nil, nil, receipt, err
		}
		objects = append(objects, recordObj)
		conditions = append(conditions, objectgraph.ObjectCondition{CanonicalID: recordObj.CanonicalID})

		row := types.InboxRecordReceipt{RecordID: rec.RecordID, Inbox: rec.Inbox, InboxSeq: rec.InboxSeq,
			ConversationID: rec.ConversationID, Round: rec.Round, OpenedRound: opened, ClosedRound: closed, HypothesisIDs: ids}
		if inboxRecordWakes(rec) {
			content, err := json.Marshal(inboxWakeContent{Inbox: rec.Inbox, InboxSeq: rec.InboxSeq, RecordID: rec.RecordID,
				ConversationID: rec.ConversationID, Round: rec.Round})
			if err != nil {
				return nil, nil, receipt, err
			}
			_, wakeObj, err := actorWakeOutbox(recordObj, "inbox:"+rec.Inbox+":"+strconv.FormatInt(rec.InboxSeq, 10), rec.Inbox, "",
				rec.SenderInbox, InboxRecordWakeKind, string(content), time.Time{}, inboxWakeKey(rec.Inbox, rec.InboxSeq))
			if err != nil {
				return nil, nil, receipt, err
			}
			objects = append(objects, wakeObj)
			conditions = append(conditions, objectgraph.ObjectCondition{CanonicalID: wakeObj.CanonicalID})
			row.WakeID = wakeObj.CanonicalID
		}
		receipt.Records = append(receipt.Records, row)
	}

	if cursor != nil {
		advance := *cursor
		advance.ReaderAgentID, advance.Inbox = strings.TrimSpace(advance.ReaderAgentID), strings.TrimSpace(advance.Inbox)
		if advance.ReaderAgentID == "" || !validInboxID(advance.Inbox) {
			return nil, nil, receipt, inboxRefused("cursor advance needs a reader and an inbox")
		}
		stored, err := loadInboxCached[types.InboxCursor](ctx, s, ogKindInboxCursor, ownerID, computerID, advance.ReaderAgentID)
		if err != nil {
			return nil, nil, receipt, err
		}
		current := stored.value.Cursors[advance.Inbox]
		if current != advance.ExpectedSeq {
			return nil, nil, receipt, fmt.Errorf("inbox: cursor %s on %s is %d, not %d: %w", advance.ReaderAgentID, advance.Inbox, current, advance.ExpectedSeq, ErrConcurrentStateChange)
		}
		head, err := loadHead(advance.Inbox)
		if err != nil {
			return nil, nil, receipt, err
		}
		if advance.Seq < advance.ExpectedSeq || advance.Seq > head.value.Seq {
			return nil, nil, receipt, inboxRefused("cursor %s on %s may move from %d only forward through head %d, not to %d",
				advance.ReaderAgentID, advance.Inbox, advance.ExpectedSeq, head.value.Seq, advance.Seq)
		}
		stored.value.ReaderAgentID = advance.ReaderAgentID
		if stored.value.Cursors == nil {
			stored.value.Cursors = map[string]int64{}
		}
		stored.value.Cursors[advance.Inbox] = advance.Seq
		obj, condition, err := stored.write(ogKindInboxCursor, ownerID, computerID, map[string]any{"reader_agent_id": advance.ReaderAgentID}, now)
		if err != nil {
			return nil, nil, receipt, err
		}
		objects, conditions = append(objects, obj), append(conditions, condition)
		receipt.Cursor = &advance
	}

	for _, inbox := range headOrder {
		obj, condition, err := heads[inbox].write(ogKindInboxHead, ownerID, computerID, map[string]any{"inbox": inbox, "seq": heads[inbox].value.Seq}, now)
		if err != nil {
			return nil, nil, receipt, err
		}
		objects, conditions = append(objects, obj), append(conditions, condition)
	}
	for _, id := range conversationOrder {
		obj, condition, err := conversations[id].write(ogKindConversationHead, ownerID, computerID, map[string]any{"conversation_id": id}, now)
		if err != nil {
			return nil, nil, receipt, err
		}
		objects, conditions = append(objects, obj), append(conditions, condition)
	}
	return objects, conditions, receipt, nil
}

// normalizeInboxCommand trims an inbox command's scope and identity and
// checks its digest.
func normalizeInboxCommand(req types.CommitInboxRecordsRequest) (types.CommitInboxRecordsRequest, error) {
	ownerID, computerID, err := normalizeLifecycleScope(req.OwnerID, req.ComputerID)
	if err != nil {
		return req, err
	}
	req.OwnerID, req.ComputerID = ownerID, computerID
	req.CommandID, req.CommandDigest = strings.TrimSpace(req.CommandID), strings.TrimSpace(req.CommandDigest)
	if req.CommandID == "" {
		return req, fmt.Errorf("commit inbox records: command_id is required")
	}
	computed, digestErr := ComputeCommitInboxRecordsDigest(req)
	if err := requireLifecycleDigest(req.CommandDigest, computed, digestErr); err != nil {
		return req, err
	}
	return req, nil
}

// CommitInboxRecords commits addressed records and an optional cursor
// advance as one idempotent command: the same command id and digest
// replays the stored receipt, a different digest is a conflict.
func (s *Store) CommitInboxRecords(ctx context.Context, req types.CommitInboxRecordsRequest) (types.InboxCommitReceipt, error) {
	req, err := normalizeInboxCommand(req)
	if err != nil {
		return types.InboxCommitReceipt{}, err
	}
	ownerID, computerID := req.OwnerID, req.ComputerID
	if s.ogStore == nil {
		return types.InboxCommitReceipt{}, fmt.Errorf("commit inbox records: object graph not initialized")
	}

	s.trajectoryMu.Lock()
	defer s.trajectoryMu.Unlock()
	if replay, found, err := s.replayInboxCommand(ctx, ownerID, computerID, req.CommandID, req.CommandDigest); found || err != nil {
		return replay, err
	}
	objects, conditions, storyEdges, receipt, err := s.planInboxCommand(ctx, req, time.Now().UTC())
	if err != nil {
		return types.InboxCommitReceipt{}, err
	}
	if err := s.ogStore.PutBatchConditional(ctx, conditions, objectgraph.Batch{Objects: objects, Edges: storyEdges}); err != nil {
		if errors.Is(err, objectgraph.ErrConflict) {
			if replay, found, replayErr := s.replayInboxCommand(ctx, ownerID, computerID, req.CommandID, req.CommandDigest); found || replayErr != nil {
				return replay, replayErr
			}
			return types.InboxCommitReceipt{}, ErrConcurrentStateChange
		}
		return types.InboxCommitReceipt{}, err
	}
	s.signalActorWakesIfAny(objects)
	return receipt, nil
}

// planInboxCommand plans one inbox command's whole batch: support verdicts,
// story openings, the records and cursor advance, and the inbox_command
// object that makes the command replayable. It takes no lock; the caller
// holds trajectoryMu and has already checked replay. CommitInboxRecords
// commits the plan alone; OpenEngineeringAssignment composes it into the
// assignment's transaction (C1e D4), so a cell's records, cursor, the
// assignment row and its binding commit together.
func (s *Store) planInboxCommand(ctx context.Context, req types.CommitInboxRecordsRequest, now time.Time) ([]objectgraph.Object, []objectgraph.ObjectCondition, []objectgraph.Edge, types.InboxCommitReceipt, error) {
	ownerID, computerID := req.OwnerID, req.ComputerID
	// Support verdicts (the runtime's Jev job, as system:support) commit
	// their memos here, and their failure records ride the same plan.
	verdictObjects, verdictConditions, failures, recorded, err := s.planSupportVerdicts(ctx, ownerID, computerID, req.Judge, req.CommandID, req.Verdicts, now)
	if err != nil {
		return nil, nil, nil, types.InboxCommitReceipt{}, err
	}
	// A management Cast to texture:new opens its story in this commit (Cut 2
	// 2b): the story's objects join the batch and the Cast is delivered to
	// the new texture:<doc>.
	records, storyObjects, storyConditions, storyEdges, opened, err := s.planStoryOpenings(ctx, ownerID, computerID, req.CommandID, req.Records, now)
	if err != nil {
		return nil, nil, nil, types.InboxCommitReceipt{}, err
	}
	// The story-library head (management delegation R2-5) refuses an open
	// from a run whose last read of the library is behind it.
	libraryObjects, libraryConditions, libraryHead, err := s.planStoryLibrary(ctx, ownerID, computerID, req.StoryLibraryHead, records, opened, now)
	if err != nil {
		return nil, nil, nil, types.InboxCommitReceipt{}, err
	}
	storyObjects, storyConditions = append(storyObjects, libraryObjects...), append(storyConditions, libraryConditions...)
	// Management tasks (S2a): task rows with their opening Casts, the
	// task-scope precondition, the capture disposition index, and each task
	// closure's outcome, all under this command's lock.
	records, taskObjects, taskConditions, err := s.planManagementTasks(ctx, ownerID, computerID, req.Tasks, records, opened, now)
	if err != nil {
		return nil, nil, nil, types.InboxCommitReceipt{}, err
	}
	storyObjects, storyConditions = append(storyObjects, taskObjects...), append(storyConditions, taskConditions...)
	if len(failures) > 0 {
		records = append(append([]types.CommitmentRecord(nil), records...), failures...)
	}
	var objects []objectgraph.Object
	var conditions []objectgraph.ObjectCondition
	receipt := types.InboxCommitReceipt{CreatedAt: now}
	// A command of verdicts that were all memo hits commits only its receipt.
	if len(records) > 0 || req.Cursor != nil || len(req.Verdicts) == 0 {
		objects, conditions, receipt, err = s.planInboxRecords(ctx, ownerID, computerID, records, req.Cursor, now)
		if err != nil {
			return nil, nil, nil, types.InboxCommitReceipt{}, err
		}
	}
	for i := range receipt.Records {
		receipt.Records[i].OpenedStory = opened[receipt.Records[i].RecordID]
	}
	objects, conditions = append(objects, storyObjects...), append(conditions, storyConditions...)
	objects, conditions = append(objects, verdictObjects...), append(conditions, verdictConditions...)
	receipt.Verdicts = recorded
	receipt.StoryLibraryHead = libraryHead
	receipt.CommandID, receipt.CommandDigest = req.CommandID, req.CommandDigest
	commandObj, err := lifecycleObject(ogKindInboxCommand, ownerID, computerID, req.CommandID, receipt,
		map[string]any{"command_id": req.CommandID, "computer_id": computerID}, now, now)
	if err != nil {
		return nil, nil, nil, types.InboxCommitReceipt{}, err
	}
	objects = append(objects, commandObj)
	conditions = append(conditions, objectgraph.ObjectCondition{CanonicalID: commandObj.CanonicalID})
	return objects, conditions, storyEdges, receipt, nil
}

func (s *Store) replayInboxCommand(ctx context.Context, ownerID, computerID, commandID, digest string) (types.InboxCommitReceipt, bool, error) {
	obj, err := s.lifecycleGetObject(ctx, ogKindInboxCommand, ownerID, computerID, commandID)
	if errors.Is(err, ErrNotFound) {
		return types.InboxCommitReceipt{}, false, nil
	}
	if err != nil {
		return types.InboxCommitReceipt{}, false, err
	}
	receipt, err := decodeLifecycleObject[types.InboxCommitReceipt](obj)
	if err != nil {
		return types.InboxCommitReceipt{}, true, err
	}
	if receipt.CommandDigest != digest {
		return types.InboxCommitReceipt{}, true, ErrLifecycleCommandConflict
	}
	receipt.Replay = true
	return receipt, true, nil
}

// InboxHead returns the last sequence number taken in an inbox (0 when
// none). It reads the head cache; RebuildInboxHeads checks it.
func (s *Store) InboxHead(ctx context.Context, ownerID, computerID, inbox string) (int64, error) {
	head, err := loadInboxCached[types.InboxHead](ctx, s, ogKindInboxHead, strings.TrimSpace(ownerID), strings.TrimSpace(computerID), strings.TrimSpace(inbox))
	if err != nil {
		return 0, err
	}
	return head.value.Seq, nil
}

// ReaderCursor returns the sequence a reader has read through in an inbox.
func (s *Store) ReaderCursor(ctx context.Context, ownerID, computerID, readerAgentID, inbox string) (int64, error) {
	cursor, err := loadInboxCached[types.InboxCursor](ctx, s, ogKindInboxCursor, strings.TrimSpace(ownerID), strings.TrimSpace(computerID), strings.TrimSpace(readerAgentID))
	if err != nil {
		return 0, err
	}
	return cursor.value.Cursors[strings.TrimSpace(inbox)], nil
}

// UnreadFor lists the inboxes a reader has not read through: every inbox in
// its cursor map plus the inbox named by its own agent id.
func (s *Store) UnreadFor(ctx context.Context, ownerID, computerID, readerAgentID string) ([]types.InboxUnread, error) {
	ownerID, computerID, readerAgentID = strings.TrimSpace(ownerID), strings.TrimSpace(computerID), strings.TrimSpace(readerAgentID)
	cursor, err := loadInboxCached[types.InboxCursor](ctx, s, ogKindInboxCursor, ownerID, computerID, readerAgentID)
	if err != nil {
		return nil, err
	}
	inboxes := []string{readerAgentID}
	for inbox := range cursor.value.Cursors {
		if inbox != readerAgentID {
			inboxes = append(inboxes, inbox)
		}
	}
	sort.Strings(inboxes)
	var out []types.InboxUnread
	for _, inbox := range inboxes {
		head, err := s.InboxHead(ctx, ownerID, computerID, inbox)
		if err != nil {
			return nil, err
		}
		if read := cursor.value.Cursors[inbox]; head > read {
			out = append(out, types.InboxUnread{Inbox: inbox, Cursor: read, Head: head})
		}
	}
	return out, nil
}

func (s *Store) listInboxRecordsBy(ctx context.Context, ownerID, computerID, field, value string) ([]types.CommitmentRecord, error) {
	objs, err := s.ogListAllByMetadata(ctx, ogKindCommitmentRecord, field, value)
	if err != nil {
		return nil, err
	}
	return inboxRecordsOf(objs, ownerID, computerID)
}

// inboxRecordsOf keeps the live inbox records of one owner and computer,
// decoded and sorted by (inbox, seq).
func inboxRecordsOf(objs []objectgraph.Object, ownerID, computerID string) ([]types.CommitmentRecord, error) {
	records := make([]types.CommitmentRecord, 0, len(objs))
	for _, obj := range objs {
		if obj.Tombstone || obj.OwnerID != ownerID || obj.ComputerID != computerID {
			continue
		}
		rec, err := decodeLifecycleObject[types.CommitmentRecord](obj)
		if err != nil {
			return nil, fmt.Errorf("inbox record decode %s: %w", obj.CanonicalID, err)
		}
		if rec.InboxSeq > 0 {
			records = append(records, rec)
		}
	}
	sort.Slice(records, func(i, j int) bool {
		if records[i].Inbox != records[j].Inbox {
			return records[i].Inbox < records[j].Inbox
		}
		return records[i].InboxSeq < records[j].InboxSeq
	})
	return records, nil
}

// ListInboxRecords returns an inbox's records in (afterSeq, throughSeq] by
// sequence; throughSeq <= 0 means no upper bound.
//
// It reads only the (afterSeq, throughSeq] range of the inbox's sequence
// index (a cell reads its unread records, not the whole inbox) when that
// index is live. The range is on the metadata inbox_seq, which
// planInboxRecords writes from the same value as the body's; the body
// filter below still decides, so the result is the unindexed read's.
func (s *Store) ListInboxRecords(ctx context.Context, ownerID, computerID, inbox string, afterSeq, throughSeq int64) ([]types.CommitmentRecord, error) {
	ownerID, computerID, inbox = strings.TrimSpace(ownerID), strings.TrimSpace(computerID), strings.TrimSpace(inbox)
	var records []types.CommitmentRecord
	graph := s.ogReadStore
	if graph == nil {
		graph = s.ogStore
	}
	objs, indexed, err := graph.ListRecordsByInboxSeq(ctx, inbox, afterSeq, throughSeq)
	switch {
	case err != nil:
		return nil, err
	case indexed:
		records, err = inboxRecordsOf(objs, ownerID, computerID)
	default:
		records, err = s.listInboxRecordsBy(ctx, ownerID, computerID, "inbox", inbox)
	}
	if err != nil {
		return nil, err
	}
	out := records[:0]
	for _, rec := range records {
		if rec.InboxSeq > afterSeq && (throughSeq <= 0 || rec.InboxSeq <= throughSeq) {
			out = append(out, rec)
		}
	}
	return out, nil
}

// foldConversation derives a conversation head from its records alone. It
// does not re-validate: commits were validated by stepConversation, so the
// fold is order-free across the two inboxes. Casts are folded before
// Reports because a Report can only settle a hypothesis already opened.
func foldConversation(conversationID string, records []types.CommitmentRecord) types.ConversationHead {
	head := types.ConversationHead{ConversationID: conversationID}
	sorted := append([]types.CommitmentRecord(nil), records...)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Inbox != sorted[j].Inbox {
			return sorted[i].Inbox < sorted[j].Inbox
		}
		return sorted[i].InboxSeq < sorted[j].InboxSeq
	})
	finals := map[int]bool{}
	type party struct{ caster, recipient string }
	parties := map[int]party{}
	for _, rec := range sorted {
		if rec.Round > head.Round {
			head.Round = rec.Round
		}
		if rec.Final {
			finals[rec.Round] = true
		}
		if !rec.IsCast() || rec.Round == 0 {
			continue
		}
		parties[rec.Round] = party{caster: rec.SenderInbox, recipient: rec.Inbox}
		if rec.Cast == nil {
			continue
		}
		for _, revision := range rec.Cast.Revisions {
			if state, ok := head.Hypotheses[revision.ID]; ok {
				if revision.Statement != "" {
					state.Statement = revision.Statement
				}
				if revision.Probability != nil {
					state.Probability = *revision.Probability
				}
				head.Hypotheses[revision.ID] = state
			}
		}
		for _, withdrawal := range rec.Cast.Withdrawals {
			if state, ok := head.Hypotheses[withdrawal.ID]; ok {
				state.Status, state.Reason = types.HypothesisWithdrawn, withdrawal.Reason
				head.Hypotheses[withdrawal.ID] = state
			}
		}
		for _, h := range rec.Cast.Hypotheses {
			if head.Hypotheses == nil {
				head.Hypotheses = map[string]types.HypothesisState{}
			}
			head.Hypotheses[h.ID] = openHypothesisState(h, rec.SenderInbox, rec.Round)
		}
	}
	for _, rec := range sorted {
		if rec.Report == nil {
			continue
		}
		for _, settled := range rec.Report.Settled {
			if state, ok := head.Hypotheses[settled.ID]; ok {
				state.Status, state.Outcome = types.HypothesisSettled, settled.Outcome
				head.Hypotheses[settled.ID] = state
			}
		}
		for _, dropped := range rec.Report.Dropped {
			if state, ok := head.Hypotheses[dropped.ID]; ok {
				state.Status, state.Reason = types.HypothesisDropped, dropped.Reason
				head.Hypotheses[dropped.ID] = state
			}
		}
	}
	head.HypothesisCount = len(head.Hypotheses)
	if head.Round > 0 {
		head.Open = !finals[head.Round]
		head.CasterInbox, head.RecipientInbox = parties[head.Round].caster, parties[head.Round].recipient
	}
	return head
}

// ConversationRecords returns one conversation's records, by (inbox, seq),
// from the conversation index.
func (s *Store) ConversationRecords(ctx context.Context, ownerID, computerID, conversationID string) ([]types.CommitmentRecord, error) {
	return s.listInboxRecordsBy(ctx, strings.TrimSpace(ownerID), strings.TrimSpace(computerID), "conversation_id", strings.TrimSpace(conversationID))
}

// ConversationState folds one conversation's records into its head.
func (s *Store) ConversationState(ctx context.Context, ownerID, computerID, conversationID string) (types.ConversationHead, error) {
	records, err := s.listInboxRecordsBy(ctx, strings.TrimSpace(ownerID), strings.TrimSpace(computerID), "conversation_id", strings.TrimSpace(conversationID))
	if err != nil {
		return types.ConversationHead{}, err
	}
	return foldConversation(strings.TrimSpace(conversationID), records), nil
}

// HypothesisStates folds one conversation's hypotheses, ordered H1, H2, ...
func (s *Store) HypothesisStates(ctx context.Context, ownerID, computerID, conversationID string) ([]types.HypothesisState, error) {
	head, err := s.ConversationState(ctx, ownerID, computerID, conversationID)
	if err != nil {
		return nil, err
	}
	states := make([]types.HypothesisState, 0, len(head.Hypotheses))
	for _, state := range head.Hypotheses {
		states = append(states, state)
	}
	sort.Slice(states, func(i, j int) bool {
		a, _ := strconv.Atoi(strings.TrimPrefix(states[i].ID, "H"))
		b, _ := strconv.Atoi(strings.TrimPrefix(states[j].ID, "H"))
		return a < b
	})
	return states, nil
}

// OpenRounds folds every conversation the inbox takes part in and returns
// those with an open round.
func (s *Store) OpenRounds(ctx context.Context, ownerID, computerID, inbox string) ([]types.ConversationHead, error) {
	ownerID, computerID, inbox = strings.TrimSpace(ownerID), strings.TrimSpace(computerID), strings.TrimSpace(inbox)
	ids := map[string]bool{}
	for _, field := range []string{"inbox", "sender_inbox"} {
		records, err := s.listInboxRecordsBy(ctx, ownerID, computerID, field, inbox)
		if err != nil {
			return nil, err
		}
		for _, rec := range records {
			ids[rec.ConversationID] = true
		}
	}
	var open []types.ConversationHead
	for id := range ids {
		head, err := s.ConversationState(ctx, ownerID, computerID, id)
		if err != nil {
			return nil, err
		}
		if head.Open {
			open = append(open, head)
		}
	}
	sort.Slice(open, func(i, j int) bool { return open[i].ConversationID < open[j].ConversationID })
	return open, nil
}

// OpenRoundsAddressedTo returns the open rounds whose recipient is inbox,
// sorted by conversation id: OpenRounds filtered to RecipientInbox == inbox,
// read from the conversation heads instead of folded from records.
//
// It is the run-end check's read (closeRoundsOfEndedRun). OpenRounds folds
// every conversation the inbox ever joined from all of its records, so its
// cost grew with the computer's history and ran at every desk run's end
// (44% of the paper's CPU; docs/problems/guest-cpu-is-unindexed-store-
// scans-2026-10-14.md). A conversation head is written by planInboxRecords
// in the same compare-and-set batch as every record that changes it, and it
// is what stepConversation validates a closure against, so the head is the
// authority a closure commits under. VerifyInboxHeads checks the heads
// against the fold. This read costs one header row per conversation of the
// computer and one primary-key read per conversation of the inbox.
func (s *Store) OpenRoundsAddressedTo(ctx context.Context, ownerID, computerID, inbox string) ([]types.ConversationHead, error) {
	inbox = strings.TrimSpace(inbox)
	// A conversation id is "conv:<a>|<b>" with a < b (InboxConversationID),
	// so a conversation of inbox starts "conv:<inbox>|" or ends "|<inbox>".
	return s.openConversationHeads(ctx, ownerID, computerID, func(conversationID string) bool {
		return strings.HasPrefix(conversationID, "conv:"+inbox+"|") || strings.HasSuffix(conversationID, "|"+inbox)
	}, func(head types.ConversationHead) bool { return head.RecipientInbox == inbox })
}

// OpenConversationHeads returns every open round of the computer, sorted by
// conversation id, read from the conversation heads. It is the crash boot's
// read (closeOpenRoundsAfterCrash): RebuildInboxHeads folds every inbox
// record of the computer, a cost that grows with its whole history (4m36s
// on the paper; docs/problems/paper-crash-boot-holds-work-six-minutes-
// 2026-10-22.md C1). The head is written in the same atomic batch as every
// record that changes it, so a crash cannot leave a record without its
// head (C2).
func (s *Store) OpenConversationHeads(ctx context.Context, ownerID, computerID string) ([]types.ConversationHead, error) {
	return s.openConversationHeads(ctx, ownerID, computerID, nil, nil)
}

// openConversationHeads reads the open conversation heads of one owner and
// computer: one header row per conversation of the owner, and one
// primary-key read per head that wanted(conversation id) keeps. Metadata
// that does not name its conversation is read whole rather than skipped.
func (s *Store) openConversationHeads(ctx context.Context, ownerID, computerID string, wanted func(string) bool, keep func(types.ConversationHead) bool) ([]types.ConversationHead, error) {
	ownerID, computerID = strings.TrimSpace(ownerID), strings.TrimSpace(computerID)
	graph := s.ogReadStore
	if graph == nil {
		graph = s.ogStore
	}
	if graph == nil {
		return nil, fmt.Errorf("open rounds: object graph not initialized")
	}
	refs, err := graph.ListAllObjectRefsByKindOwner(ctx, string(ogKindConversationHead), ownerID)
	if err != nil {
		return nil, err
	}
	var open []types.ConversationHead
	for _, ref := range refs {
		if ref.Tombstone || ref.ComputerID != computerID {
			continue
		}
		var meta struct {
			ConversationID string `json:"conversation_id"`
		}
		if wanted != nil && json.Unmarshal(ref.Metadata, &meta) == nil && meta.ConversationID != "" && !wanted(meta.ConversationID) {
			continue
		}
		obj, err := graph.GetObject(ctx, ref.CanonicalID)
		if errors.Is(err, objectgraph.ErrNotFound) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if obj.Tombstone {
			continue
		}
		head, err := decodeLifecycleObject[types.ConversationHead](obj)
		if err != nil {
			return nil, fmt.Errorf("conversation head decode %s: %w", obj.CanonicalID, err)
		}
		if head.Open && (keep == nil || keep(head)) {
			open = append(open, head)
		}
	}
	sort.Slice(open, func(i, j int) bool { return open[i].ConversationID < open[j].ConversationID })
	return open, nil
}

// InboxFold is the work state rebuilt from records alone: each inbox's
// head, each conversation's head, and any record that breaks the
// sequence (a gap or a duplicate) or its derived conversation id.
type InboxFold struct {
	Heads         map[string]int64
	Conversations map[string]types.ConversationHead
	Violations    []string
}

// RebuildInboxHeads recomputes inbox and conversation heads from the
// computer's inbox records.
func (s *Store) RebuildInboxHeads(ctx context.Context, ownerID, computerID string) (InboxFold, error) {
	ownerID, computerID = strings.TrimSpace(ownerID), strings.TrimSpace(computerID)
	records, err := s.listInboxRecordsBy(ctx, ownerID, computerID, "record_plane", inboxRecordPlane)
	if err != nil {
		return InboxFold{}, err
	}
	fold := InboxFold{Heads: map[string]int64{}, Conversations: map[string]types.ConversationHead{}}
	byConversation := map[string][]types.CommitmentRecord{}
	for _, rec := range records { // sorted by (inbox, seq)
		if want := fold.Heads[rec.Inbox] + 1; rec.InboxSeq != want {
			fold.Violations = append(fold.Violations, fmt.Sprintf("inbox %s: record %s has seq %d, want %d", rec.Inbox, rec.RecordID, rec.InboxSeq, want))
		}
		if rec.InboxSeq > fold.Heads[rec.Inbox] {
			fold.Heads[rec.Inbox] = rec.InboxSeq
		}
		if rec.ConversationID != types.InboxConversationID(rec.Inbox, rec.SenderInbox) {
			fold.Violations = append(fold.Violations, fmt.Sprintf("record %s: conversation %s does not derive from its inboxes", rec.RecordID, rec.ConversationID))
		}
		byConversation[rec.ConversationID] = append(byConversation[rec.ConversationID], rec)
	}
	for id, recs := range byConversation {
		fold.Conversations[id] = foldConversation(id, recs)
	}
	return fold, nil
}

// VerifyInboxHeads compares the cached heads and every cursor with the
// rebuild from records and returns each difference (empty when they
// agree), plus the rebuild's own violations.
func (s *Store) VerifyInboxHeads(ctx context.Context, ownerID, computerID string) ([]string, error) {
	ownerID, computerID = strings.TrimSpace(ownerID), strings.TrimSpace(computerID)
	fold, err := s.RebuildInboxHeads(ctx, ownerID, computerID)
	if err != nil {
		return nil, err
	}
	problems := append([]string(nil), fold.Violations...)
	cached := func(kind objectgraph.ObjectKind) ([]objectgraph.Object, error) {
		objs, err := s.ogListAllByMetadata(ctx, kind, "computer_id", computerID)
		out := objs[:0]
		for _, obj := range objs {
			if obj.OwnerID == ownerID && !obj.Tombstone {
				out = append(out, obj)
			}
		}
		return out, err
	}
	headObjs, err := cached(ogKindInboxHead)
	if err != nil {
		return nil, err
	}
	stored := map[string]int64{}
	for _, obj := range headObjs {
		head, err := decodeLifecycleObject[types.InboxHead](obj)
		if err != nil {
			return nil, err
		}
		stored[head.Inbox] = head.Seq
	}
	for inbox, seq := range fold.Heads {
		if stored[inbox] != seq {
			problems = append(problems, fmt.Sprintf("inbox %s: head %d, records %d", inbox, stored[inbox], seq))
		}
	}
	for inbox, seq := range stored {
		if _, ok := fold.Heads[inbox]; !ok {
			problems = append(problems, fmt.Sprintf("inbox %s: head %d with no records", inbox, seq))
		}
	}
	conversationObjs, err := cached(ogKindConversationHead)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	for _, obj := range conversationObjs {
		head, err := decodeLifecycleObject[types.ConversationHead](obj)
		if err != nil {
			return nil, err
		}
		seen[head.ConversationID] = true
		cachedJSON, _ := json.Marshal(head)
		foldJSON, _ := json.Marshal(fold.Conversations[head.ConversationID])
		if string(cachedJSON) != string(foldJSON) {
			problems = append(problems, fmt.Sprintf("conversation %s: head %s, records %s", head.ConversationID, cachedJSON, foldJSON))
		}
	}
	for id := range fold.Conversations {
		if !seen[id] {
			problems = append(problems, fmt.Sprintf("conversation %s: records with no head", id))
		}
	}
	cursorObjs, err := cached(ogKindInboxCursor)
	if err != nil {
		return nil, err
	}
	for _, obj := range cursorObjs {
		cursor, err := decodeLifecycleObject[types.InboxCursor](obj)
		if err != nil {
			return nil, err
		}
		for inbox, seq := range cursor.Cursors {
			if seq > fold.Heads[inbox] {
				problems = append(problems, fmt.Sprintf("cursor %s on %s: %d past head %d", cursor.ReaderAgentID, inbox, seq, fold.Heads[inbox]))
			}
		}
	}
	sort.Strings(problems)
	return problems, nil
}
