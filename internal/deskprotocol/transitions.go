package deskprotocol

import "sort"

// Machine names one lifecycle the desks share.
type Machine string

const (
	// MachineWorkItem is types.WorkItemStatus.
	MachineWorkItem Machine = "work_item"
	// MachineTrajectory is types.TrajectoryStatus.
	MachineTrajectory Machine = "trajectory"
	// MachineEngineeringAssignment is types.EngineeringAssignmentDisposition.
	MachineEngineeringAssignment Machine = "engineering_assignment"
	// MachineEngineeringCapsule is types.EngineeringCapsuleDisposition.
	MachineEngineeringCapsule Machine = "engineering_capsule"
	// MachineCommitment is the answered-by relation of commitment record
	// kinds: From is the kind of the record answered, To the kind of the
	// record that answers it.
	MachineCommitment Machine = "commitment_record"
)

// Transition is one allowed move of a machine.
type Transition struct {
	Machine Machine
	From    string
	To      string
}

// table is the explicit form of the lifecycle rules the store enforced as
// scattered checks (docs/design/app-agent-desks-2026-11-04.md 5.3). The state
// names are the values of the types constants; a store test pins that.
//
// A row with an empty From is creation: the state an object is first written
// in. It is a row so that the table names every state the store assigns.
var table = []Transition{
	// A work item is created open, and stays open until it ends; an ended
	// work item never moves.
	{MachineWorkItem, "", "open"},
	{MachineWorkItem, "open", "completed"},
	{MachineWorkItem, "open", "cancelled"},
	{MachineWorkItem, "open", "refused"},

	// A trajectory is created live; it is live until it settles or is cancelled.
	{MachineTrajectory, "", "live"},
	{MachineTrajectory, "live", "settled"},
	{MachineTrajectory, "live", "cancelled"},

	// An engineering assignment opens, is bound to a run, and ends. A report
	// on an assignment that is not bound is refused, so only bound ends in
	// completed or failed; either open or bound may be cancelled.
	{MachineEngineeringAssignment, "", "open"},
	{MachineEngineeringAssignment, "open", "bound"},
	{MachineEngineeringAssignment, "open", "cancelled"},
	{MachineEngineeringAssignment, "bound", "completed"},
	{MachineEngineeringAssignment, "bound", "failed"},
	{MachineEngineeringAssignment, "bound", "cancelled"},

	// The capsule disposition of an assignment: freeze and revoke are
	// requested, then acknowledged. The intent and acknowledgement refs a
	// move also needs stay with the store's check.
	{MachineEngineeringCapsule, "", "unbound"},
	// Binding the assignment to a run activates its capsule. This move is
	// made by the bind command, not by the disposition command, which keeps
	// refusing "active" as a destination.
	{MachineEngineeringCapsule, "unbound", "active"},
	{MachineEngineeringCapsule, "active", "freeze_requested"},
	{MachineEngineeringCapsule, "freeze_requested", "frozen"},
	{MachineEngineeringCapsule, "unbound", "revoke_requested"},
	{MachineEngineeringCapsule, "active", "revoke_requested"},
	{MachineEngineeringCapsule, "freeze_requested", "revoke_requested"},
	{MachineEngineeringCapsule, "frozen", "revoke_requested"},
	{MachineEngineeringCapsule, "revoke_requested", "revoked"},

	// Which record kind may answer which. Recorded for the lock; no store
	// check reads these rows yet (see Enforced).
	{MachineCommitment, "precommit", "resolve"},
	// retract is not a row while the machine is unenforced: it is a directive
	// subtype, not a record kind, so it needs a kind:subtype form before a check
	// reads this machine.
	{MachineCommitment, "directive", "report"},
	{MachineCommitment, "resolve", "disagreement"},
}

// enforced names the machines a runtime check reads this table for. The
// others are locked but not yet enforced from here.
var enforced = map[Machine]bool{
	MachineWorkItem:           true,
	MachineTrajectory:         true,
	MachineEngineeringCapsule: true,
}

// Enforced reports whether the store reads the machine's rows for a check.
func Enforced(m Machine) bool { return enforced[m] }

// Transitions returns the whole table, sorted by machine, from, to.
func Transitions() []Transition {
	out := append([]Transition{}, table...)
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Machine != b.Machine {
			return a.Machine < b.Machine
		}
		if a.From != b.From {
			return a.From < b.From
		}
		return a.To < b.To
	})
	return out
}

// Allowed reports whether the machine may move from one state to another.
func Allowed(m Machine, from, to string) bool {
	for _, t := range table {
		if t.Machine == m && t.From == from && t.To == to {
			return true
		}
	}
	return false
}

// Terminal reports whether state is a state of the machine from which no
// move leaves. A state the machine does not name is not terminal, so an
// unknown value behaves as it did when the checks were inline.
func Terminal(m Machine, state string) bool {
	named := false
	for _, t := range table {
		if t.Machine != m {
			continue
		}
		if t.From == state {
			return false
		}
		if t.To == state {
			named = true
		}
	}
	return named
}

// Machines lists the machines the table has rows for.
func Machines() []Machine {
	seen := map[Machine]bool{}
	var out []Machine
	for _, t := range table {
		if !seen[t.Machine] {
			seen[t.Machine] = true
			out = append(out, t.Machine)
		}
	}
	return out
}
