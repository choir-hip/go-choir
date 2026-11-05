package deskprotocol

import (
	"strings"
	"testing"
)

func base() *Surface {
	return &Surface{
		Schema: SurfaceSchema, Version: "1.0.0",
		Names: map[string]Name{
			"t.Req": {Kind: "struct", Direction: Emitted, Fields: map[string]Field{
				"A": {Type: "string", JSON: "a"},
				"B": {Type: "[]string", JSON: "b", Optional: true},
			}},
			"t.Resp": {Kind: "struct", Direction: Received, Fields: map[string]Field{"A": {Type: "string", JSON: "a"}}},
			"t.Kind": {Kind: "enum", Direction: Both, Underlying: "string", Values: map[string]string{"X": `"x"`, "Y": `"y"`}},
			"t.Open": {Kind: "enum", Direction: Both, Open: true, Underlying: "string", Values: map[string]string{"X": `"x"`}},
			"t.Max":  {Kind: "const", Direction: Both, Quota: true, Value: "16"},
			"t.F":    {Kind: "func", Direction: Both, Sig: &Sig{Params: []string{"string"}, Names: []string{"a"}, Results: []string{"error"}}},
		},
		Verbs:       map[string]Verb{"Cast": {Sig: &Sig{Params: []string{"string"}, Names: []string{"d"}, Results: []string{"error"}}}},
		Desks:       map[string]DeskRow{"m": {Kind: "system", Verbs: []string{"Cast"}, CapsuleVerbs: []string{"exec"}}},
		Transitions: map[string][]string{"work_item": {"open>completed"}},
		Enforced:    []string{"work_item"},
	}
}

func clone(s *Surface) *Surface {
	raw, _ := s.Marshal()
	out, err := parseNoSeal(raw)
	if err != nil {
		panic(err)
	}
	return out
}

func mutate(f func(*Surface)) Result {
	b := base()
	c := clone(b)
	f(c)
	return Classify(b, c)
}

