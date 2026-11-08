package agentcore

import (
	"testing"

	"github.com/yusefmosiah/go-choir/internal/deskmanifest"
	"github.com/yusefmosiah/go-choir/internal/disclosure"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Enforce flip S1, D1 (sections 6.2, 7.3), FM1 and FM2, written first.
//
//	FM1 OwnerFlow is set for a package desk, or for a built-in desk whose grant
//	    moved to a package.
//	FM2 OwnerFlow lifts a Deny, or covers a non-desk-message sink.

func TestOwnerFlowPairsAreTheDelegationChainOnly(t *testing.T) {
	tex := deskmanifest.Texture
	res := deskmanifest.ResearcherName()
	mgmt := deskmanifest.AdmitterName()
	eng := deskmanifest.PerformerName()
	task := deskmanifest.ManagementTask
	for _, c := range []struct {
		from, to string
		want     bool
	}{
		{tex, res, true}, {res, tex, true},
		{tex, mgmt, true}, {mgmt, tex, true},
		{mgmt, eng, true}, {eng, mgmt, true},
		{mgmt, task, true}, {task, mgmt, true},
		// Nothing else is covered.
		{tex, eng, false}, {eng, tex, false},
		{res, eng, false}, {eng, res, false},
		{res, mgmt, false}, {mgmt, res, false},
		{task, tex, false}, {task, eng, false}, {task, res, false},
		{deskmanifest.ResearchWeb, res, false}, {res, deskmanifest.ResearchWeb, false},
		{tex, tex, false}, {mgmt, mgmt, false},
		{deskmanifest.Mail, tex, false}, {tex, deskmanifest.Mail, false},
		{deskmanifest.Mail, mgmt, false}, {mgmt, "app:notes", false},
		{"nonexistent", tex, false}, {tex, "", false}, {"", "", false},
	} {
		if got := ownerDeskPair(c.from, c.to); got != c.want {
			t.Errorf("ownerDeskPair(%q, %q) = %v, want %v", c.from, c.to, got, c.want)
		}
	}
}

// FM1: a role whose grant moved to a package desk, or a package desk, is
// never an end of a pair. The roles are built from a holder lookup, so a
// holder that is an app desk yields no pair at all.
func TestFM1AGrantHeldByAPackageDeskCoversNoPair(t *testing.T) {
	app := deskmanifest.Desk{Name: "app:notes", Kind: deskmanifest.KindApp}
	sys := func(name string) deskmanifest.Desk { return deskmanifest.Desk{Name: name, Kind: deskmanifest.KindSystem} }
	holders := func(admitter, performer, researcher deskmanifest.Desk) func(string) (deskmanifest.Desk, bool) {
		return func(grant string) (deskmanifest.Desk, bool) {
			switch grant {
			case deskmanifest.GrantAdmitAssignment:
				return admitter, true
			case deskmanifest.GrantPerformAssignment:
				return performer, true
			case deskmanifest.GrantResearchSpawn:
				return researcher, true
			}
			return deskmanifest.Desk{}, false
		}
	}
	ok := ownerFlowRolesFrom(holders(sys("m"), sys("e"), sys("r")), "t")
	if !ok.pair("m", "e") || !ok.pair("t", "r") || !ok.pair("t", "m") {
		t.Fatalf("system holders yield no pairs: %+v", ok)
	}
	moved := ownerFlowRolesFrom(holders(app, sys("e"), sys("r")), "t")
	if moved.pair("app:notes", "e") || moved.pair("t", "app:notes") || moved.pair("m", "e") {
		t.Errorf("an admitter grant held by a package desk still covers a pair: %+v", moved)
	}
	if !moved.pair("t", "r") {
		t.Error("an unrelated pair was lost when one grant moved")
	}
	if none := ownerFlowRolesFrom(func(string) (deskmanifest.Desk, bool) { return deskmanifest.Desk{}, false }, "t"); none.pair("t", "r") || none.pair("", "") {
		t.Errorf("no holders must cover no pair: %+v", none)
	}
	// A package desk is never an end, whatever the registry holds.
	for _, from := range []string{deskmanifest.Mail, "app:notes"} {
		for _, to := range []string{deskmanifest.Texture, deskmanifest.AdmitterName(), deskmanifest.ResearcherName(), deskmanifest.PerformerName()} {
			if ownerDeskPair(from, to) || ownerDeskPair(to, from) {
				t.Errorf("package desk %s covered with %s", from, to)
			}
		}
	}
}

// FM2: the rule lifts an Ask only, on the desk-message sink only, for the
// seven atoms only.
func TestFM2OwnerFlowLiftsOnlyAnAskOnTheDeskMessageSink(t *testing.T) {
	atoms := disclosure.OwnerFlowAtoms()
	want := []string{"capture", "evidence", "document", "texture", "model-policy", "computers", "prompt-bar"}
	if len(atoms) != len(want) {
		t.Fatalf("atoms = %v", atoms)
	}
	var all []string
	for i, a := range atoms {
		if a != want[i] {
			t.Fatalf("atom %d = %s, want %s", i, a, want[i])
		}
		all = append(all, types.OwnerPrivateLabel(a))
		for _, sink := range disclosure.Sinks {
			got := disclosure.OwnerFlow(types.OwnerPrivateLabel(a), sink, true)
			if got != (sink == disclosure.SinkDeskMessage) {
				t.Errorf("OwnerFlow(%s, %s) = %v", a, sink, got)
			}
		}
	}
	for _, other := range []string{"mail", "run_memory", "story", "app_object", "router", "prompt_bar", "leases", "x"} {
		if disclosure.OwnerFlow(types.OwnerPrivateLabel(other), disclosure.SinkDeskMessage, true) {
			t.Errorf("atom %s is covered", other)
		}
	}
	// A Deny in the table survives the rule.
	cell, _ := disclosure.CellFor(disclosure.ClassOwnerPrivate, disclosure.SinkDeskMessage)
	disclosure.Table[disclosure.ClassOwnerPrivate][disclosure.SinkDeskMessage] = disclosure.Cell{Unadmitted: disclosure.Deny, Admitted: disclosure.Deny}
	defer func() { disclosure.Table[disclosure.ClassOwnerPrivate][disclosure.SinkDeskMessage] = cell }()
	v := disclosure.Decide(types.NewLabelSet(all...), disclosure.SinkDeskMessage, disclosure.Facts{OwnerDesks: true})
	if v.Decision != disclosure.Deny {
		t.Errorf("OwnerFlow lifted a Deny: %+v", v)
	}
}
