package yaegikernel

import (
	"encoding/json"
	"fmt"
)

// A desk cell's frame is one transport frame (MaxFramePayload), and two of
// its parts grow with a desk's backlog: the records in (cursor, head] and the
// pending update packets. An unbounded part wedges the desk: the Autopaper's
// management desk had a backlog of 5.5 MB and then 8.6 MB, every cell failed
// with "frame: payload 5489824 bytes exceeds 4194304", and the backlog could
// then never be read, so it only grew
// (docs/evidence/paper-token-budget-2026-11-09.md section 1.3).
//
// The frame therefore carries a window of each, oldest first, and says what
// remains (FrameBacklog). The cursor moves only through the records a
// choir.Records() call returned, so the next cell's window starts where this
// one's ended and the backlog drains in order.
const (
	// FrameRecordsBudget and FrameUpdatesBudget bound the JSON bytes of the
	// records and of the update packets one frame carries. Together they use
	// half the frame; the rest is the doc, pack, inbox and analysis.
	FrameRecordsBudget = 1 << 20
	FrameUpdatesBudget = 1 << 20
	// FrameItemBudget bounds one record or one update. A larger one is shown
	// as a stub that keeps its identity (seq, ids, kind, sender), so the
	// cursor can still pass it and the desk can still name it.
	FrameItemBudget = 256 << 10
	// FrameUpdateMaxBytes bounds one update that can be shown whole. A larger
	// one cannot be shown whole in a frame, so the host settles it with a
	// visible cause (too_large_to_show) instead of holding the head of the
	// window or consuming it unseen.
	FrameUpdateMaxBytes = 2 << 20
	// frameHeadroom is kept free under MaxFramePayload.
	frameHeadroom = 64 << 10
	// stubTextBytes bounds the text a stub keeps.
	stubTextBytes = 2 << 10
)

// FrameBacklog says what a windowed frame left out. It rides the frame and
// is read through choir.Context() (records_remaining, updates_remaining,
// backlog_truncated).
type FrameBacklog struct {
	// RecordsRemaining counts inbox sequence numbers past the window (or, when
	// the transport cut the frame, records dropped), UpdatesRemaining the
	// pending updates past the window.
	RecordsRemaining int64 `json:"records_remaining,omitempty"`
	UpdatesRemaining int   `json:"updates_remaining,omitempty"`
	// UpdatesTooLarge counts the pending updates the host settled at this
	// cell's start because one could not be shown whole (UpdateTooLarge).
	UpdatesTooLarge int `json:"updates_too_large,omitempty"`
	// Truncated is set when the transport, not the host's window, cut the
	// frame to fit MaxFramePayload.
	Truncated bool `json:"truncated,omitempty"`
}

// distinctSeqs counts the sequence numbers in records (in seq order).
func distinctSeqs(recs []InboxRecord) int {
	n := 0
	for i, r := range recs {
		if i == 0 || r.Seq != recs[i-1].Seq {
			n++
		}
	}
	return n
}

// RecordSize is a record's size on the wire.
func RecordSize(r InboxRecord) int {
	b, err := json.Marshal(r)
	if err != nil {
		return FrameItemBudget
	}
	return len(b)
}

// UpdateSize is an update's size on the wire.
func UpdateSize(u PendingUpdate) int { return updateSize(u) }

func updateSize(u PendingUpdate) int {
	b, err := json.Marshal(u)
	if err != nil {
		return FrameItemBudget
	}
	return len(b)
}

func clipText(s string) string {
	if len(s) <= stubTextBytes {
		return s
	}
	cut := stubTextBytes
	for cut > 0 && s[cut]&0xC0 == 0x80 {
		cut--
	}
	return s[:cut]
}