// Each case is a negative test of the checker: one edit, the classification
// it must get.
func TestClassifyRules(t *testing.T) {
	cases := []struct {
		name string
		edit func(*Surface)
		want Level
	}{
		{"identical", func(s *Surface) {}, LevelNone},
		{"field leaves", func(s *Surface) { delete(s.Names["t.Req"].Fields, "A") }, LevelMajor},
		{"field type changes", func(s *Surface) { s.Names["t.Req"].Fields["A"] = Field{Type: "int", JSON: "a"} }, LevelMajor},
		{"json tag changes", func(s *Surface) { s.Names["t.Req"].Fields["A"] = Field{Type: "string", JSON: "aa"} }, LevelMajor},
		{"required field added to emitted type", func(s *Surface) { s.Names["t.Req"].Fields["C"] = Field{Type: "string", JSON: "c"} }, LevelMajor},
		{"optional field added", func(s *Surface) { s.Names["t.Req"].Fields["C"] = Field{Type: "string", JSON: "c", Optional: true} }, LevelMinor},
		{"required field added to received type", func(s *Surface) { s.Names["t.Resp"].Fields["C"] = Field{Type: "string", JSON: "c"} }, LevelMinor},
		{"optional becomes required (emitted)", func(s *Surface) { s.Names["t.Req"].Fields["B"] = Field{Type: "[]string", JSON: "b"} }, LevelMajor},
		{"required becomes optional", func(s *Surface) { s.Names["t.Req"].Fields["A"] = Field{Type: "string", JSON: "a", Optional: true} }, LevelMinor},
		{"closed enum gains a value", func(s *Surface) { s.Names["t.Kind"].Values["Z"] = `"z"` }, LevelMajor},
		{"open enum gains a value", func(s *Surface) { s.Names["t.Open"].Values["Z"] = `"z"` }, LevelMinor},
		{"enum value leaves", func(s *Surface) { delete(s.Names["t.Kind"].Values, "Y") }, LevelMajor},
		{"enum value changes", func(s *Surface) { s.Names["t.Kind"].Values["Y"] = `"yy"` }, LevelMajor},
		{"open enum closes", func(s *Surface) { n := s.Names["t.Open"]; n.Open = false; s.Names["t.Open"] = n }, LevelMajor},
		{"name added", func(s *Surface) { s.Names["t.New"] = Name{Kind: "type"} }, LevelMinor},
		{"name leaves", func(s *Surface) { delete(s.Names, "t.F") }, LevelMajor},
		{"quota raised", func(s *Surface) { n := s.Names["t.Max"]; n.Value = "32"; s.Names["t.Max"] = n }, LevelMinor},
		{"quota lowered", func(s *Surface) { n := s.Names["t.Max"]; n.Value = "8"; s.Names["t.Max"] = n }, LevelMajor},
		{"param renamed", func(s *Surface) { s.Names["t.F"].Sig.Names[0] = "b" }, LevelPatch},
		{"param type changes", func(s *Surface) { s.Names["t.F"].Sig.Params[0] = "int" }, LevelMajor},
		{"param added", func(s *Surface) {
			s.Names["t.F"].Sig.Params = append(s.Names["t.F"].Sig.Params, "int")
			s.Names["t.F"].Sig.Names = append(s.Names["t.F"].Sig.Names, "n")
		}, LevelMajor},
		{"trailing variadic added", func(s *Surface) {
			g := s.Names["t.F"].Sig
			g.Params = append(g.Params, "[]int")
			g.Names = append(g.Names, "n")
			g.Variadic = true
		}, LevelMinor},
		{"result changes", func(s *Surface) { s.Names["t.F"].Sig.Results[0] = "bool" }, LevelMajor},
		{"verb leaves", func(s *Surface) { delete(s.Verbs, "Cast") }, LevelMajor},
		{"verb added", func(s *Surface) { s.Verbs["Ask"] = Verb{Closure: true} }, LevelMinor},
		{"desk verb leaves", func(s *Surface) { d := s.Desks["m"]; d.Verbs = nil; s.Desks["m"] = d }, LevelMajor},
		{"desk verb added", func(s *Surface) { d := s.Desks["m"]; d.Verbs = []string{"Ask", "Cast"}; s.Desks["m"] = d }, LevelMinor},
		{"desk module added", func(s *Surface) { d := s.Desks["m"]; d.Modules = []string{"updates.read"}; s.Desks["m"] = d }, LevelMajor},
		{"desk capsule verb added", func(s *Surface) { d := s.Desks["m"]; d.CapsuleVerbs = []string{"exec", "go_eval"}; s.Desks["m"] = d }, LevelMajor},
		{"desk capsule verb leaves", func(s *Surface) { d := s.Desks["m"]; d.CapsuleVerbs = nil; s.Desks["m"] = d }, LevelMajor},
		{"desk owned kind added", func(s *Surface) { d := s.Desks["m"]; d.OwnedKinds = []string{"assignment"}; s.Desks["m"] = d }, LevelMajor},
		{"desk taint changes", func(s *Surface) { d := s.Desks["m"]; d.Taint = "privileged"; s.Desks["m"] = d }, LevelMajor},
		{"desk carrier changes", func(s *Surface) { d := s.Desks["m"]; d.Carrier = "capsule"; s.Desks["m"] = d }, LevelMajor},
		{"edge leaves a terminal state of an enforced machine", func(s *Surface) {
			s.Transitions["work_item"] = append(s.Transitions["work_item"], "completed>open")
		}, LevelMajor},
		{"desk profile flips", func(s *Surface) { d := s.Desks["m"]; d.Profile = true; s.Desks["m"] = d }, LevelMajor},
		{"desk identity changes", func(s *Surface) { d := s.Desks["m"]; d.Identity = "owner"; s.Desks["m"] = d }, LevelMajor},
		{"desk own channel flips", func(s *Surface) { d := s.Desks["m"]; d.OwnChannel = true; s.Desks["m"] = d }, LevelMajor},
		{"desk prompt flips", func(s *Surface) { d := s.Desks["m"]; d.Prompt = true; s.Desks["m"] = d }, LevelMajor},
		{"desk cell deadline changes", func(s *Surface) { d := s.Desks["m"]; d.CellDeadlineNs = 1; s.Desks["m"] = d }, LevelMajor},
		{"desk worker memory changes", func(s *Surface) { d := s.Desks["m"]; d.WorkerMaxBytes = 1; s.Desks["m"] = d }, LevelMajor},
		{"desk capsule request changes", func(s *Surface) { d := s.Desks["m"]; d.CapsuleRequestedBytes = 1; s.Desks["m"] = d }, LevelMajor},
		{"desk model role changes", func(s *Surface) { d := s.Desks["m"]; d.ModelRole = "x"; s.Desks["m"] = d }, LevelMajor},
		{"machine leaves enforcement", func(s *Surface) { s.Enforced = nil }, LevelMajor},
		{"machine gains enforcement", func(s *Surface) { s.Enforced = append(s.Enforced, "trajectory") }, LevelMinor},
		{"desk leaves", func(s *Surface) { delete(s.Desks, "m") }, LevelMajor},
		{"transition leaves", func(s *Surface) { s.Transitions["work_item"] = nil }, LevelMajor},
		{"transition added", func(s *Surface) { s.Transitions["work_item"] = append(s.Transitions["work_item"], "open>refused") }, LevelMinor},
		{"direction changes", func(s *Surface) { n := s.Names["t.Req"]; n.Direction = Received; s.Names["t.Req"] = n }, LevelMajor},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := mutate(tc.edit)
			if got.Level != tc.want {
				t.Fatalf("level %s, want %s:\n%s", got.Level, tc.want, FormatChanges(got))
			}
		})
	}
}

