// Package disclosure is the kernel's disclosure rules (modular architecture
// plan MA3.6, R15, section 7.2 "Disclosure path").
//
// A run carries the union of the labels of everything it read (MA3.5). When
// it tries a SINK, a place where its data leaves its custody, the decision
// runs in order: this table first (most decisions end here), then System 1's
// typed question for an "ask", then the custodian's model or an owner tap.
// This package is the first step and the arithmetic of the second: a pure,
// deterministic table of label class by sink to allow, deny or ask, and the
// rule that reads a System 1 distribution. It imports only internal/types,
// so the table is data a test can pin cell by cell.
//
// Sinks (the inventory, with the code path that reaches each, is in
// docs/definitions/choir-modular-architecture-2026-11-04.ledger.md):
//
//	publication      a story or Texture published to the Autopaper graph
//	mail_send        an email leaving the owner's mailbox
//	egress           a request to a destination the run composed (a URL, a
//	                 provider endpoint, a search query)
//	desk_message     a Cast, Report, Ask, Feed or Escalate to a desk that is
//	                 not a custodian admitted for a label the run carries
//	library_release  a package release published to the Library
//
// A model call is NOT a sink (owner ruling R15): the provider is chosen by
// the user's VM policy, logged with the run's labels, and never constrained
// by a rule.
//
// The defaults are chosen so the paper keeps publishing. The Autopaper
// publishes world content around the clock, so world_untrusted, platform and
// publisher labels allow every sink the product uses. Only an owner_private
// label restricts: its data may go to its own custodian (or a desk a live
// lease admits) freely, and to anywhere else only on an ask.
package disclosure