// stubRecord keeps a record's identity and marks the rest cut. A stub is
// deliberately past the cursor: the desk is told the body was cut (and how
// large it was), and the cursor moves past it with the rest of its window, so
// one record too large for a frame cannot hold the desk's inbox. The full
// record stays on the tape.
func stubRecord(r InboxRecord, size int) InboxRecord {
	return InboxRecord{
		Seq: r.Seq, RecordID: r.RecordID, Kind: r.Kind, From: r.From,
		ConversationID: r.ConversationID, Round: r.Round, Final: r.Final,
		ParentID: r.ParentID, RelatedIDs: r.RelatedIDs, CreatedAt: r.CreatedAt,
		Objective: clipText(r.Objective),
		Body:      fmt.Sprintf("[record cut: its %d bytes do not fit a cell frame (one record is shown whole up to %d bytes, and a seq that is still over the frame is stubbed largest first); its identity and this note remain, the full record is on the tape]", size, FrameItemBudget),
	}
}

// WindowRecords returns the oldest records, in order, that fit budget bytes.
// A cut falls between sequence numbers: records of one seq (a Cast and the
// captures records it delegated share it) stay together, so the cursor can
// pass the last seq returned without skipping a record. The first seq is
// always returned, so a window is never empty; a record over FrameItemBudget
// is returned as a stub. cut reports whether records were left out.
func WindowRecords(recs []InboxRecord, budget int) (kept []InboxRecord, cut bool) {
	total := 0
	for i := 0; i < len(recs); {
		j := i
		groupSize := 0
		var group []InboxRecord
		for j < len(recs) && recs[j].Seq == recs[i].Seq {
			r := recs[j]
			size := RecordSize(r)
			if size > FrameItemBudget {
				r = stubRecord(r, size)
				size = RecordSize(r)
			}
			group = append(group, r)
			groupSize += size
			j++
		}
		if len(kept) > 0 && total+groupSize > budget {
			return kept, true
		}
		kept = append(kept, group...)
		total += groupSize
		i = j
	}
	return kept, false
}

// TailRecords returns the newest records that fit budget bytes, in order.
func TailRecords(recs []InboxRecord, budget int) []InboxRecord {
	total, start := 0, len(recs)
	for i := len(recs) - 1; i >= 0; i-- {
		size := RecordSize(recs[i])
		if size > FrameItemBudget || total+size > budget {
			break
		}
		total += size
		start = i
	}
	return recs[start:]
}

// WindowUpdates returns the oldest updates, in order, that fit budget bytes,
// and how many were left out. The first update is always returned, whole: an
// update is never shown cut, because the host consumes what the cell was shown
// (a Texture turn delivers the reports its cell saw, a Management report
// consumes the ids its wake turn named), and a cut update would be consumed
// unread. The caller has already settled any update over FrameUpdateMaxBytes
// (it cannot be shown whole), so the first fits the frame.
//
// The oldest window repeats until its updates are disposed: a desk that
// leaves them pending is shown the same ones again, not the ones past them.
func WindowUpdates(updates []PendingUpdate, budget int) (kept []PendingUpdate, remaining int) {
	total := 0
	for i, u := range updates {
		size := updateSize(u)
		if len(kept) > 0 && total+size > budget {
			return kept, len(updates) - i
		}
		kept = append(kept, u)
		total += size
	}
	return kept, 0
}

