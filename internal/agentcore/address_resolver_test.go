package agentcore

import (
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/deskmanifest"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// The contract of the address resolver (MA1.2). The resolver replaced
// recordsRoute; before recordsRoute was deleted a differential harness ran
// both on 9996 inputs (every sender form by every recipient form, both
// owner states) and found no difference except the intended ones, listed in
// the commit that deleted it. The cases below pin the old routes and the
// intended changes.

const contractOwner = "owner-1"

func contractRoute(t *testing.T, from, owner, to string) Route {
	t.Helper()
	r, err := ResolveAddress(from, owner, to)
	if err != nil {
		t.Fatalf("ResolveAddress(%q, %q, %q): %v", from, owner, to, err)
	}
	return r
}

// Every system desk to every system desk resolves to the recipient's
// registered inbox, by the recipient's declared instance.
func TestResolveAddressEveryDeskToEveryDesk(t *testing.T) {
	desks := []string{agentprofile.Texture, agentprofile.Research, agentprofile.Management, agentprofile.Engineering}
	for _, from := range desks {
		for _, to := range desks {
			if from == to {
				if r, err := ResolveAddress(from+":x", contractOwner, to); err == nil {
					t.Errorf("%s addresses itself: %+v", from, r)
				}
				continue
			}
			recipient, _ := deskmanifest.Lookup(to)
			// A sender names the subject for a recipient it cannot derive one for.
			for _, name := range []string{to, to + ":subj"} {
				r, err := ResolveAddress(from+":subj", contractOwner, name)
				var want string
				switch {
				case to == agentprofile.Engineering && from != agentprofile.Management:
					want = agentprofile.Management + ":" + contractOwner
					if !r.Forwarded {
						t.Errorf("%s -> %s: not forwarded", from, name)
					}
				case recipient.Instance == deskmanifest.InstanceOwner:
					want = to + ":" + contractOwner
					if name != to {
						continue // a subject other than the owner is not the owner's inbox
					}
				default:
					if err != nil || !r.OnRoute {
						// A bare name the sender cannot resolve to a subject (management or
						// engineering to a Texture or research) must be named.
						if name == to && (from == agentprofile.Management || from == agentprofile.Engineering) {
							continue
						}
						t.Errorf("%s -> %s: %+v, %v", from, name, r, err)
					}
					want = to + ":subj"
				}
				if err != nil || !r.OnRoute || r.Inbox != want {
					t.Errorf("%s -> %s: got %+v, %v; want inbox %q", from, name, r, err, want)
				}
			}
		}
	}
}

// The routes recordsRoute had, unchanged.
func TestResolveAddressKeepsTheRoutesRecordsRouteHad(t *testing.T) {
	for _, c := range []struct{ from, to, want string }{
		{"texture:d1", "research", "research:d1"},
		{"texture:d1", "research:d1", "research:d1"},
		{"texture:d1", "management", "management:" + contractOwner},
		{"texture:d1", "management:" + contractOwner, "management:" + contractOwner},
		{"research:d1", "texture", "texture:d1"},
		{"research:d1", "texture:d1", "texture:d1"},
		{"management:" + contractOwner, "texture:d2", "texture:d2"},
		{"management:" + contractOwner, "engineering:a1", "engineering:a1"},
		{"engineering:a1", "management", "management:" + contractOwner},
		{"engineering:a1", "management:" + contractOwner, "management:" + contractOwner},
		// Non-manifest rules: the open-subject address, a source both ways,
		// and a management task's outward routes.
		{"management:" + contractOwner, types.StoryOpenInbox, types.StoryOpenInbox},
		{"management:" + contractOwner, "source:s1", "source:s1"},
		{"management-task:t1", "source:s1", "source:s1"},
		{"management-task:t1", "texture:d3", "texture:d3"},
		{"management-task:t1", types.StoryOpenInbox, types.StoryOpenInbox},
	} {
		r := contractRoute(t, c.from, contractOwner, c.to)
		if !r.OnRoute || r.Inbox != c.want || r.Forwarded {
			t.Errorf("%s -> %s: got %+v, want %q", c.from, c.to, r, c.want)
		}
	}
	// Scoping rules recordsRoute encoded: a subject desk reaches only its own
	// subject, the owner inbox only the owner's, a Texture needs a subject.
	for _, c := range []struct{ from, owner, to string }{
		{"texture:d1", contractOwner, "research:d2"},
		{"texture:d1", contractOwner, "management:other-owner"},
		{"texture:d1", "", "management"},
		{"research:d1", contractOwner, "texture:d2"},
		{"texture", contractOwner, "research"},
		{"engineering", contractOwner, "management"},
		{"texture:d1", contractOwner, "management:"},
		{"conductor:x", contractOwner, "texture:d1"},
		{"management-task", contractOwner, "texture:d1"},
	} {
		if r, err := ResolveAddress(c.from, c.owner, c.to); err == nil {
			t.Errorf("%s -> %s (owner %q): resolved to %+v, want a refusal", c.from, c.to, c.owner, r)
		}
	}
}

func TestResolveAddressRefusals(t *testing.T) {
	for _, c := range []struct{ from, to, why string }{
		{"texture:d1", "no-such-desk", "an unregistered name"},
		{"texture:d1", "no-such-desk:d1", "an unregistered name with a subject"},
		{"research:d1", "job:j1", "a system inbox only the system writes"},
		{"management:" + contractOwner, "management-task:t1", "a task role is reached by Delegate"},
		{"management:" + contractOwner, "texture", "management must name the Texture inbox"},
		{"management:" + contractOwner, "research", "management must name the research inbox"},
		{"management:" + contractOwner, "engineering", "a bare Cast opens an assignment elsewhere"},
		{"management-task:t1", "texture", "a task names its story"},
		{"management-task:t1", "engineering:a1", "a task sends only to stories and sources"},
		{"management-task:t1", "management", "a task sends only to stories and sources"},
		{"management-task:t1", "job:j1", "a task sends only to stories and sources"},
	} {
		if r, err := ResolveAddress(c.from, contractOwner, c.to); err == nil {
			t.Errorf("%s -> %s (%s): resolved to %+v, want a refusal", c.from, c.to, c.why, r)
		}
	}
}

// A source's inbox takes management's decline and nothing from other desks;
// the open-subject address is management's too (the store refuses any other
// sender at commit).
func TestResolveAddressRegisteredClassesNameTheirSenders(t *testing.T) {
	for _, from := range []string{"texture:d1", "research:d1", "engineering:a1"} {
		for _, to := range []string{"source:s1", types.StoryOpenInbox} {
			if r, err := ResolveAddress(from, contractOwner, to); err == nil {
				t.Errorf("%s -> %s resolved to %+v, want a refusal", from, to, r)
			}
		}
	}
	got := map[string]bool{}
	for _, c := range InboxClasses() {
		got[c.Name] = c.Addressable
	}
	want := map[string]bool{types.StoryOpenInbox: true, captureSourceDesk: true, jobInboxClass: false}
	for name, addressable := range want {
		if v, ok := got[name]; !ok || v != addressable {
			t.Errorf("inbox class %q = (%v, registered %v), want addressable %v", name, v, ok, addressable)
		}
	}
}

// An engineering request from a desk that does not hold the assignment kind
// lands in management's inbox, never as an assignment (authority never
// travels), whatever the form of the address.
func TestResolveAddressEngineeringCastFromNonHolderForwardsToManagement(t *testing.T) {
	for _, from := range []string{"research:d1", "texture:d1"} {
		for _, to := range []string{"engineering", "engineering:a1"} {
			r := contractRoute(t, from, contractOwner, to)
			if !r.OnRoute || !r.Forwarded || r.Inbox != "management:"+contractOwner {
				t.Errorf("%s -> %s: got %+v, want a forward to management:%s", from, to, r, contractOwner)
			}
		}
	}
	// The holder is not forwarded: it names an assignment it opened.
	if r := contractRoute(t, "management:"+contractOwner, contractOwner, "engineering:a1"); r.Forwarded || r.Inbox != "engineering:a1" {
		t.Errorf("management -> engineering:a1: %+v", r)
	}
}

// A registered desk without a records inbox is not refused as unknown,
// and it is not on the records path either.
func TestResolveAddressDeskWithoutInbox(t *testing.T) {
	for _, to := range []string{"conductor", "email", "verifier"} {
		if r, err := ResolveAddress("texture:d1", contractOwner, to); err == nil {
			t.Errorf("texture -> %s resolved to %+v, want a refusal", to, r)
		}
	}
	if !isRecordsDesk(agentprofile.Engineering) || isRecordsDesk(agentprofile.Conductor) || isRecordsDesk("") || isRecordsDesk("app:x") {
		t.Error("isRecordsDesk does not follow the manifest's instances")
	}
	if _, err := ResolveAddress("texture:d1", contractOwner, strings.Repeat("x", 3)); err == nil {
		t.Error("an unregistered name resolved")
	}
}

// "TEXTURE" is the Texture: names are normalised before lookup and before
// invariant 6.
func TestResolveAddressNormalisesDeskNames(t *testing.T) {
	r := contractRoute(t, "research:d1", contractOwner, "TEXTURE")
	if !r.OnRoute || r.Inbox != "texture:d1" {
		t.Errorf("research -> TEXTURE: %+v", r)
	}
	r = contractRoute(t, "texture:d1", contractOwner, " Engineering ")
	if !r.Forwarded || r.Inbox != "management:"+contractOwner {
		t.Errorf("texture -> Engineering: %+v", r)
	}
	err := validateCellIntents(ReductionScope{FromRole: agentprofile.Engineering}, []yaegikernel.StagedIntent{
		{LocalID: "n1", Kind: yaegikernel.IntentNote, ToDesk: "TEXTURE", Body: "x"}})
	if err == nil {
		t.Error("engineering -> TEXTURE evaded invariant 6")
	}
}

// A forwarded request of any kind says in its text that it was aimed at
// engineering.
func TestMarkForwardedNamesTheOriginalAddressee(t *testing.T) {
	for _, in := range []yaegikernel.StagedIntent{
		{Kind: yaegikernel.IntentCast, Objective: "o"}, {Kind: yaegikernel.IntentNote, Body: "o"},
		{Kind: yaegikernel.IntentReply, Answer: "o"},
		{Kind: yaegikernel.IntentReport, Claim: "o"},
	} {
		in.ToDesk = "engineering"
		out := markForwarded(in, "research:d1")
		if !strings.Contains(out.Objective+out.Body+out.Answer+out.Claim, "aimed at engineering by research:d1") {
			t.Errorf("%s not marked: %+v", in.Kind, out)
		}
	}
}

// The off-route inputs that used to commit silently on the packet path
// (CanMessage was their only gate) are refusals, by the resolver and by the
// reducer.
func TestOffRouteRecordsIntentsAreRefused(t *testing.T) {
	for _, c := range []struct{ from, to string }{
		{"research:docA", "research:docB"},
		{"engineering:a1", "engineering:a2"},
		{"management:" + contractOwner, "management-task:t1"},
	} {
		if r, err := ResolveAddress(c.from, contractOwner, c.to); err == nil {
			t.Errorf("%s -> %s resolved to %+v", c.from, c.to, r)
		}
	}
}

// The old-versus-new matrix, reconstructed as a literal table from the
// differential that ran recordsRoute against ResolveAddress: a sender, a
// recipient, and the old and new answers. "off" is the old silent
// fall-through to the packet path; "refuse" an error; "forward" a landing in
// management's inbox. Rows marked changed are the intended differences.
func TestAddressMatrixOldVersusNew(t *testing.T) {
	o := contractOwner
	for _, c := range []struct {
		from, to, old, new string
	}{
		// Unchanged routes.
		{"texture:d1", "research", "research:d1", "research:d1"},
		{"texture:d1", "management", "management:" + o, "management:" + o},
		{"research:d1", "texture", "texture:d1", "texture:d1"},
		{"management:" + o, "texture:d2", "texture:d2", "texture:d2"},
		{"management:" + o, "engineering:a1", "engineering:a1", "engineering:a1"},
		{"management:" + o, "source:s1", "source:s1", "source:s1"},
		{"management:" + o, "texture:new", "texture:new", "texture:new"},
		{"engineering:a1", "management", "management:" + o, "management:" + o},
		{"management-task:t1", "source:s1", "source:s1", "source:s1"},
		{"management-task:t1", "texture:d3", "texture:d3", "texture:d3"},
		{"management-task:t1", "texture:new", "texture:new", "texture:new"},
		// Unchanged refusals.
		{"management:" + o, "texture", "refuse", "refuse"},
		{"management:" + o, "engineering", "refuse", "refuse"},
		{"management-task:t1", "texture", "refuse", "refuse"},
		{"management-task:t1", "engineering:a1", "refuse", "refuse"},
		// Changed: allowlist refusals removed.
		{"research:d1", "management", "off", "management:" + o},
		{"engineering:a1", "research:d2", "off", "research:d2"},
		{"engineering:a1", "texture:d2", "off", "texture:d2"}, // validateCellIntents still refuses it (invariant 6)
		{"management:" + o, "research:d2", "off", "research:d2"},
		// Changed: forwarded to management.
		{"research:d1", "engineering", "off", "forward:management:" + o},
		{"texture:d1", "engineering:a1", "off", "forward:management:" + o},
		// Changed: the silent fall-through is a refusal.
		{"research:d1", "research:d2", "off", "refuse"},
		{"engineering:a1", "engineering:a2", "off", "refuse"},
		{"management:" + o, "management-task:t1", "off", "refuse"},
		{"research:d1", "job:j1", "off", "refuse"},
		{"research:d1", "no-such-desk", "off", "refuse"},
		{"texture:d1", "conductor", "off", "refuse"},
		{"texture:d1", "source:s1", "off", "refuse"},
		{"management:" + o, "management:", "off", "refuse"},
	} {
		r, err := ResolveAddress(c.from, o, c.to)
		got := r.Inbox
		switch {
		case err != nil:
			got = "refuse"
		case r.Forwarded:
			got = "forward:" + r.Inbox
		}
		if got != c.new {
			t.Errorf("%s -> %s: got %q, want %q (old: %s)", c.from, c.to, got, c.new, c.old)
		}
	}
}
