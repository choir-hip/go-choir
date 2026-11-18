// Package runsheet is the Autoradio run sheet: the one ordered list of
// everything a listener hears, and the reducer that applies ops to it.
//
// The wire format is JSON with snake_case keys. The Swift twin is
// mobile/AutoradioKit/Sources/AutoradioKit/RunSheet.swift (on
// origin/lane/autoradio-integ); both read the
// shared fixtures in mobile/fixtures/runsheet and must agree on every one.
//
// The reducer holds mechanical invariants only. Nothing here interprets
// meaning.
package runsheet

import (
	"encoding/json"
	"fmt"
)

// Bounds the reducer enforces (bytes, not runes).
const (
	MaxTitleBytes  = 300
	MaxTextBytes   = 8 * 1024
	MaxReasonBytes = 160
	// MaxTail is how many heard or skipped entries the sheet keeps.
	MaxTail = 50
	// MaxAppliedOps is how many applied op_ids the sheet remembers for the
	// duplicate rule.
	MaxAppliedOps = 512
	// MaxAhead is how many entries may be ahead. place is bounded by the
	// serving route (a place always applies in the reducer); previous, which
	// moves entries back to ahead, is bounded here.
	MaxAhead = 100

	// Field bounds (bytes). An entry that passes none of them is invalid.
	MaxIDBytes    = 64
	MaxRefBytes   = 512  // source.label, clip_id; cites.label
	MaxURLBytes   = 2048 // source.ref, source.url; cites.ref (real feed and enclosure URLs run long)
	MaxCites      = 12
	MaxShortBytes = 160 // forecast.caused_by, claim_id; origin
	MaxTimeBytes  = 40  // RFC3339 strings
	MaxEstSeconds = 86400
	MaxEntryJSON  = 32 * 1024 // the entry as the record carries it (see entryJSONSize)
)

// enum is a closed string vocabulary. Unmarshalling a value outside the set
// is an error, so an unknown kind, state or op never reaches the reducer.
type enum interface{ valid() bool }

func decodeEnum[T ~string](b []byte, dst *T, what string, set map[T]bool) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	if !set[T(s)] {
		return fmt.Errorf("runsheet: unknown %s %q", what, s)
	}
	*dst = T(s)
	return nil
}

type Kind string

const (
	KindOpening    Kind = "opening"
	KindTransition Kind = "transition"
	KindStory      Kind = "story"
	KindClip       Kind = "clip"
	KindTake       Kind = "take"
	KindEpisode    Kind = "episode"
	KindPage       Kind = "page"
	KindDocument   Kind = "document"
	KindMail       Kind = "mail"
	KindResult     Kind = "result"
	KindAnswer     Kind = "answer"
)

var kinds = map[Kind]bool{KindOpening: true, KindTransition: true, KindStory: true, KindClip: true, KindTake: true,
	KindEpisode: true, KindPage: true, KindDocument: true, KindMail: true, KindResult: true, KindAnswer: true}

func (k Kind) valid() bool { return kinds[k] }
func (k *Kind) UnmarshalJSON(b []byte) error {
	return decodeEnum(b, k, "kind", kinds)
}

// TextRequired reports whether entries of this kind carry words to speak.
func (k Kind) TextRequired() bool {
	switch k {
	case KindOpening, KindTransition, KindStory, KindTake, KindAnswer, KindResult:
		return true
	}
	return false
}

type By string

const (
	ByListener By = "listener"
	ByDesk     By = "desk"
	ByPhone    By = "phone"
)

var bys = map[By]bool{ByListener: true, ByDesk: true, ByPhone: true}

func (b By) valid() bool { return bys[b] }
func (b *By) UnmarshalJSON(x []byte) error {
	return decodeEnum(x, b, "placed_by", bys)
}

type State string

const (
	StateAhead   State = "ahead"
	StateOnAir   State = "on_air"
	StateHeard   State = "heard"
	StateSkipped State = "skipped"
)

var states = map[State]bool{StateAhead: true, StateOnAir: true, StateHeard: true, StateSkipped: true}