// fitFrame marshals the frame, and when it is over the transport bound cuts
// the backlog parts, records and updates, until it fits, saying so in
// Backlog. The host windows both parts already, so this is the last guard.
// It returns the frame it marshalled, because the host consumes what the cell
// was shown and must know which updates that was. The cut keeps at least one
// record past the cursor (RecordsAfter), so the cursor can still advance. A
// frame still over the bound has a doc, pack or analysis that alone does not
// fit: it returns an error having written nothing, so the worker is not
// poisoned by the failed write.
func fitFrame(frame SessionFrame) ([]byte, SessionFrame, error) {
	limit := MaxFramePayload - frameHeadroom
	raw, err := json.Marshal(frame)
	if err != nil {
		return nil, frame, fmt.Errorf("desk eval marshal: %w", err)
	}
	if len(raw) <= limit {
		return raw, frame, nil
	}
	backlog := FrameBacklog{}
	if frame.Backlog != nil {
		backlog = *frame.Backlog
	}
	backlog.Truncated = true
	frame.Backlog = &backlog
	remarshal := func() error {
		var merr error
		raw, merr = json.Marshal(frame)
		if merr != nil {
			return fmt.Errorf("desk eval marshal: %w", merr)
		}
		return nil
	}
	// Records the run already read (at or below the cursor) are context:
	// they go first, and are not backlog.
	if frame.RecordsAfter > 0 {
		unread := frame.Records[:0:0]
		for _, r := range frame.Records {
			if r.Seq > frame.RecordsAfter {
				unread = append(unread, r)
			}
		}
		frame.Records = unread
		if err := remarshal(); err != nil {
			return nil, frame, err
		}
	}
	for len(raw) > limit && (distinctSeqs(frame.Records) > 1 || len(frame.Updates) > 1) {
		// Halve the larger part by bytes; the cursor follows what is kept.
		recBytes, updBytes := 0, 0
		for _, r := range frame.Records {
			recBytes += RecordSize(r)
		}
		for _, u := range frame.Updates {
			updBytes += updateSize(u)
		}
		if distinctSeqs(frame.Records) > 1 && (recBytes >= updBytes || len(frame.Updates) <= 1) {
			// Whole seqs only: the cursor passes a seq once it is shown, so
			// a seq's records (a Cast and the captures records it delegated
			// share one) are all shown or none are.
			kept, _ := WindowRecords(frame.Records, recBytes/2)
			backlog.RecordsRemaining += int64(len(frame.Records) - len(kept))
			frame.Records = kept
		} else {
			kept, _ := WindowUpdates(frame.Updates, updBytes/2)
			if len(kept) == len(frame.Updates) {
				kept = kept[:1]
			}
			backlog.UpdatesRemaining += len(frame.Updates) - len(kept)
			frame.Updates = kept
		}
		if err := remarshal(); err != nil {
			return nil, frame, err
		}
	}
	// What is left is a lone seq of records, and perhaps one update, still
	// over the frame. Updates go first: left out they stay pending, and
	// nothing is lost. Then the seq's records are stubbed, largest first, until
	// it fits: every record reaches the desk whole or as a visible identity
	// stub, so the cursor always advances past a seq and the body stays on the
	// tape. Records are the cursor's, updates are the window's.
	if len(raw) > limit && len(frame.Updates) > 0 {
		backlog.UpdatesRemaining += len(frame.Updates)
		frame.Updates = nil
		if err := remarshal(); err != nil {
			return nil, frame, err
		}
	}
	if len(raw) > limit && len(frame.Records) > 0 {
		frame.Records = append([]InboxRecord(nil), frame.Records...)
		for len(raw) > limit {
			largest, largestSize := -1, 4<<10
			for i, r := range frame.Records {
				if size := RecordSize(r); size > largestSize {
					largest, largestSize = i, size
				}
			}
			if largest < 0 {
				break
			}
			frame.Records[largest] = stubRecord(frame.Records[largest], largestSize)
			if err := remarshal(); err != nil {
				return nil, frame, err
			}
		}
	}
	// Even all stubbed it does not fit (thousands of records in one seq):
	// nothing of it is shown, the cursor does not pass it, and the frame says
	// the desk was cut.
	if len(raw) > limit && len(frame.Records) > 0 {
		backlog.RecordsRemaining += int64(len(frame.Records))
		frame.Records = nil
		if err := remarshal(); err != nil {
			return nil, frame, err
		}
	}
	if len(raw) > MaxFramePayload {
		return nil, frame, fmt.Errorf("desk eval frame is %d bytes with no backlog, over the %d bound: the document, pack or analysis alone does not fit", len(raw), MaxFramePayload)
	}
	return raw, frame, nil
}
