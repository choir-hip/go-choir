package agentcore

// The programme route puts what it commits on the listener's server run sheet
// (station 1 of the settlement slice, docs/reference/AUTORADIO-SHEET.md "The
// desk's entries"). It does so in process, through the one reducer and the
// one bound set (applySheetOps, trusted: the desk acts in process, never over
// HTTP), so the server holds every entry the phone plays, with the forecast it
// carries. The route never writes an outcome: the sheet's own op path does,
// when the listener's device says what happened.

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/calibration"
	"github.com/yusefmosiah/go-choir/internal/runsheet"
)

// programmeSheet is the slice of the run sheet the programme call uses.
type programmeSheet interface {
	place(ctx context.Context, ops []runsheet.Op) ([]runsheet.Result, error)
	ahead(ctx context.Context) ([]runsheet.Entry, error)
}

type programmeSheetOf struct {
	h       *APIHandler
	ownerID string
}

func (p programmeSheetOf) place(ctx context.Context, ops []runsheet.Op) ([]runsheet.Result, error) {
	return p.h.applySheetOps(ctx, p.ownerID, sheetOwnerID, ops, true)
}

func (p programmeSheetOf) ahead(ctx context.Context) ([]runsheet.Entry, error) {
	ls, err := p.h.rt.sheetStore().get(ctx, p.h, p.ownerID, sheetOwnerID)
	if err != nil {
		return nil, err
	}
	ls.mu.Lock()
	defer ls.mu.Unlock()
	var out []runsheet.Entry
	for _, e := range ls.sheet.Entries {
		if e.State == runsheet.StateAhead {
			out = append(out, e)
		}
	}
	return out, nil
}

// programmeEntryID is the id of a segment's entry on the sheet: the phone's
// own grammar (d-, the segment id, characters outside [A-Za-z0-9_.-] made -,
// 64 bytes), so the phone's entry and the server's are one.
func programmeEntryID(segmentID string) string {
	var b strings.Builder
	b.WriteString("d-")
	for _, r := range segmentID {
		ok := r >= '0' && r <= '9' || r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' || r == '_' || r == '.' || r == '-'
		if ok {
			b.WriteRune(r)
		} else {
			b.WriteByte('-')
		}
		if b.Len() >= 64 {
			break
		}
	}
	return b.String()
}

// programmeEntryKind maps the programme's kinds as the phone does.
func programmeEntryKind(kind string) runsheet.Kind {
	switch kind {
	case "opening", "ident":
		return runsheet.KindOpening
	case "transition", "bridge":
		return runsheet.KindTransition
	case "podcast_clip":
		return runsheet.KindClip
	}
	return runsheet.KindStory // story, update, headlines
}

func cutBytes(s string, n int) string {
	if len(s) <= n {
		return s
	}
	s = s[:n]
	for len(s) > 0 && !utf8Valid(s) {
		s = s[:len(s)-1]
	}
	return s
}

func utf8Valid(s string) bool { return strings.ToValidUTF8(s, "") == s }

const programmeOriginPrefix = "programme/"

// programmeEntry is the entry a committed segment is on the sheet. The text
// is what the segment record holds (cut to the sheet's bound), the forecast
// is the segment's claim and its stated probability.
func (c *programmeCall) programmeEntry(s *programmeSegment) (runsheet.Entry, bool) {
	text := cutBytes(strings.TrimSpace(s.Text), runsheet.MaxTextBytes)
	if text == "" {
		return runsheet.Entry{}, false
	}
	kind := programmeEntryKind(s.Kind)
	title := s.Title
	switch kind {
	case runsheet.KindOpening:
		title = "Opening"
	case runsheet.KindTransition:
		title = ""
	}
	e := runsheet.Entry{
		ID: programmeEntryID(s.ID), Kind: kind, PlacedBy: runsheet.ByDesk, State: runsheet.StateAhead,
		Title: cutBytes(title, runsheet.MaxTitleBytes), Text: text, EstSeconds: float64(estSeconds(text)),
		Origin: programmeOriginPrefix + c.stretchID,
	}
	seen := map[string]bool{}
	for _, ct := range s.Cites {
		if ct.Ref == "" || seen[ct.Ref] || len(e.Cites) >= runsheet.MaxCites {
			continue
		}
		seen[ct.Ref] = true
		role := runsheet.Role(ct.Role)
		if role != runsheet.RoleSubject && role != runsheet.RoleQuoted && role != runsheet.RoleMentioned {
			role = runsheet.RoleMentioned
		}
		e.Cites = append(e.Cites, runsheet.Cite{Ref: cutBytes(ct.Ref, runsheet.MaxURLBytes), Role: role, Label: cutBytes(ct.Label, runsheet.MaxRefBytes)})
	}
	for _, ct := range e.Cites {
		if strings.HasPrefix(ct.Ref, "pub:") {
			e.Source = &runsheet.Source{Ref: ct.Ref, Label: ct.Label}
			break
		}
	}
	if s.BetweenAfter != "" {
		e.Between = &runsheet.Between{After: programmeEntryID(s.BetweenAfter), Before: programmeEntryID(s.BetweenBefore)}
	}
	if kind == runsheet.KindClip && s.ClipID != "" {
		e.Source = &runsheet.Source{Ref: "clip:" + s.ClipID, Label: cutBytes(s.Title, runsheet.MaxRefBytes), ClipID: s.ClipID}
		e.EstSeconds += s.ClipSeconds
	}
	if s.ClaimID != "" {
		e.Forecast = &runsheet.Forecast{PHeard: s.P, CausedBy: cutBytes(s.CausedBy, runsheet.MaxShortBytes), ClaimID: s.ClaimID}
	}
	return e, true
}

