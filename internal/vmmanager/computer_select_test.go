package vmmanager

import "testing"

// The per-computer selector every staged feature reads (FP8 P2): VM ids,
// computer ids, class:<warmness class> and "*". An empty list selects none.
func TestSelectsComputer(t *testing.T) {
	test := VMConfig{VMID: "vm-t", ComputerID: "computer-t", WarmnessClass: "test"}
	primary := VMConfig{VMID: "vm-p", ComputerID: "computer-p", WarmnessClass: "primary"}
	unclassed := VMConfig{VMID: "vm-u", ComputerID: "computer-u"}
	cases := []struct {
		only []string
		cfg  VMConfig
		want bool
	}{
		{nil, test, false},
		{[]string{""}, test, false},
		{[]string{"vm-t"}, test, true},
		{[]string{" computer-t "}, test, true},
		{[]string{"computer-t"}, primary, false},
		{[]string{"class:test"}, test, true},
		{[]string{"class:test"}, primary, false},
		{[]string{"class:test"}, unclassed, false},
		{[]string{"class:"}, unclassed, false},
		{[]string{"class:primary", "vm-u"}, primary, true},
		{[]string{"class:primary", "vm-u"}, unclassed, true},
		{[]string{"*"}, unclassed, true},
		// A token is never matched against the wrong field.
		{[]string{"class:test"}, VMConfig{VMID: "class:test"}, false},
	}
	for i, c := range cases {
		if got := selectsComputer(c.only, c.cfg); got != c.want {
			t.Errorf("case %d: selectsComputer(%q, %+v) = %v, want %v", i, c.only, c.cfg, got, c.want)
		}
	}
	if got := parseComputerSelector(" a, class:test ,,b c\n*"); len(got) != 5 {
		t.Fatalf("parseComputerSelector = %q", got)
	}
}

// SEC-04 rollout: VMCTL_JAILER_ONLY=class:test confines every test computer
// and no other.
func TestDecideConfinementSelectsClassTest(t *testing.T) {
	mgr, _ := jailedManager(t, t.TempDir())
	mgr.cfg.Jailer.Only = []string{"class:test"}
	if mode, _ := mgr.decideConfinement(VMConfig{VMID: "vm-t", ComputerID: "c-t", WarmnessClass: "test"}); mode != ConfinementJailer {
		t.Fatalf("a test computer must be jailed, got %s", mode)
	}
	if mode, reason := mgr.decideConfinement(VMConfig{VMID: "vm-p", ComputerID: "c-p", WarmnessClass: "primary"}); mode != ConfinementDirect || reason != confinementReasonNotSelected {
		t.Fatalf("a primary computer must stay direct, got %s %s", mode, reason)
	}
}