import (
	"fmt"
	"log"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// Sink is a place where a run's data leaves its custody.
type Sink string

const (
	SinkPublication    Sink = "publication"
	SinkMailSend       Sink = "mail_send"
	SinkEgress         Sink = "egress"
	SinkDeskMessage    Sink = "desk_message"
	SinkLibraryRelease Sink = "library_release"
)

// Sinks lists every sink, in table order.
var Sinks = []Sink{SinkPublication, SinkMailSend, SinkEgress, SinkDeskMessage, SinkLibraryRelease}

// Decision is what the table says about one label at one sink.
type Decision string

const (
	Allow Decision = "allow"
	Deny  Decision = "deny"
	Ask   Decision = "ask"
)

// severity orders the decisions; a set of labels decides by its strictest.
func (d Decision) severity() int {
	switch d {
	case Deny:
		return 2
	case Ask:
		return 1
	}
	return 0
}

// Class is a label class of the lattice (types.labels): the atom's kind, not
// its suffix.
type Class string

const (
	ClassOwnerPrivate Class = "owner_private"
	ClassWorldUntrust Class = "world_untrusted"
	ClassPlatform     Class = "platform"
	ClassPublisher    Class = "publisher"
	ClassUnknownAtom  Class = "unknown"
)

// Classes lists the label classes the table covers, in table order.
var Classes = []Class{ClassOwnerPrivate, ClassWorldUntrust, ClassPlatform, ClassPublisher}

// ClassOf is the class of a label atom.
func ClassOf(label string) Class {
	switch {
	case label == types.LabelWorldUntrusted:
		return ClassWorldUntrust
	case label == types.LabelPlatform:
		return ClassPlatform
	case strings.HasPrefix(label, types.LabelOwnerPrivatePrefix):
		return ClassOwnerPrivate
	case strings.HasPrefix(label, types.LabelPublisherPrefix):
		return ClassPublisher
	}
	return ClassUnknownAtom
}

// Cell is one cell of the table: the decision when the destination is not
// admitted for the label, and when it is. Admission is a fact the host
// computes, never the cell's claim: for a desk message, the target is the
// custodian of the label's kind or holds a live lease for it; for a mail
// send, the mail custodian is replying in the owner's own thread. For a
// sink with no notion of admission both decisions are the same.
type Cell struct {
	Unadmitted Decision
	Admitted   Decision
}

func same(d Decision) Cell { return Cell{Unadmitted: d, Admitted: d} }

// Table is the rules table: label class by sink. It is data, one place, and
// TestTableCellsPinned pins every cell. Changing a cell is a deliberate edit
// of this file and that test.
var Table = map[Class]map[Sink]Cell{
	// An owner-private label restricts: the owner's data goes to its custodian
	// (or to a desk a live lease admits) without asking, and anywhere else on
	// an ask. Nothing is denied outright by default: a deny is reserved for
	// the owner's own future rules, and Decide supports it.
	ClassOwnerPrivate: {
		SinkPublication:    same(Ask),
		SinkMailSend:       {Unadmitted: Ask, Admitted: Allow},
		SinkEgress:         same(Ask),
		SinkDeskMessage:    {Unadmitted: Ask, Admitted: Allow},
		SinkLibraryRelease: same(Ask),
	},
	// World content, the platform's own data and a publisher's content are
	// not the owner's to restrict: every sink the product uses allows them.
	ClassWorldUntrust: allowAll(),
	ClassPlatform:     allowAll(),
	ClassPublisher:    allowAll(),
}

func allowAll() map[Sink]Cell {
	m := make(map[Sink]Cell, len(Sinks))
	for _, s := range Sinks {
		m[s] = same(Allow)
	}
	return m
}

// CellFor returns the table's cell for a class and sink.
func CellFor(c Class, s Sink) (Cell, bool) {
	row, ok := Table[c]
	if !ok {
		return Cell{}, false
	}
	cell, ok := row[s]
	return cell, ok
}

// Facts are what the host knows about one sink attempt, computed from the run
// record and the registry, never from the cell.
type Facts struct {
	// AdmittedFor reports whether the destination is admitted for a label.
	// nil admits nothing.
	AdmittedFor func(label string) bool
}

// Hit is the table's decision for one label.
type Hit struct {
	Label    string   `json:"label"`
	Class    Class    `json:"class"`
	Admitted bool     `json:"admitted"`
	Decision Decision `json:"decision"`
}

// Verdict is the table's decision for a run's whole label set at a sink: the
// strictest of its labels' cells, with every cell that spoke.
type Verdict struct {
	Sink     Sink     `json:"sink"`
	Decision Decision `json:"decision"`
	Hits     []Hit    `json:"hits"`
}

// Binding is the label class that decided, for the counters: the strictest
// hit's class ("owner_private" when an owner-private label asked), and "-"
// when every label allowed the sink (a world-only run, or none).
func (v Verdict) Binding() string {
	if v.Decision == Allow {
		return "-"
	}
	best := -1
	class := ""
	for _, h := range v.Hits {
		if sev := h.Decision.severity(); sev > best {
			best, class = sev, string(h.Class)
		}
	}
	return class
}

// Reason is a sentence the refusal and the owner's view carry.
func (v Verdict) Reason() string {
	var parts []string
	for _, h := range v.Hits {
		if h.Decision == Allow {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s (%s) at %s is %s", h.Label, h.Class, v.Sink, h.Decision))
	}
	if len(parts) == 0 {
		return fmt.Sprintf("every label the run carries allows %s", v.Sink)
	}
	return strings.Join(parts, "; ")
}

// Decide runs the table over the run's labels at a sink. A run that carries no
// label has read nothing restricted and is allowed. An atom outside the
// lattice cannot be in a LabelSet (NewLabelSet drops it); a label of an
// unknown class is treated as owner-private, the safe reading.
func Decide(labels types.LabelSet, sink Sink, facts Facts) Verdict {
	v := Verdict{Sink: sink, Decision: Allow}
	for _, l := range labels {
		class := ClassOf(l)
		lookup := class
		if class == ClassUnknownAtom {
			lookup = ClassOwnerPrivate
		}
		cell, ok := CellFor(lookup, sink)
		if !ok {
			cell = same(Ask)
		}
		admitted := facts.AdmittedFor != nil && facts.AdmittedFor(l)
		d := cell.Unadmitted
		if admitted {
			d = cell.Admitted
		}
		v.Hits = append(v.Hits, Hit{Label: l, Class: class, Admitted: admitted, Decision: d})
		if d.severity() > v.Decision.severity() {
			v.Decision = d
		}
	}
	return v
}

// The ask rule. System 1 returns a distribution over yes, no and unsure to
// "is this disclosure within what the owner's request implies"; the rule, not
// the judge, decides from it. It allows only when the judge is confident in
// yes and nearly rules out no; any other distribution, a missing one and a
// failed call all escalate to the owner.
const (
	// QuestionID is the id of the disclosure question in a System 1 call.
	QuestionID = "disclosure"
	// MinYes is the least P(yes) the rule allows on.
	MinYes = 0.8
	// MaxNo is the most P(no) the rule allows on.
	MaxNo = 0.1
)

// Choices are the closed choices of the question.
var Choices = []string{"yes", "no", "unsure"}

// Question is the wording of the typed question. The evidence (the owner's
// request and the disclosure) rides in the host-composed context.
const Question = "Is this disclosure within what the owner's request implies? yes: the request asks for it or clearly implies it; no: it sends owner-private data somewhere the request does not call for; unsure: the request does not settle it."

// Outcome is what the rule decides from a distribution.
type Outcome string

const (
	// OutcomeAllow: the distribution clears the rule.
	OutcomeAllow Outcome = "allow"
	// OutcomeEscalate: the owner decides.
	OutcomeEscalate Outcome = "escalate"
)

// FromDistribution applies the rule. dist is the judge's probabilities by
// choice, as returned; nil or an empty map escalates.
func FromDistribution(dist map[string]float64) Outcome {
	yes, okY := dist["yes"]
	no := dist["no"]
	if !okY {
		return OutcomeEscalate
	}
	if yes >= MinYes && no <= MaxNo {
		return OutcomeAllow
	}
	return OutcomeEscalate
}

// Gate mode. Mirrors the writer and reader gates (objectgraph.WriterGateMode):
// a built-in desk's decision is recorded and counted, and a deny or an ask
// does not block yet (MA3.7 flips it, in code). An installed package desk is
// enforced now. It is not a setting.
type Mode int32

const (
	// ModeObserve counts a built-in desk's deny or ask and lets it pass.
	ModeObserve Mode = iota
	// ModeEnforce refuses a built-in desk's deny and parks its ask too.
	ModeEnforce
)

func (m Mode) String() string {
	if m == ModeEnforce {
		return "enforce"
	}
	return "observe"
}

var gateMode atomic.Int32

// CurrentMode is the mode in force for built-in desks (read by the
// observability route).
func CurrentMode() Mode { return Mode(gateMode.Load()) }

// SetMode sets the mode and returns the function that restores the previous
// one. The test injection seam: production runs the default.
func SetMode(m Mode) (restore func()) {
	previous := gateMode.Swap(int32(m))
	return func() { gateMode.Store(previous) }
}

var (
	countMu sync.Mutex
	counts  = map[string]uint64{}
	logged  = map[string]bool{}
)

// Note counts one decision under "sink|class|decision|outcome". Outcome is
// "allowed" (the table allowed), "observed" (a built-in desk's deny or ask,
// not blocked), "enforced" (a package desk's, or a built-in's in enforce
// mode). A deny or ask logs once per key.
func Note(v Verdict, outcome string) {
	key := fmt.Sprintf("%s|%s|%s|%s", v.Sink, v.Binding(), v.Decision, outcome)
	countMu.Lock()
	counts[key]++
	first := !logged[key]
	logged[key] = true
	countMu.Unlock()
	if first && v.Decision != Allow {
		log.Printf("disclosure gate: %s (%s): %s", key, outcome, v.Reason())
	}
}

// Counts returns the decisions counted, keyed "sink|class|decision|outcome".
func Counts() map[string]uint64 {
	countMu.Lock()
	defer countMu.Unlock()
	out := make(map[string]uint64, len(counts))
	for k, v := range counts {
		out[k] = v
	}
	return out
}

// ResetCounts clears the counters (tests).
func ResetCounts() {
	countMu.Lock()
	counts = map[string]uint64{}
	logged = map[string]bool{}
	countMu.Unlock()
}
