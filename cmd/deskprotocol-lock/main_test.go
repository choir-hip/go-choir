package main

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/deskprotocol"
	"github.com/yusefmosiah/go-choir/internal/deskprotocol/extract"
)

// The check the CI job runs, on the real extraction: it passes against its own
// lock, and a deliberately breaking change to the surface makes it fail with the
// right classification. This is the demonstration that a build that breaks the
// protocol fails the job (MA1.4).
func TestCheckPassesOwnLockAndFailsOnBreakingChanges(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	cur, err := extract.Surface(root)
	if err != nil {
		t.Fatal(err)
	}
	locks, err := deskprotocol.LoadLocks(filepath.Join(root, filepath.FromSlash(deskprotocol.LockDir)))
	if err != nil {
		t.Fatal(err)
	}
	if ps := compare(locks, cur); len(ps) != 0 {
		t.Fatalf("the surface does not match its own lock:\n%s", strings.Join(ps, "\n"))
	}
	if ps := deskprotocol.CheckLockSet(locks); len(ps) != 0 {
		t.Fatalf("lock set: %v", ps)
	}

	mutate := func(f func(s *deskprotocol.Surface)) *deskprotocol.Surface {
		raw, _ := json.Marshal(cur)
		var out deskprotocol.Surface
		if err := json.Unmarshal(raw, &out); err != nil {
			t.Fatal(err)
		}
		f(&out)
		return &out
	}
	cases := []struct {
		name  string
		level string
		f     func(s *deskprotocol.Surface)
	}{
		{"a field is removed from the commitment record", "major", func(s *deskprotocol.Surface) {
			delete(s.Names["types.CommitmentRecord"].Fields, "RecordID")
		}},
		{"a required field is added to a record desks emit", "major", func(s *deskprotocol.Surface) {
			n := s.Names["types.RequestFields"]
			n.Fields["Urgency"] = deskprotocol.Field{Type: "string", JSON: "urgency"}
			s.Names["types.RequestFields"] = n
		}},
		{"a value is added to a closed enum", "major", func(s *deskprotocol.Surface) {
			s.Names["types.CommitmentRecordKind"].Values["CommitmentKindSurprise"] = `"surprise"`
		}},
		{"a module is added to a desk", "major", func(s *deskprotocol.Surface) {
			d := s.Desks["engineering"]
			d.Modules = append(d.Modules, "zz.module")
			s.Desks["engineering"] = d
		}},
		{"a capsule verb is added to a desk", "major", func(s *deskprotocol.Surface) {
			d := s.Desks["research"]
			d.CapsuleVerbs = append(d.CapsuleVerbs, "zz_verb")
			s.Desks["research"] = d
		}},
		{"a verb leaves", "major", func(s *deskprotocol.Surface) { delete(s.Verbs, "Cast") }},
	}
	for _, c := range cases {
		ps := compare(locks, mutate(c.f))
		if len(ps) == 0 {
			t.Errorf("%s: the check passed; it must fail the job", c.name)
			continue
		}
		if !strings.Contains(ps[0], "("+c.level+" change)") {
			t.Errorf("%s: classified wrongly: %s", c.name, ps[0])
		}
	}
}
