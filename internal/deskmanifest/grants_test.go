package deskmanifest

import (
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"
)

// Failure modes (MA4.1, written first):
//
//	G1 A grant edit changes a built-in desk's authority silently.
//	   -> the pinned literals below are the verbs, capsule verbs, modules and
//	      budgets the tables held before grants; the union must equal them.
//	G2 A power with no grant (a verb in no grant) or a verb in two grants.
//	   -> every verb the table uses is in exactly one grant (init panics on
//	      a duplicate); GrantsForVerbs refuses an unmapped verb.
//	G3 A package asks for a privileged grant.
//	   -> NewAppDesk accepts only the app set; GrantPrivileged marks the rest.
//	G4 A role widens its parent, or holds Done without ending its run.
//	   -> validateRole refuses both.
//	G5 A bad argument (an unknown rule kind, a non-duration).
//	   -> resolve refuses.
func TestGrantUnionReproducesTodaysAuthority(t *testing.T) {
	type want struct {
		verbs, capsule, modules []string
		cell                    time.Duration
	}
	pinned := map[string]want{
		"engineering": {verbs: []string{"Ask", "Calibration", "Complete", "Exec", "Freeze", "Poll", "Precommit", "Reply", "Report", "Restate", "Start", "Stop", "WriteFile"},
			capsule: []string{"close_session", "exec", "go_eval", "init_session", "job_start", "job_status", "job_stop", "kill_session", "list_dir", "mkdir", "read_file", "remove", "stat", "write_file"}},
		"management": {verbs: []string{"Ask", "Calibration", "CancelAct", "CancelAssignment", "CancelTask", "Cast", "Decline", "Delegate", "Disagreement", "Escalate", "Feed", "Precommit", "ProductAPI", "ReadCapture", "Reply", "Report", "ReportPacket", "Resolve", "Restate", "Stories", "Verify"},
			modules: []string{"updates.read"}, cell: 300 * time.Second},
		"management_task": {verbs: []string{"Cast", "Decline", "Done", "Feed", "ReadCapture", "Stories"}, cell: 300 * time.Second},
		"research": {verbs: []string{"Ask", "CancelAct", "Cast", "Disagreement", "Escalate", "ListEvidence", "PollSource", "Precommit", "Read", "ReadCapture", "ReadEvidence", "Relate", "RelateStories", "Reply", "Report", "ReportPacket", "Research", "Resolve", "RunMemoryEntry", "SaveEvidence", "SearchStart", "SearchWait", "SearchWireCorpus", "Searches", "SourceAPI", "StoryAnalysis", "System1"},
			capsule: []string{"go_eval", "list_dir", "read_file", "stat"}},
		"research_child": {verbs: []string{"ListEvidence", "Read", "ReadCapture", "ReadEvidence", "SearchStart", "SearchWait", "SearchWireCorpus", "Searches"}},
		"texture":        {verbs: []string{"Ask", "Cast", "Done", "ReadCapture", "ReadDoc", "Reject", "Wait", "Write"}, modules: []string{"updates.read"}},
	}
	for name, w := range pinned {
		d, ok := Lookup(name)
		if !ok {
			t.Fatalf("desk %s missing", name)
		}
		sorted := func(in []string) []string { out := append([]string(nil), in...); sort.Strings(out); return out }
		norm := func(in []string) []string {
			if len(in) == 0 {
				return nil
			}
			return sorted(in)
		}
		if got := norm(d.Verbs); !reflect.DeepEqual(got, norm(w.verbs)) {
			t.Errorf("%s verbs %v, want %v", name, got, w.verbs)
		}
		if got := norm(d.CapsuleVerbs); !reflect.DeepEqual(got, norm(w.capsule)) {
			t.Errorf("%s capsule verbs %v, want %v", name, got, w.capsule)
		}
		if got := norm(d.Modules); !reflect.DeepEqual(got, norm(w.modules)) {
			t.Errorf("%s modules %v, want %v", name, got, w.modules)
		}
		if d.CellDeadline != w.cell {
			t.Errorf("%s cell deadline %s, want %s", name, d.CellDeadline, w.cell)
		}
	}
	// Run budgets: only the task has a run bound; engineering alone stages rules.
	if d, _ := Lookup("management_task"); d.RunDeadline() != 10*time.Minute {
		t.Errorf("task run deadline %s, want 10m", d.RunDeadline())
	}
	for _, name := range Names() {
		d, _ := Lookup(name)
		if name != "management_task" && d.RunDeadline() != 0 {
			t.Errorf("%s has a run deadline %s", name, d.RunDeadline())
		}
		if d.RulePrecommit() != (name == "engineering") {
			t.Errorf("%s RulePrecommit = %v", name, d.RulePrecommit())
		}
	}
	// A desk with no grants keeps its (empty) fields: the carrier-less rows.
	for _, name := range []string{"conductor", "email"} {
		if d, _ := Lookup(name); len(d.Grants) != 0 || len(d.Verbs) != 0 {
			t.Errorf("%s should hold nothing", name)
		}
	}
}