func TestBump(t *testing.T) {
	for _, c := range []struct {
		a, b string
		want Level
	}{{"1.0.0", "1.0.0", LevelNone}, {"1.0.0", "1.0.1", LevelPatch}, {"1.0.5", "1.1.0", LevelMinor}, {"1.4.2", "2.0.0", LevelMajor}} {
		got, err := Bump(c.a, c.b)
		if err != nil || got != c.want {
			t.Errorf("Bump(%s,%s) = %v, %v; want %v", c.a, c.b, got, err, c.want)
		}
	}
	if _, err := Bump("1.1.0", "1.0.9"); err == nil {
		t.Error("a downward step must be an error")
	}
	if _, err := ParseVersion("1.0"); err == nil {
		t.Error("1.0 is not a version")
	}
}

func TestSealDetectsInPlaceEdit(t *testing.T) {
	s := base()
	if err := s.Seal(); err != nil {
		t.Fatal(err)
	}
	raw, _ := s.Marshal()
	if _, err := ParseLock(raw); err != nil {
		t.Fatalf("sealed lock rejected: %v", err)
	}
	edited := strings.Replace(string(raw), `"verbs": [`, `"verbs": [ "Sneak",`, 1)
	if _, err := ParseLock([]byte(edited)); err == nil || !strings.Contains(err.Error(), "edited in place") {
		t.Fatalf("an in-place edit must fail the seal, got %v", err)
	}
}

func TestLockSetStepCheck(t *testing.T) {
	big := func(v string) *Surface {
		s := base()
		s.Version = v
		for i := 0; i < MinNames; i++ {
			s.Names["pad."+string(rune('A'+i%26))+string(rune('a'+i/26))] = Name{Kind: "type"}
		}
		for i := 0; i < MinVerbs; i++ {
			s.Verbs["V"+string(rune('A'+i%26))+string(rune('a'+i/26))] = Verb{Closure: true}
		}
		for i := 0; i < MinDesks; i++ {
			s.Desks["d"+string(rune('a'+i))] = DeskRow{Kind: "system"}
		}
		return s
	}
	a, b := big("1.0.0"), big("1.0.1")
	delete(b.Names, "t.F") // a name leaves: major
	if p := CheckLockSet([]*Surface{a, b}); len(p) == 0 {
		t.Fatal("a patch bump over a major change must fail")
	}
	b.Version = "2.0.0"
	if p := CheckLockSet([]*Surface{a, b}); len(p) != 0 {
		t.Fatalf("a major bump over a major change must pass: %v", p)
	}
	small := base()
	if p := CheckLockSet([]*Surface{small}); len(p) == 0 {
		t.Fatal("a lock below the floors must fail")
	}
}

func TestTransitionTable(t *testing.T) {
	if !Allowed(MachineWorkItem, "open", "completed") || Allowed(MachineWorkItem, "completed", "open") || Allowed(MachineWorkItem, "open", "open") {
		t.Fatal("work item rows")
	}
	if !Terminal(MachineWorkItem, "refused") || Terminal(MachineWorkItem, "open") || Terminal(MachineWorkItem, "") || Terminal(MachineWorkItem, "bogus") {
		t.Fatal("Terminal must be false for a state the table does not name")
	}
	seen := map[Transition]bool{}
	for _, tr := range Transitions() {
		if seen[tr] {
			t.Fatalf("duplicate row %v", tr)
		}
		seen[tr] = true
	}
}
