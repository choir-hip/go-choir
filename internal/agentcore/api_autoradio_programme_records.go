package agentcore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/calibration"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// The records of the programme call (design 12.2). Every one is a notice
// directive: a record that no request scan counts, that names no inbox so it
// wakes nothing, and that the desk-pack query leaves out (the same shape the
// turn route gives the listener's request). The text of a segment is the
// notice's body; the facts are subject refs.

const (
	kindRequest      = "autoradio.request"
	kindStretchStart = "autoradio.stretch_start"
	kindSegment      = "autoradio.segment"
	kindStretchIndex = "autoradio.stretch"
	kindTurn         = "autoradio.turn"
	kindNote         = "autoradio.note"
	kindLesson       = "autoradio.lesson"

	requestRecordPrefix      = "autoradio-request:"
	stretchStartRecordPrefix = "autoradio-stretch-start:"
	stretchIndexRecordPrefix = "autoradio-stretch:"
	segmentRecordPrefix      = "autoradio-seg:"
	noteRecordPrefix         = "autoradio-note:"
	lessonRecordPrefix       = "autoradio-lesson:"
)

// autoradioLedger is the slice of the guest store the programme call writes
// and reads through. The runtime's store is the one implementation in
// production; a test may put a decorator in front of it to make a write fail.
type autoradioLedger interface {
	AppendCommitmentRecord(ctx context.Context, ownerID, computerID string, rec types.CommitmentRecord) (string, error)
	GetCommitmentRecord(ctx context.Context, ownerID, computerID, recordID string) (*types.CommitmentRecord, error)
	ListCommitmentRecordsBySubjectRef(ctx context.Context, ownerID, computerID, key, value string, limit int) ([]types.CommitmentRecord, error)
	// ListCommitmentRecordsBySubjectRefOfComputer counts only this computer's
	// rows toward the limit (the run sheet replay pages by it).
	ListCommitmentRecordsBySubjectRefOfComputer(ctx context.Context, ownerID, computerID, key, value string, limit int) (records []types.CommitmentRecord, undecodable, rows int, err error)
	// ListCommitmentRecordsByBodyOfComputer is the predicate list the heard
	// reads use (calibration.HeardLedger); the scorecard reads page through
	// calibration.PagedLedger.
	ListCommitmentRecordsByBodyOfComputer(ctx context.Context, ownerID, computerID string, match map[string]string, limit int) (records []types.CommitmentRecord, undecodable, rows int, err error)
	calibration.PagedLedger
}

func (h *APIHandler) programmeLedger() autoradioLedger {
	if h.autoradioLedgerOverride != nil {
		return h.autoradioLedgerOverride
	}
	return h.rt.store
}

func segmentRecordID(stretchID, segmentID string) string {
	return segmentRecordPrefix + stretchID + "/" + segmentID
}

// noticeRecord builds one owner-private notice of the programme call.
func (c *programmeCall) noticeRecord(id, kind string, refs map[string]string, objective, body string, related []string, at time.Time) (types.CommitmentRecord, error) {
	label, ok := types.AutoradioKindLabel(kind)
	if !ok {
		return types.CommitmentRecord{}, fmt.Errorf("record kind %q is not in the table of design 12.2", kind)
	}
	out := map[string]string{"desk": autoradioDesk, "record_kind": kind}
	for k, v := range refs {
		out[k] = v
	}
	return types.CommitmentRecord{
		SchemaID: types.CommitmentRecordSchemaV1, RecordID: id, Kind: types.CommitmentKindDirective,
		Discrepancy: types.DiscrepancyUnresolved,
		Directive:   &types.CommitmentDirective{Subtype: types.CommitmentDirectiveNotice, Objective: objective, Body: body},
		Provenance: types.CommitmentProvenance{
			AgentID: autoradioDesk + ":" + c.ownerID, ModelID: c.memberName(), RunID: c.stretchID,
			CommittedAt: at.UTC().Format(time.RFC3339Nano), Labels: types.NewLabelSet(label).Strings(),
		},
		EmitterKind: types.CommitmentEmitterAgent, ComputerID: c.rt.cfg.ComputerID,
		RelatedIDs: related, SubjectRefs: out,
	}, nil
}

func (c *programmeCall) append(ctx context.Context, rec types.CommitmentRecord) error {
	_, err := c.ledger.AppendCommitmentRecord(ctx, c.ownerID, c.rt.cfg.ComputerID, rec)
	return err
}

func (c *programmeCall) get(ctx context.Context, id string) (*types.CommitmentRecord, error) {
	return c.ledger.GetCommitmentRecord(ctx, c.ownerID, c.rt.cfg.ComputerID, id)
}

