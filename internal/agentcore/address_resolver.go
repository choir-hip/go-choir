package agentcore

import (
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/deskmanifest"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// The address resolver (modular architecture MA1.2, R5): one data-driven
// function from a request's sender and the recipient name it staged to the
// inbox the request lands in. It replaces recordsRoute's per-sender switch,
// recordsDesk, the manifest's MessageTargets and Addresses allowlists, and
// agentprofile.CanMessage.
//
// Any desk may send a request to any registered desk. Where the request
// lands comes from the recipient's declared instance (deskmanifest.Instance:
// per owner, per subject, per assignment). Authority never travels with a
// request: the recipient acts on its own grants. The one place that shows is
// an engineering request from a desk that does not hold the assignment kind;
// an assignment opens only at its holder's admission, so the request lands in
// the holder's inbox as an admission request (Forwarded).
//
// The rules no manifest states are registered inbox classes (below) and the
// management task's outward routes.

// Route is a resolved address.
type Route struct {
	// Inbox is the inbox the request lands in; empty when OnRoute is false.
	Inbox string
	// OnRoute is false for an address that is not on the records path (a
	// desk with no records inbox, an address the sender cannot name): the
	// act stays on its packet path, as before.
	OnRoute bool
	// Forwarded is true when the request was aimed at a desk whose inbox
	// only a holder of the desk's admission may write, and landed in the
	// holder's inbox instead (an engineering Cast from a non-holder).
	Forwarded bool
}

// InboxClass is a registered inbox that is not a desk: an address the
// service that reads or writes it owns.
type InboxClass struct {
	// Name is the class: the prefix of <name>:<subject>, or the whole of a
	// fixed address.
	Name string
	// Fixed marks a class that is one address (texture:new), not a prefix.
	Fixed bool
	// Addressable is false for an inbox only the system writes (job:<id>):
	// a desk's request to it is refused.
	Addressable bool
	// Senders are the desks (and the management task's own desk name) whose
	// requests the class accepts, the owning service's own admission rule;
	// empty means any desk.
	Senders []string
}

var (
	inboxClassMu sync.RWMutex
	inboxClasses = map[string]InboxClass{}
)

// RegisterInboxClass registers a non-desk inbox class. It panics on a
// duplicate, a name that is a desk, or an empty name: classes are registered
// at init by the service that owns the inbox.
func RegisterInboxClass(c InboxClass) {
	inboxClassMu.Lock()
	defer inboxClassMu.Unlock()
	if strings.TrimSpace(c.Name) == "" {
		panic("agentcore: inbox class with no name")
	}
	if _, dup := inboxClasses[c.Name]; dup {
		panic("agentcore: duplicate inbox class " + c.Name)
	}
	if _, isDesk := deskmanifest.Lookup(c.Name); isDesk {
		panic("agentcore: inbox class " + c.Name + " is a desk")
	}
	c.Senders = append([]string(nil), c.Senders...)
	inboxClasses[c.Name] = c
}

// inboxClassOf finds the class an address belongs to: a fixed address by its
// whole, a prefixed one by its name part.
func inboxClassOf(to, toName string) (InboxClass, bool) {
	inboxClassMu.RLock()
	defer inboxClassMu.RUnlock()
	if c, ok := inboxClasses[to]; ok && c.Fixed {
		return c, true
	}
	if c, ok := inboxClasses[toName]; ok && !c.Fixed {
		return c, true
	}
	return InboxClass{}, false
}

func (c InboxClass) accepts(fromDesk string) bool {
	if len(c.Senders) == 0 {
		return true
	}
	for _, s := range c.Senders {
		if types.IsManagementTaskDesk(s) && types.IsManagementTaskDesk(fromDesk) {
			return true
		}
		if s == fromDesk {
			return true
		}
	}
	return false
}

// InboxClasses lists the registered classes, sorted by name.
func InboxClasses() []InboxClass {
	inboxClassMu.RLock()
	defer inboxClassMu.RUnlock()
	out := make([]InboxClass, 0, len(inboxClasses))
	for _, c := range inboxClasses {
		c.Senders = append([]string(nil), c.Senders...)
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// captureSourceDesk names a source's inbox (source:<source_id>): the
// sender of its capture arrivals and the addressee of management's
// declines (Cut 2 2b).
const captureSourceDesk = "source"

// jobInboxClass names a capsule job's inbox (job:<id>), the sender of its
// end record (jobs.go).
const jobInboxClass = "job"

func init() {
	// The open-subject address: a Cast to it opens a story, and the store
	// rewrites it to the new story's texture:<doc> at commit. Management
	// (and its tasks) open stories; the store refuses any other sender.
	RegisterInboxClass(InboxClass{Name: types.StoryOpenInbox, Fixed: true, Addressable: true,
		Senders: []string{agentprofile.Management, types.ManagementTaskDesk}})
	// A source's inbox takes management's decline of its captures, from
	// management or from a task sending as management (Cut 2 2b); the
	// source writes its capture arrivals to management the other way, as a
	// system record.
	RegisterInboxClass(InboxClass{Name: captureSourceDesk, Addressable: true,
		Senders: []string{agentprofile.Management, types.ManagementTaskDesk}})
	// A job's inbox is the sender of its end record; no desk writes to it.
	RegisterInboxClass(InboxClass{Name: jobInboxClass, Addressable: false})
}

// assignmentKind is the owned kind whose holder admits engineering work.
const assignmentKind = "assignment"

// assignmentHolder is the system desk that owns the assignment kind.
func assignmentHolder() (deskmanifest.Desk, bool) {
	for _, name := range deskmanifest.Names() {
		d, ok := deskmanifest.Lookup(name)
		if !ok || d.Kind != deskmanifest.KindSystem {
			continue
		}
		for _, k := range d.OwnedKinds {
			if k == assignmentKind {
				return d, true
			}
		}
	}
	return deskmanifest.Desk{}, false
}

// ResolveAddress is the inbox a request lands in. Every address but the empty
// one either resolves or is refused; there is no silent off-route.
// fromAgentID is the
// sender's agent id (<desk>:<subject>), ownerID the computer's owner, toDesk
// the recipient name or inbox the cell staged. An error is a refusal the
// sender should read. The empty address is the only one that is not on the
// records path and not refused: a ledger-only act names no recipient.
func ResolveAddress(fromAgentID, ownerID, toDesk string) (Route, error) {
	fromDesk, fromSubject, _ := strings.Cut(strings.TrimSpace(fromAgentID), ":")
	to := strings.TrimSpace(toDesk)
	toName, toSubject, hasSubject := strings.Cut(to, ":")
	toSubject = strings.TrimSpace(toSubject)
	if canonical, err := agentprofile.Canonical(toName); err == nil {
		toName = canonical
	}
	named := hasSubject && toSubject != ""
	owner := strings.TrimSpace(ownerID)
	isTask := types.IsManagementTaskDesk(fromDesk)

	if isTask && fromSubject == "" {
		return Route{}, fmt.Errorf("a management task's agent id %q names no task", fromAgentID)
	}
	if to == "" && !isTask {
		return Route{}, nil
	}

	// Registered inbox classes (non-desk inboxes).
	if class, ok := inboxClassOf(to, toName); ok {
		if !class.Addressable {
			return Route{}, fmt.Errorf("%q is a system inbox the system writes; a desk does not address it", to)
		}
		if class.accepts(fromDesk) {
			if class.Fixed {
				return Route{Inbox: class.Name, OnRoute: true}, nil
			}
			if named {
				return Route{Inbox: class.Name + ":" + toSubject, OnRoute: true}, nil
			}
			if !isTask {
				return Route{}, fmt.Errorf("%q names no %s; address %s:<id>", to, class.Name, class.Name)
			}
		}
		if !isTask {
			return Route{}, fmt.Errorf("%q does not take requests from %s (it names its senders: %s)", to, fromDesk, strings.Join(class.Senders, ", "))
		}
	}

	// A management task speaks outward as management, to a story or a
	// source only; its sender is set by the reducer. Engineering, management
	// and the computer are the parent's.
	if isTask {
		if toName == agentprofile.Texture {
			if !named {
				return Route{}, fmt.Errorf("a task addresses a story by its inbox (texture:<doc>), or opens one at texture:new")
			}
			return Route{Inbox: agentprofile.Texture + ":" + toSubject, OnRoute: true}, nil
		}
		return Route{}, fmt.Errorf("a management task sends only to stories (texture:<doc>, texture:new) and to sources; %q is management's to address", to)
	}

	// The recipient desk, from the registry.
	recipient, ok := deskmanifest.Lookup(to)
	if !ok {
		recipient, ok = deskmanifest.Lookup(toName)
	}
	if !ok {
		return Route{}, fmt.Errorf("%q is not a registered desk or inbox", to)
	}
	if recipient.Instance == deskmanifest.InstanceNone {
		return Route{}, fmt.Errorf("%q has no records inbox", to)
	}
	sender, ok := deskmanifest.Lookup(fromDesk)
	if !ok || sender.Instance == deskmanifest.InstanceNone {
		return Route{}, fmt.Errorf("%q has no records inbox to send from", fromAgentID)
	}
	if (sender.Instance == deskmanifest.InstanceSubject || sender.Instance == deskmanifest.InstanceAssignment) && fromSubject == "" {
		return Route{}, fmt.Errorf("%q names no %s instance to send from", fromAgentID, fromDesk)
	}
	if fromDesk == recipient.Name {
		return Route{}, fmt.Errorf("%s does not address another %s; a desk reaches the other desks", fromDesk, recipient.Name)
	}

	// An assignment opens only at the holder's admission: a request to the
	// assignment desk from a desk that does not hold the kind lands in the
	// holder's inbox as an admission request. Authority never travels.
	if recipient.Instance == deskmanifest.InstanceAssignment && !holdsAssignments(sender) {
		holder, ok := assignmentHolder()
		if !ok || holder.Instance != deskmanifest.InstanceOwner || owner == "" {
			return Route{}, fmt.Errorf("no %s inbox to forward %q to", holder.Name, to)
		}
		return Route{Inbox: holder.Name + ":" + owner, OnRoute: true, Forwarded: true}, nil
	}

	switch recipient.Instance {
	case deskmanifest.InstanceOwner:
		if owner == "" || (hasSubject && toSubject != owner) {
			return Route{}, fmt.Errorf("%q is not this computer's %s inbox (%s:<owner>)", to, recipient.Name, recipient.Name)
		}
		return Route{Inbox: recipient.Name + ":" + owner, OnRoute: true}, nil
	case deskmanifest.InstanceSubject:
		if sender.Instance == deskmanifest.InstanceSubject {
			// A subject desk talks to its subject's other desks.
			if hasSubject && toSubject != fromSubject {
				return Route{}, fmt.Errorf("%s:%s reaches only its own subject's %s, not %q", sender.Name, fromSubject, recipient.Name, to)
			}
			return Route{Inbox: recipient.Name + ":" + fromSubject, OnRoute: true}, nil
		}
		if !named {
			if sender.Instance == deskmanifest.InstanceOwner {
				return Route{}, fmt.Errorf("%s addresses a %s by its inbox: pass the record's from (%s:<doc>), not %q", sender.Name, recipient.Name, recipient.Name, to)
			}
			return Route{}, fmt.Errorf("%s names the %s inbox it means (%s:<doc>), not %q", sender.Name, recipient.Name, recipient.Name, to)
		}
		return Route{Inbox: recipient.Name + ":" + toSubject, OnRoute: true}, nil
	case deskmanifest.InstanceAssignment:
		// Only the holder reaches here, and it names the assignment it
		// opened; a Cast to the bare name opens a new one and is planned by
		// planEngineeringCast before the resolver runs.
		if !named {
			return Route{}, fmt.Errorf("%s speaks into an open assignment by its inbox (%s:<assignment>, the record's from); a Cast to %q opens a new one", sender.Name, recipient.Name, recipient.Name)
		}
		return Route{Inbox: recipient.Name + ":" + toSubject, OnRoute: true}, nil
	}
	return Route{}, fmt.Errorf("desk %s declares the unknown instance %q", recipient.Name, recipient.Instance)
}

// markForwarded says in the request's own text that it was aimed at another
// desk and forwarded to the assignment holder.
func markForwarded(in yaegikernel.StagedIntent, sender string) yaegikernel.StagedIntent {
	prefix := "Admission request, a " + string(in.Kind) + " aimed at " + strings.TrimSpace(in.ToDesk) + " by " + sender + ": "
	switch in.Kind {
	case yaegikernel.IntentCast:
		in.Objective = prefix + in.Objective
	case yaegikernel.IntentReply:
		in.Answer = prefix + in.Answer
	case yaegikernel.IntentReport:
		in.Claim = prefix + in.Claim
	default:
		in.Body = prefix + in.Body
	}
	return in
}

func holdsAssignments(d deskmanifest.Desk) bool {
	if d.Kind != deskmanifest.KindSystem {
		return false
	}
	for _, k := range d.OwnedKinds {
		if k == assignmentKind {
			return true
		}
	}
	return false
}

// isRecordsDesk reports whether a desk's conversations run on records: it
// declares a records inbox (an instance).
func isRecordsDesk(profile string) bool {
	d, ok := deskmanifest.Lookup(strings.TrimSpace(profile))
	return ok && d.Instance != deskmanifest.InstanceNone
}

// canonicalDeskName is the canonical spelling of a desk name, or the name
// unchanged when it is not a profile (the caller then compares unequal).
func canonicalDeskName(name string) string {
	if c, err := agentprofile.Canonical(name); err == nil {
		return c
	}
	return strings.TrimSpace(name)
}