func TestEveryGrantedVerbIsInOneGrantAndPrivilegeIsMarked(t *testing.T) {
	for _, name := range Names() {
		d, _ := Lookup(name)
		got, err := GrantsForVerbs(d.Verbs)
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		for _, g := range got {
			if !d.HasGrant(g.Name) {
				t.Errorf("%s: verbs imply grant %s the desk does not hold", name, g.Name)
			}
		}
	}
	for _, g := range []string{"capsule.mutate", "cancel.assignment", "intake.dispose", "precommit.rule", "world.read", "delegate.task", "product.api"} {
		if !GrantPrivileged(g) {
			t.Errorf("%s must be first-party only", g)
		}
	}
	if !GrantPrivileged("no.such.grant") {
		t.Error("an unknown grant must read as privileged (deny)")
	}
	for _, g := range []string{"act.cast", "act.cancel", "captures.read", "lease.request", "lifecycle.done"} {
		if GrantPrivileged(g) {
			t.Errorf("%s is in the app set", g)
		}
	}
	// The app verb set and the unprivileged verb grants agree.
	for v := range appDeskVerbs {
		g, ok := VerbGrant(v)
		if !ok || GrantPrivileged(g) {
			t.Errorf("app verb %s is carried by %q (privileged or missing)", v, g)
		}
	}
}

func TestAppDeskRefusesPrivilegedGrantVerbs(t *testing.T) {
	for _, v := range []string{"CancelAssignment", "Feed", "Decline", "Stories", "ProductAPI", "Delegate", "WriteFile", "Read"} {
		_, err := NewAppDesk(AppDeskSpec{Name: "app:grantprobe", Verbs: []string{v}, Taint: TaintUntrustedInput, UID: 62000, WorkerMaxBytes: mib})
		if err == nil {
			t.Errorf("app desk with %s must be refused", v)
		}
	}
	d, err := NewAppDesk(AppDeskSpec{Name: "app:grantprobe", Verbs: []string{"Cast", "CancelAct"}, Taint: TaintUntrustedInput, UID: 62000, WorkerMaxBytes: mib})
	if err != nil {
		t.Fatal(err)
	}
	if got := d.GrantStrings(); !reflect.DeepEqual(got, []string{"act.cancel", "act.cast"}) {
		t.Errorf("app grants %v", got)
	}
}

func TestResolveRefusals(t *testing.T) {
	base := func(g ...Grant) Desk {
		return Desk{Name: "probe", Kind: KindSystem, Carrier: CarrierNone, Sandbox: Sandbox{Profile: "none"}, Taint: TaintPrivileged, Restart: RestartContract, Grants: g}
	}
	cases := map[string]Desk{
		"unknown grant":     base(G("no.such")),
		"grant twice":       base(G("act.cast"), G("act.cast")),
		"unwanted argument": base(G("act.cast", "x")),
		"unknown rule kind": base(G("precommit.rule", "vibes")),
		"empty rule kinds":  base(G("precommit.rule")),
		"bad duration":      base(G("cell.deadline", "soon")),
		"zero duration":     base(G("run.deadline", "0s")),
	}
	for name, d := range cases {
		if err := d.resolve(); err == nil {
			t.Errorf("%s: resolved", name)
		}
	}
	d := base(G("act.cast"))
	d.Verbs = []string{"Cast", "Exec"}
	if err := d.resolve(); err == nil || !strings.Contains(err.Error(), "not what its grants produce") {
		t.Errorf("a verb beside the grants must be refused, got %v", err)
	}
}

func TestRoleMayOnlyNarrowExceptDone(t *testing.T) {
	role := func(ends bool, g ...Grant) Desk {
		return Desk{Name: "probe_role", Kind: KindRole, Carrier: CarrierDeskWorker, Sandbox: Sandbox{Profile: "desk_cell", UID: 61001},
			Taint: TaintPrivileged, Memory: Memory{WorkerMaxBytes: mib}, Restart: RestartContract, Grants: g, EndsRun: ends}
	}
	if err := register(role(false, G("act.cast"), G("lifecycle.done"))); err == nil {
		t.Error("done without ends_run must be refused")
	}
	if err := register(role(true, G("act.cast"), G("capsule.mutate"))); err == nil {
		t.Error("a role that widens its parent must be refused")
	}
	if err := register(role(false, G("act.cast"), G("precommit.rule", "tests_pass"))); err == nil {
		t.Error("a role may not take precommit.rule its parent lacks")
	}
	if err := register(role(false, G("act.cast"))); err != nil {
		t.Fatalf("a narrowing role registers: %v", err)
	}
	mu.Lock()
	delete(registry, "probe_role")
	mu.Unlock()
	orphan := role(false, G("act.cast"))
	orphan.Sandbox.UID = 60999
	if err := register(orphan); err == nil {
		t.Error("a role with no parent must be refused")
	}
}

func TestPrecommitRuleGrantIsTheClosedKinds(t *testing.T) {
	d, _ := Lookup("engineering")
	for _, k := range PrecommitRuleKinds {
		if !d.MayRule(k) {
			t.Errorf("engineering may not stage rule %s", k)
		}
	}
	if d.MayRule("vibes") {
		t.Error("engineering staged an unknown rule kind")
	}
	if m, _ := Lookup("management"); m.MayRule("tests_pass") || m.RulePrecommit() {
		t.Error("management must not stage rule precommits")
	}
}