// writeStart writes the stretch start record, then the request record that
// maps the request_id to the stretch (design 3.3). The start time the claims'
// deadlines derive from is read back from the stored start record.
func (c *programmeCall) writeStart(ctx context.Context) error {
	now := time.Now().UTC()
	startRefs := map[string]string{
		"stretch_id": c.stretchID, "started_at": now.Format(time.RFC3339Nano), "request_id": c.req.RequestID, "cause": c.req.Cause,
	}
	if c.input != nil && c.input.nbFound {
		// The input receipt: which notebook revision the model was shown, with
		// the hash and length of the text, never the text (rule 2: the text is
		// the notebook, which the listener reads).
		startRefs["notebook_revision_id"] = c.input.nbRevID
		startRefs["notebook_sha256"] = c.input.nbReceiptSHA
		startRefs["notebook_bytes"] = fmt.Sprint(c.input.nbReceiptLen)
		startRefs["marked_cases"] = fmt.Sprint(len(c.input.cases))
		startRefs["notebook_lines_shown"] = fmt.Sprint(len(c.input.nbLines))
		startRefs["notebook_lines_total"] = fmt.Sprint(c.input.nbTotal)
	}
	start, err := c.noticeRecord(stretchStartRecordPrefix+c.stretchID, kindStretchStart, startRefs, "stretch "+c.stretchID+" started", "", nil, now)
	if err != nil {
		return err
	}
	if err := c.append(ctx, start); err != nil {
		return err
	}
	stored, err := c.get(ctx, start.RecordID)
	if err != nil {
		return err
	}
	if stored == nil {
		return errors.New("stretch start record not readable after append")
	}
	at, err := time.Parse(time.RFC3339Nano, stored.SubjectRefs["started_at"])
	if err != nil {
		return fmt.Errorf("stretch start record: %w", err)
	}
	c.startedAt = at
	req, err := c.noticeRecord(requestRecordPrefix+c.req.RequestID, kindRequest, map[string]string{
		"request_id": c.req.RequestID, "stretch_id": c.stretchID, "cause": c.req.Cause,
	}, "programme request "+c.req.RequestID, "", []string{start.RecordID}, time.Now())
	if err != nil {
		return err
	}
	return c.append(ctx, req)
}

// writeSegment writes the segment record: the text the claim's hash names.
func (c *programmeCall) writeSegment(ctx context.Context, s *programmeSegment) error {
	cites, _ := json.Marshal(s.Cites)
	refs := map[string]string{
		"stretch_id": c.stretchID, "segment_id": s.ID, "kind": s.Kind, "sha256": s.SHA256,
		"blocks": fmt.Sprint(s.Blocks), "forecast": fmt.Sprint(s.Content), "cites": string(cites),
	}
	if s.BetweenAfter != "" {
		refs["between_after"], refs["between_before"] = s.BetweenAfter, s.BetweenBefore
	}
	if s.ClipID != "" {
		refs["clip_id"], refs["clip_seconds"] = s.ClipID, fmt.Sprint(s.ClipSeconds)
	}
	if s.TakeID != "" {
		refs["take_id"], refs["take_seconds"], refs["take_publisher"] = s.TakeID, fmt.Sprint(s.TakeSeconds), s.TakePublisher
	}
	rec, err := c.noticeRecord(segmentRecordID(c.stretchID, s.ID), kindSegment, refs, s.Title, s.Text, nil, time.Now())
	if err != nil {
		return err
	}
	return c.append(ctx, rec)
}

// writeClaim appends the segment's forecast and compares the stored claim with
// the one written, as the generic handler does: the append answers success
// when the id exists, so only the re-read says what is stored.
func (c *programmeCall) writeClaim(ctx context.Context, rec types.CommitmentRecord) error {
	rec.ComputerID = c.rt.cfg.ComputerID
	existing, err := c.get(ctx, rec.RecordID)
	if err != nil {
		return err
	}
	if existing == nil {
		if err := c.append(ctx, rec); err != nil {
			return err
		}
		if existing, err = c.get(ctx, rec.RecordID); err != nil {
			return err
		}
		if existing == nil {
			return errors.New("claim not readable after append")
		}
	}
	if !calibration.SameClaim(*existing, rec) {
		return fmt.Errorf("claim %s is stored differently from the claim written", rec.RecordID)
	}
	return nil
}

type droppedLine struct {
	N      int    `json:"n"`
	Reason string `json:"reason"`
}