// placeOnSheet puts the segment's entry at the end of the sheet, by the desk.
// The op id follows the entry id (as the phone's does), so a replay of the
// request, a second call for the same segment and a later `place` of the same
// id by the phone are each one entry: the first applies, the rest are
// answered from the tape or dropped duplicate_id. A failure is logged and
// counted, never fatal: the programme is what the listener hears, the sheet is
// the custodian's copy of it.
func (c *programmeCall) placeOnSheet(ctx context.Context, s *programmeSegment) {
	if c.sheet == nil {
		return
	}
	e, ok := c.programmeEntry(s)
	if !ok {
		return
	}
	if !c.madeRoom {
		c.madeRoom = true
		c.makeRoom(ctx)
	}
	reason := "written to extend the programme"
	if c.req.Cause == "open" {
		reason = "written to open the listening"
	}
	opID := ("o" + e.ID)
	if len(opID) > 64 {
		opID = opID[:64]
	}
	res, err := c.sheet.place(ctx, []runsheet.Op{{
		OpID: opID, By: runsheet.ByDesk, Op: runsheet.OpPlace, At: runsheet.AtEnd, Entry: &e, Reason: reason,
		Origin: programmeOriginPrefix + c.stretchID,
	}})
	switch {
	case err != nil:
		c.placeFailed++
		log.Printf("autoradio programme: stretch %s: entry %s not placed on the run sheet: %v", c.stretchID, e.ID, err)
	case len(res) != 1 || (!res[0].Applied && res[0].Dropped != runsheet.DroppedDuplicate && res[0].Dropped != runsheet.DroppedDuplicateID):
		c.placeFailed++
		why := "no answer"
		if len(res) == 1 {
			why = res[0].Dropped
		}
		log.Printf("autoradio programme: stretch %s: entry %s not placed on the run sheet: %s", c.stretchID, e.ID, why)
	default:
		c.placed++
	}
}

// makeRoom removes, once per call and only when the ahead list is nearly
// full, the desk's entries that were never played and whose stretch is older
// than the forecasts' lapse (12 hours): a phone that went away leaves its
// unplayed entries ahead, and they would otherwise fill the sheet's 100 ahead
// places for ever. Their forecasts lapse at the deadline, unsettled, and stay
// on the scorecard as open or overdue. A listener's or the phone's own entry
// is never removed, and neither is a desk entry whose stretch is still within
// the lapse.
func (c *programmeCall) makeRoom(ctx context.Context) {
	ahead, err := c.sheet.ahead(ctx)
	if err != nil || len(ahead) < sheetMaxAhead-programmeMaxSegments {
		return
	}
	starts := map[string]time.Time{}
	var ops []runsheet.Op
	for _, e := range ahead {
		sid, ok := strings.CutPrefix(e.Origin, programmeOriginPrefix)
		if !ok || e.PlacedBy != runsheet.ByDesk || sid == c.stretchID {
			continue
		}
		at, seen := starts[sid]
		if !seen {
			rec, gerr := c.get(ctx, stretchStartRecordPrefix+sid)
			if gerr == nil && rec != nil {
				at, _ = time.Parse(time.RFC3339Nano, rec.SubjectRefs["started_at"])
			}
			starts[sid] = at
		}
		if at.IsZero() || time.Since(at) < calibration.AutoradioLapse {
			continue
		}
		ops = append(ops, runsheet.Op{
			OpID: ("rm-" + e.ID + "-" + c.stretchID), By: runsheet.ByDesk, Op: runsheet.OpRemove, ID: e.ID,
			Reason: "never played; its stretch is more than 12 hours old",
		})
	}
	for i := range ops {
		if len(ops[i].OpID) > 64 {
			ops[i].OpID = ops[i].OpID[:64]
		}
	}
	if len(ops) == 0 {
		return
	}
	if len(ops) > sheetMaxOps {
		ops = ops[:sheetMaxOps]
	}
	if _, err := c.sheet.place(ctx, ops); err != nil {
		log.Printf("autoradio programme: stretch %s: removing %d lapsed entries: %v", c.stretchID, len(ops), fmt.Sprint(err))
	}
}
