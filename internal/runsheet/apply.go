package runsheet

import "fmt"

// Interpretations of the frozen interface (each is also in the slice report):
//
//   - Duplicate detection needs memory, so RunSheet carries applied_ops (the
//     last 512 applied op_ids). A dropped op is not remembered: it changed
//     nothing, so a resend is judged afresh.
//   - Dropped reasons beyond duplicate and not_ahead: not_on_air (skip),
//     not_phone and bad_transition and other_on_air and unknown_id (aired),
//     stale_version (clear, previous), no_previous, duplicate_id (place),
//     reason_required, invalid (malformed op or entry).
//   - The reducer reads no clock. ended.at is whatever the op carried ("" when
//     nothing). skip and `now` take at and last_block from op.ended when given.
//   - skip, previous and aired never promote anything to on_air; only an
//     aired op from the phone puts an entry on air. After a skip nothing is on
//     air until the phone says so.
//   - previous returns the entry on air and the most recent heard or skipped
//     entry to ahead, in place; nothing is on air afterwards.
//   - An entry that goes on air is moved to the head of the ahead entries so the
//     order stays tail, on_air, ahead.
//   - Entries heard or skipped beyond the most recent 50 are trimmed. The ahead
//     list is not capped here: place always applies.
//   - place needs state "ahead" (invalid otherwise), a fresh id, and the text
//     the kind requires.
//   - Check order: op shape, duplicate, aired's authority, reason, then the
//     op's own rules.

// Apply applies op to s and returns the new sheet and what happened. s is not
// modified. An op name outside the vocabulary is an error (it cannot arise
// from decoded JSON, which rejects it earlier).
func Apply(s RunSheet, op Op) (RunSheet, Result, error) {
	if !op.Op.valid() {
		return s, Result{OpID: op.OpID, Version: s.Version}, fmt.Errorf("runsheet: unknown op %q", op.Op)
	}
	out := clone(s)
	drop := func(why string) (RunSheet, Result, error) {
		return s, Result{OpID: op.OpID, Dropped: why, Version: s.Version}, nil
	}
	if !ValidID(op.OpID) || !op.By.valid() {
		return drop(DroppedInvalid)
	}
	for _, id := range s.AppliedOps {
		if id == op.OpID {
			return drop(DroppedDuplicate)
		}
	}
	if op.Op == OpAired && op.By != ByPhone {
		return drop(DroppedNotPhone)
	}
	if len(op.Reason) > MaxReasonBytes {
		return drop(DroppedInvalid)
	}
	if op.By == ByDesk && op.Reason == "" {
		return drop(DroppedReasonNeeded)
	}
	if why := out.apply(op); why != "" {
		return drop(why)
	}
	out.trimTail()
	out.Version = s.Version + 1
	out.AppliedOps = append(out.AppliedOps, op.OpID)
	if n := len(out.AppliedOps) - MaxAppliedOps; n > 0 {
		out.AppliedOps = append([]string(nil), out.AppliedOps[n:]...)
	}
	return out, Result{OpID: op.OpID, Applied: true, Version: out.Version}, nil
}

// ApplyAll applies ops in order and returns every result.
func ApplyAll(s RunSheet, ops []Op) (RunSheet, []Result, error) {
	results := make([]Result, 0, len(ops))
	for _, op := range ops {
		var r Result
		var err error
		s, r, err = Apply(s, op)
		if err != nil {
			return s, results, err
		}
		results = append(results, r)
	}
	return s, results, nil
}

func clone(s RunSheet) RunSheet {
	c := s
	c.Entries = append(make([]Entry, 0, len(s.Entries)+1), s.Entries...)
	c.AppliedOps = append([]string(nil), s.AppliedOps...)
	return c
}

func (s *RunSheet) index(id string) int {
	for i := range s.Entries {
		if s.Entries[i].ID == id {
			return i
		}
	}
	return -1
}