func (s State) valid() bool { return states[s] }
func (s *State) UnmarshalJSON(b []byte) error {
	return decodeEnum(b, s, "state", states)
}

type Outcome string

const (
	OutcomeFinished Outcome = "finished"
	OutcomeSkipped  Outcome = "skipped"
	OutcomeCut      Outcome = "cut"
	OutcomeLeft     Outcome = "left"
	// OutcomeFailed: the phone could not make the entry play. It is not the
	// listener's judgement and settles no forecast.
	OutcomeFailed Outcome = "failed"
)

var outcomes = map[Outcome]bool{OutcomeFinished: true, OutcomeSkipped: true, OutcomeCut: true, OutcomeLeft: true, OutcomeFailed: true}

func (o Outcome) valid() bool { return outcomes[o] }
func (o *Outcome) UnmarshalJSON(b []byte) error {
	return decodeEnum(b, o, "outcome", outcomes)
}

type Role string

const (
	RoleSubject   Role = "subject"
	RoleQuoted    Role = "quoted"
	RoleMentioned Role = "mentioned"
)

var roles = map[Role]bool{RoleSubject: true, RoleQuoted: true, RoleMentioned: true}

func (r Role) valid() bool { return roles[r] }
func (r *Role) UnmarshalJSON(b []byte) error {
	return decodeEnum(b, r, "role", roles)
}

type OpName string

const (
	OpPlace    OpName = "place"
	OpMove     OpName = "move"
	OpRemove   OpName = "remove"
	OpSkip     OpName = "skip"
	OpPrevious OpName = "previous"
	OpClear    OpName = "clear"
	OpRewrite  OpName = "rewrite"
	OpAired    OpName = "aired"
)

var opNames = map[OpName]bool{OpPlace: true, OpMove: true, OpRemove: true, OpSkip: true,
	OpPrevious: true, OpClear: true, OpRewrite: true, OpAired: true}

func (o OpName) valid() bool { return opNames[o] }
func (o *OpName) UnmarshalJSON(b []byte) error {
	return decodeEnum(b, o, "op", opNames)
}

type At string

const (
	AtNow    At = "now"
	AtNext   At = "next"
	AtQueued At = "queued"
	AtEnd    At = "end"
	AtAfter  At = "after"
)

var ats = map[At]bool{AtNow: true, AtNext: true, AtQueued: true, AtEnd: true, AtAfter: true}

func (a At) valid() bool { return ats[a] }
func (a *At) UnmarshalJSON(b []byte) error {
	return decodeEnum(b, a, "at", ats)
}

// Ended says how an entry stopped being on air. At is RFC3339 and is empty
// when the placer supplied none (the reducer reads no clock).
type Ended struct {
	Outcome   Outcome `json:"outcome"`
	At        string  `json:"at"`
	LastBlock int     `json:"last_block"`
}

type Source struct {
	Ref    string   `json:"ref"`
	Label  string   `json:"label"`
	URL    string   `json:"url,omitempty"`
	Start  *float64 `json:"start,omitempty"`
	End    *float64 `json:"end,omitempty"`
	ClipID string   `json:"clip_id,omitempty"`
}

type Cite struct {
	Ref   string `json:"ref"`
	Role  Role   `json:"role"`
	Label string `json:"label"`
}

type Between struct {
	After  string `json:"after"`
	Before string `json:"before"`
}

type Forecast struct {
	PHeard   float64 `json:"p_heard"`
	CausedBy string  `json:"caused_by"`
	ClaimID  string  `json:"claim_id"`
}

// Entry is one item on the run sheet. Unknown JSON fields are ignored.
type Entry struct {
	ID         string    `json:"id"`
	Kind       Kind      `json:"kind"`
	PlacedBy   By        `json:"placed_by"`
	State      State     `json:"state"`
	Ended      *Ended    `json:"ended,omitempty"`
	Title      string    `json:"title"`
	Text       string    `json:"text,omitempty"`
	Source     *Source   `json:"source,omitempty"`
	Cites      []Cite    `json:"cites,omitempty"`
	Between    *Between  `json:"between,omitempty"`
	EstSeconds float64   `json:"est_seconds"`
	Forecast   *Forecast `json:"forecast,omitempty"`
	// The additions of 2026-11-16 (master stream). Recorded, not interpreted:
	// the reducer neither sets nor reads them.
	AirAt    string `json:"air_at,omitempty"`
	Origin   string `json:"origin,omitempty"`
	Revision int    `json:"revision,omitempty"`
}