// writeIndex writes the stretch index: the claim ids, how the call ended and
// why lines were dropped.
func (c *programmeCall) writeIndex(ctx context.Context, status string) (string, error) {
	dropped := c.dropped
	if len(dropped) > 100 { // dropped_count stays exact
		dropped = dropped[:100]
	}
	droppedJSON, _ := json.Marshal(dropped)
	ids, _ := json.Marshal(c.segmentIDs)
	timings, _ := json.Marshal(c.timings(time.Now()))
	idxRefs := map[string]string{
		"stretch_id": c.stretchID, "request_id": c.req.RequestID, "status": status, "segments": fmt.Sprint(len(c.segmentIDs)),
		"claims": fmt.Sprint(len(c.claimIDs)), "dropped_count": fmt.Sprint(len(c.dropped)), "dropped": string(droppedJSON),
		"provider": c.provider, "model": c.model, "timings_ms": string(timings), "client_gone": fmt.Sprint(c.writeErr != nil),
	}
	if c.nbSummary != "" {
		idxRefs["notebook"] = c.nbSummary
	}
	rec, err := c.noticeRecord(stretchIndexRecordPrefix+c.stretchID, kindStretchIndex, idxRefs, "stretch "+c.stretchID+" "+status, string(ids), append([]string{}, c.claimIDs...), time.Now())
	if err != nil {
		return "", err
	}
	return rec.RecordID, c.append(ctx, rec)
}

// readSegments reads the stored segments of a stretch in order, by their
// deterministic ids: the leading run of segments that have what they should
// (a content segment needs its claim, read by its deterministic id), until one
// lacks it or is missing, or n are read (n < 0: all). It never returns the
// segment after a gap.
func (c *programmeCall) readSegments(ctx context.Context, stretchID string, n int) ([]programmeSegment, error) {
	var out []programmeSegment
	for i := 0; n < 0 || i < n; i++ {
		id := fmt.Sprintf("%s.s%d", stretchID, i)
		rec, err := c.get(ctx, segmentRecordID(stretchID, id))
		if err != nil {
			return nil, err
		}
		if rec == nil || rec.Directive == nil {
			break
		}
		s := programmeSegment{ID: id, Kind: rec.SubjectRefs["kind"], Title: rec.Directive.Objective, Text: rec.Directive.Body, SHA256: rec.SubjectRefs["sha256"]}
		_, _ = fmt.Sscanf(rec.SubjectRefs["blocks"], "%d", &s.Blocks)
		_ = json.Unmarshal([]byte(rec.SubjectRefs["cites"]), &s.Cites)
		s.Content = rec.SubjectRefs["forecast"] == "true"
		s.BetweenAfter, s.BetweenBefore = rec.SubjectRefs["between_after"], rec.SubjectRefs["between_before"]
		if id := rec.SubjectRefs["clip_id"]; id != "" {
			s.ClipID = id
			_, _ = fmt.Sscanf(rec.SubjectRefs["clip_seconds"], "%g", &s.ClipSeconds)
		}
		if id := rec.SubjectRefs["take_id"]; id != "" {
			s.TakeID = id
			_, _ = fmt.Sscanf(rec.SubjectRefs["take_seconds"], "%g", &s.TakeSeconds)
			s.TakePublisher = rec.SubjectRefs["take_publisher"]
		}
		if s.Content {
			// A content segment is streamed only with its claim: the segment
			// record is written first, so a record with no claim is a write
			// that failed or a process that stopped between the two.
			claim, err := c.get(ctx, calibration.ClaimRecordID(calibration.AutoradioAgent, stretchID+"/"+id, ""))
			if err != nil {
				return nil, err
			}
			if claim == nil {
				break
			}
			s.ClaimID = claim.RecordID
			if claim.Calibration != nil && claim.Calibration.Claim != nil {
				s.P = claim.Calibration.Claim.Probability
				s.CausedBy = strings.TrimPrefix(claim.Calibration.Claim.Criterion, "caused_by=")
			}
		}
		out = append(out, s)
	}
	return out, nil
}

// listNotices fetches at most 5,000 records of the kind and then filters by id
// prefix, so a listener with more than that of one kind is read short without
// saying so: a known bound, left as it is.
//
// listNotices reads the listener's own records of one kind (turn requests,
// notebook notes, lessons): the newest first, at most limit, with the id
// prefix that names the record rather than a reply.
func (c *programmeCall) listNotices(ctx context.Context, kind, idPrefix string, limit int) ([]types.CommitmentRecord, error) {
	recs, err := c.ledger.ListCommitmentRecordsBySubjectRef(ctx, c.ownerID, c.rt.cfg.ComputerID, "record_kind", kind, 5000)
	if err != nil {
		return nil, err
	}
	var out []types.CommitmentRecord
	for _, r := range recs {
		if strings.HasPrefix(r.RecordID, idPrefix) && r.Directive != nil {
			out = append(out, r)
		}
	}
	sortRecordsNewestFirst(out)
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func sortRecordsNewestFirst(recs []types.CommitmentRecord) {
	at := func(r types.CommitmentRecord) time.Time {
		t, _ := time.Parse(time.RFC3339Nano, r.Provenance.CommittedAt)
		return t
	}
	sort.SliceStable(recs, func(i, j int) bool { return at(recs[i]).After(at(recs[j])) })
}