func (s *RunSheet) onAirIndex() int {
	for i := range s.Entries {
		if s.Entries[i].State == StateOnAir {
			return i
		}
	}
	return -1
}

// firstAhead is the index where the ahead entries begin.
func (s *RunSheet) firstAhead() int {
	if oi := s.onAirIndex(); oi >= 0 {
		return oi + 1
	}
	n := 0
	for _, e := range s.Entries {
		if e.State == StateHeard || e.State == StateSkipped {
			n++
		}
	}
	return n
}

func (s *RunSheet) insertAt(i int, e Entry) {
	s.Entries = append(s.Entries, Entry{})
	copy(s.Entries[i+1:], s.Entries[i:])
	s.Entries[i] = e
}

func (s *RunSheet) removeAt(i int) Entry {
	e := s.Entries[i]
	s.Entries = append(s.Entries[:i], s.Entries[i+1:]...)
	return e
}

// slot is the index an ahead entry takes for at (not now), on the sheet as it
// stands.
func (s *RunSheet) slot(at At, after string) int {
	first := s.firstAhead()
	switch at {
	case AtNext:
		return first
	case AtEnd:
		return len(s.Entries)
	case AtQueued:
		last := -1
		for i := first; i < len(s.Entries); i++ {
			if s.Entries[i].State == StateAhead && s.Entries[i].PlacedBy == ByListener {
				last = i
			}
		}
		if last < 0 {
			return first
		}
		return last + 1
	case AtAfter:
		if i := s.index(after); i >= 0 && (s.Entries[i].State == StateAhead || s.Entries[i].State == StateOnAir) {
			return i + 1
		}
		return first
	}
	return first
}

func endedFor(op Op, outcome Outcome) *Ended {
	e := &Ended{Outcome: outcome}
	if op.Ended != nil {
		e.At = op.Ended.At
		e.LastBlock = op.Ended.LastBlock
	}
	return e
}

// putAhead places e (an ahead entry, not yet on the sheet) at the position
// at names, cutting the entry on air when at is now.
func (s *RunSheet) putAhead(e Entry, at At, after string, op Op) {
	if at != AtNow {
		s.insertAt(s.slot(at, after), e)
		return
	}
	if oi := s.onAirIndex(); oi >= 0 {
		s.Entries[oi].State = StateSkipped
		s.Entries[oi].Ended = endedFor(op, OutcomeCut)
	}
	e.State = StateOnAir
	e.Ended = nil
	s.insertAt(s.firstAhead(), e)
	s.OnAir = e.ID
	s.PositionSeconds = 0
}

func validEntry(e *Entry) bool {
	if e == nil || !ValidID(e.ID) || !e.Kind.valid() || !e.PlacedBy.valid() {
		return false
	}
	if len(e.Title) > MaxTitleBytes || len(e.Text) > MaxTextBytes {
		return false
	}
	if e.Kind.TextRequired() && e.Text == "" {
		return false
	}
	if e.Ended != nil && !e.Ended.Outcome.valid() {
		return false
	}
	return true
}