// RunSheet is the ordered list: heard/skipped tail (most recent 50), the entry
// on air, then the entries ahead. AppliedOps is the reducer's memory of the
// last MaxAppliedOps applied op_ids, oldest first; it is how a repeated op_id
// is recognised (see the interpretation notes in apply.go).
type RunSheet struct {
	Version         int64    `json:"version"`
	OnAir           string   `json:"on_air,omitempty"`
	PositionSeconds float64  `json:"position_seconds"`
	Entries         []Entry  `json:"entries"`
	AppliedOps      []string `json:"applied_ops,omitempty"`
	// The additions of 2026-11-16: recorded, not interpreted.
	Station       string `json:"station,omitempty"`
	MasterVersion int64  `json:"master_version,omitempty"`
	ClockAt       string `json:"clock_at,omitempty"`
	// SheetEpoch is the server's: zero until a replay had to repair the
	// history (skip, step over or deduplicate records), then the repair's time
	// in milliseconds. A client that holds another epoch must take the full
	// sheet. Recorded, not interpreted, by the reducer.
	SheetEpoch int64 `json:"sheet_epoch,omitempty"`
}

// Op is one change to a run sheet, the same vocabulary in both directions.
type Op struct {
	OpID            string   `json:"op_id"`
	By              By       `json:"by"`
	Op              OpName   `json:"op"`
	At              At       `json:"at,omitempty"`
	After           string   `json:"after,omitempty"`
	ID              string   `json:"id,omitempty"`
	Entry           *Entry   `json:"entry,omitempty"`
	Text            string   `json:"text,omitempty"`
	State           State    `json:"state,omitempty"`
	Ended           *Ended   `json:"ended,omitempty"`
	PositionSeconds *float64 `json:"position_seconds,omitempty"`
	BaseVersion     int64    `json:"base_version"`
	Reason          string   `json:"reason,omitempty"`
	// Origin is the 2026-11-16 addition: recorded, not interpreted here.
	Origin string `json:"origin,omitempty"`
}

// Result is what the reducer says about one op. Dropped is empty when the op
// applied. Version is the sheet's version after the op.
type Result struct {
	OpID    string `json:"op_id"`
	Applied bool   `json:"applied"`
	Dropped string `json:"dropped,omitempty"`
	Version int64  `json:"version"`
}

// Dropped reasons.
const (
	DroppedDuplicate     = "duplicate"
	DroppedNotAhead      = "not_ahead"
	DroppedNotOnAir      = "not_on_air"
	DroppedNotPhone      = "not_phone"
	DroppedBadTransition = "bad_transition"
	DroppedOtherOnAir    = "other_on_air"
	DroppedUnknownID     = "unknown_id"
	DroppedStaleVersion  = "stale_version"
	DroppedNoPrevious    = "no_previous"
	DroppedDuplicateID   = "duplicate_id"
	DroppedReasonNeeded  = "reason_required"
	DroppedInvalid       = "invalid"
	DroppedAheadFull     = "ahead_full"
	// DroppedReplaySkipped is what the server answers for an op whose tape
	// record a replay could not fold (it did not decode, the reducer drops it
	// today, or another record took its version). Never produced by Apply.
	DroppedReplaySkipped = "replay_skipped"
)

// ValidID reports whether s matches [A-Za-z0-9][A-Za-z0-9_.-]{0,63}.
func ValidID(s string) bool {
	if len(s) == 0 || len(s) > 64 {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		alnum := c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9'
		if alnum || (i > 0 && (c == '_' || c == '.' || c == '-')) {
			continue
		}
		return false
	}
	return true
}