// apply performs op on s (already a private copy) and returns the dropped
// reason, or "" when it applied.
func (s *RunSheet) apply(op Op) string {
	switch op.Op {
	case OpPlace:
		e := op.Entry
		if !validEntry(e) || e.State != StateAhead || !op.At.valid() || (op.At == AtAfter && op.After == "") {
			return DroppedInvalid
		}
		if s.index(e.ID) >= 0 {
			return DroppedDuplicateID
		}
		s.putAhead(*e, op.At, op.After, op)

	case OpMove:
		if op.ID == "" || !op.At.valid() || (op.At == AtAfter && op.After == "") {
			return DroppedInvalid
		}
		i := s.index(op.ID)
		if i < 0 || s.Entries[i].State != StateAhead {
			return DroppedNotAhead
		}
		e := s.removeAt(i)
		s.putAhead(e, op.At, op.After, op)

	case OpRemove:
		if op.ID == "" {
			return DroppedInvalid
		}
		i := s.index(op.ID)
		if i < 0 || s.Entries[i].State != StateAhead {
			return DroppedNotAhead
		}
		s.removeAt(i)

	case OpRewrite:
		if op.ID == "" || op.Text == "" || len(op.Text) > MaxTextBytes {
			return DroppedInvalid
		}
		i := s.index(op.ID)
		if i < 0 || s.Entries[i].State != StateAhead {
			return DroppedNotAhead
		}
		s.Entries[i].Text = op.Text

	case OpSkip:
		if op.ID == "" {
			return DroppedInvalid
		}
		i := s.index(op.ID)
		if i < 0 || s.Entries[i].State != StateOnAir {
			return DroppedNotOnAir
		}
		s.Entries[i].State = StateSkipped
		s.Entries[i].Ended = endedFor(op, OutcomeSkipped)
		s.OnAir = ""
		s.PositionSeconds = 0

	case OpPrevious:
		if op.BaseVersion != s.Version {
			return DroppedStaleVersion
		}
		last := -1
		for i, e := range s.Entries {
			if e.State == StateHeard || e.State == StateSkipped {
				last = i
			}
		}
		if last < 0 {
			return DroppedNoPrevious
		}
		for i := range s.Entries {
			if i == last || s.Entries[i].State == StateOnAir {
				s.Entries[i].State = StateAhead
				s.Entries[i].Ended = nil
			}
		}
		s.OnAir = ""
		s.PositionSeconds = 0

	case OpClear:
		if op.BaseVersion != s.Version {
			return DroppedStaleVersion
		}
		kept := s.Entries[:0]
		for _, e := range s.Entries {
			if e.State != StateAhead {
				kept = append(kept, e)
			}
		}
		s.Entries = kept

	case OpAired:
		return s.aired(op)
	}
	return ""
}

func (s *RunSheet) aired(op Op) string {
	if op.ID == "" || (op.State != StateOnAir && op.State != StateHeard && op.State != StateSkipped) {
		return DroppedInvalid
	}
	if op.Ended != nil && !op.Ended.Outcome.valid() {
		return DroppedInvalid
	}
	i := s.index(op.ID)
	if i < 0 {
		return DroppedUnknownID
	}
	cur := s.Entries[i].State
	switch {
	case cur == StateAhead && op.State == StateOnAir:
		if s.onAirIndex() >= 0 {
			return DroppedOtherOnAir
		}
		e := s.removeAt(i)
		e.State = StateOnAir
		e.Ended = nil
		s.insertAt(s.firstAhead(), e)
		s.OnAir = e.ID
		s.PositionSeconds = 0
		if op.PositionSeconds != nil {
			s.PositionSeconds = *op.PositionSeconds
		}
	case cur == StateOnAir && (op.State == StateHeard || op.State == StateSkipped):
		s.Entries[i].State = op.State
		if op.Ended != nil {
			c := *op.Ended
			s.Entries[i].Ended = &c
		} else if op.State == StateHeard {
			s.Entries[i].Ended = &Ended{Outcome: OutcomeFinished}
		} else {
			s.Entries[i].Ended = &Ended{Outcome: OutcomeSkipped}
		}
		s.OnAir = ""
		s.PositionSeconds = 0
	default:
		return DroppedBadTransition
	}
	return ""
}

// trimTail keeps only the most recent MaxTail heard or skipped entries.
func (s *RunSheet) trimTail() {
	n := 0
	for _, e := range s.Entries {
		if e.State == StateHeard || e.State == StateSkipped {
			n++
		}
	}
	if n <= MaxTail {
		return
	}
	drop := n - MaxTail
	kept := s.Entries[:0]
	for _, e := range s.Entries {
		if drop > 0 && (e.State == StateHeard || e.State == StateSkipped) {
			drop--
			continue
		}
		kept = append(kept, e)
	}
	s.Entries = kept
}
