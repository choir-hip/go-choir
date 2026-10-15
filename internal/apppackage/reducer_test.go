package apppackage

import (
	"errors"
	"testing"
)

// installEvent builds a correctly bound install event for a static fixture.
func installEvent(t *testing.T, name string, mutate func(*Manifest)) Event {
	t.Helper()
	m, b, _, err := BuildStatic(staticFixture(t, name, mutate))
	if err != nil {
		t.Fatal(err)
	}
	md, err := m.Digest()
	if err != nil {
		t.Fatal(err)
	}
	bd, err := b.Digest()
	if err != nil {
		t.Fatal(err)
	}
	return Event{Op: OpInstall, Package: name, Manifest: &m, Build: &b, ManifestDigest: md, BuildDigest: bd, Actor: "owner:test"}
}

func op(o Op, name string) Event {
	return Event{Op: o, Package: name, Actor: "owner:test"}
}

// apply chains an event onto inv (filling seq/prev) and applies it.
func apply(t *testing.T, inv Inventory, ev Event) (Inventory, Effect, Event, error) {
	t.Helper()
	ev, err := Chain(inv, ev)
	if err != nil {
		t.Fatal(err)
	}
	next, effect, err := Apply(inv, ev)
	return next, effect, ev, err
}

func mustApply(t *testing.T, inv Inventory, ev Event) (Inventory, Effect, Event) {
	t.Helper()
	next, effect, chained, err := apply(t, inv, ev)
	if err != nil {
		t.Fatalf("%s %s: %v", ev.Op, ev.Package, err)
	}
	return next, effect, chained
}

func TestReduceTransitions(t *testing.T) {
	inv := NewInventory()
	var events []Event
	step := func(ev Event) Effect {
		var effect Effect
		var chained Event
		inv, effect, chained = mustApply(t, inv, ev)
		events = append(events, chained)
		return effect
	}
	refuse := func(ev Event, want error) {
		t.Helper()
		if _, _, _, err := apply(t, inv, ev); !errors.Is(err, want) {
			t.Fatalf("%s %s: got %v, want %v", ev.Op, ev.Package, err, want)
		}
	}

	// F15: nothing to act on yet.
	refuse(op(OpEnable, "hello"), ErrNotInstalled)
	refuse(op(OpDisable, "hello"), ErrNotInstalled)
	refuse(op(OpPurge, "hello"), ErrNotInstalled)

	install := installEvent(t, "hello", nil)
	step(install)
	if got := inv.Packages["hello"]; got.Status != StatusInstalled || got.BuildDigest != install.BuildDigest || got.Version != "0.1.0" {
		t.Fatalf("after install: %+v", got)
	}
	if inv.Enabled("hello") {
		t.Fatal("install alone enabled the package")
	}
	// F14: a second install is not an update.
	refuse(installEvent(t, "hello", nil), ErrAlreadyInstalled)
	refuse(op(OpDisable, "hello"), ErrInvalidTransition)

	step(op(OpEnable, "hello"))
	if !inv.Enabled("hello") {
		t.Fatal("enable did not enable")
	}
	refuse(op(OpEnable, "hello"), ErrInvalidTransition)
	// F16: purge while enabled is refused; disable first.
	refuse(op(OpPurge, "hello"), ErrInvalidTransition)

	step(op(OpDisable, "hello"))
	if inv.Packages["hello"].Status != StatusDisabled {
		t.Fatalf("after disable: %+v", inv.Packages["hello"])
	}
	step(op(OpEnable, "hello"))
	step(op(OpDisable, "hello"))

	effect := step(op(OpPurge, "hello"))
	if _, present := inv.Packages["hello"]; present {
		t.Fatal("purged package still in the inventory")
	}
	if !effect.DeleteData {
		t.Fatal("purge under delete_data did not ask for data deletion")
	}
	if _, retained := inv.Retained["hello"]; retained {
		t.Fatal("delete_data purge recorded retained data")
	}

	// The fold of the recorded events is the same inventory.
	folded, err := Reduce(events)
	if err != nil {
		t.Fatal(err)
	}
	if folded.Seq != inv.Seq || folded.Head != inv.Head || len(folded.Packages) != 0 {
		t.Fatalf("fold %+v differs from stepwise %+v", folded, inv)
	}
	// A prefix fold is the state at that head: restore to a pre-purge head
	// shows the package again (F23).
	prefix, err := Reduce(events[:2])
	if err != nil {
		t.Fatal(err)
	}
	if !prefix.Enabled("hello") {
		t.Fatalf("prefix fold after install+enable: %+v", prefix.Packages)
	}
	empty, err := Reduce(nil)
	if err != nil || len(empty.Packages) != 0 || empty.Seq != 0 {
		t.Fatalf("empty fold: %+v %v", empty, err)
	}
}

func TestReduceInstallBinding(t *testing.T) {
	cases := map[string]func(*Event){
		"manifest-digest": func(ev *Event) { ev.ManifestDigest = testDigest },
		"build-digest":    func(ev *Event) { ev.BuildDigest = testDigest },
		"build-manifest": func(ev *Event) {
			b := *ev.Build
			b.ManifestDigest = testDigest
			ev.Build = &b
			ev.BuildDigest, _ = b.Digest()
		},
		"build-source": func(ev *Event) {
			b := *ev.Build
			b.SourceDigest = testDigest
			ev.Build = &b
			ev.BuildDigest, _ = b.Digest()
		},
		"build-files": func(ev *Event) {
			b := *ev.Build
			b.Files = append([]BuildFile{{Path: "a.js", SHA256: testDigest, Size: 1, MediaType: "text/javascript"}}, b.Files...)
			ev.Build = &b
			ev.BuildDigest, _ = b.Digest()
		},
		"package-name":     func(ev *Event) { ev.Package = "other" },
		"missing-manifest": func(ev *Event) { ev.Manifest = nil },
		"missing-build":    func(ev *Event) { ev.Build = nil },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			ev := installEvent(t, "hello", nil)
			mutate(&ev)
			if _, _, _, err := apply(t, NewInventory(), ev); err == nil {
				t.Fatalf("install with bad %s accepted", name)
			}
		})
	}
	t.Run("backend", func(t *testing.T) {
		// F18: a backend-declaring package is refused until S7-2.
		ev := installEvent(t, "hello", func(m *Manifest) {
			m.Backend = &Backend{Kind: BackendKindService, Entry: "frontend/index.js", ABI: BackendABIV1}
		})
		if _, _, _, err := apply(t, NewInventory(), ev); !errors.Is(err, ErrUnsupported) {
			t.Fatalf("backend package installed: %v", err)
		}
	})
}

func TestReduceRetainedDataCompat(t *testing.T) {
	retain := func(m *Manifest) { m.Restore.OnPurge = OnPurgeRetainData }
	inv := NewInventory()
	inv, _, _ = mustApply(t, inv, installEvent(t, "notes", retain))
	inv, effect, _ := mustApply(t, inv, op(OpPurge, "notes"))
	if effect.DeleteData {
		t.Fatal("retain_data purge asked for data deletion")
	}
	if inv.Retained["notes"] != 1 {
		t.Fatalf("retained schema not recorded: %+v", inv.Retained)
	}

	// F20: a version that cannot read schema 1 is refused.
	incompatible := installEvent(t, "notes", func(m *Manifest) {
		retain(m)
		m.Version = "0.2.0"
		m.Data.SchemaVersion = 2
		m.Data.ReadsSchemaVersions = []int{2}
	})
	if _, _, _, err := apply(t, inv, incompatible); !errors.Is(err, ErrIncompatibleData) {
		t.Fatalf("incompatible reinstall over retained data: %v", err)
	}
	// A version whose window includes 1 is accepted, and clears the record.
	compatible := installEvent(t, "notes", func(m *Manifest) {
		retain(m)
		m.Version = "0.2.0"
		m.Data.SchemaVersion = 2
		m.Data.ReadsSchemaVersions = []int{1, 2}
	})
	inv, _, _ = mustApply(t, inv, compatible)
	if _, retained := inv.Retained["notes"]; retained {
		t.Fatal("reinstall left the retained record behind")
	}
}

func TestReducePurgeEventMustDescribeRetention(t *testing.T) {
	inv := NewInventory()
	inv, _, _ = mustApply(t, inv, installEvent(t, "hello", nil))
	ev := op(OpPurge, "hello")
	ev.RetainedDataSchemaVersion = 1 // hello deletes data; the event may not claim retention
	if _, _, _, err := apply(t, inv, ev); !errors.Is(err, ErrInvalidEvent) {
		t.Fatalf("purge event lying about retention accepted: %v", err)
	}
}

func TestReduceRefusesBrokenChain(t *testing.T) {
	inv := NewInventory()
	inv, _, first := mustApply(t, inv, installEvent(t, "hello", nil))
	_, _, second := mustApply(t, inv, op(OpEnable, "hello"))

	cases := map[string][]Event{
		"gap":       {first, withSeq(second, 3)},
		"reorder":   {second, first},
		"duplicate": {first, first},
		"prev":      {first, withPrev(second, testDigest)},
		"genesis":   {withPrev(first, testDigest)},
	}
	for name, events := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := Reduce(events); !errors.Is(err, ErrBrokenChain) {
				t.Fatalf("broken chain (%s) folded: %v", name, err)
			}
		})
	}
	if _, err := Reduce([]Event{first, second}); err != nil {
		t.Fatalf("intact chain refused: %v", err)
	}
}

func TestReduceRefusesUnknownOpAndSchema(t *testing.T) {
	inv := NewInventory()
	ev, err := Chain(inv, Event{Op: "upgrade", Package: "hello", Actor: "owner:test"})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := Apply(inv, ev); !errors.Is(err, ErrInvalidEvent) {
		t.Fatalf("unknown op: %v", err)
	}
	ev, _ = Chain(inv, installEvent(t, "hello", nil))
	ev.Schema = "choir.app-package-event/v0"
	if _, _, err := Apply(inv, ev); !errors.Is(err, ErrInvalidEvent) {
		t.Fatalf("unknown event schema: %v", err)
	}
}

func TestApplyDoesNotMutateInput(t *testing.T) {
	inv := NewInventory()
	inv, _, _ = mustApply(t, inv, installEvent(t, "hello", nil))
	before := inv.Packages["hello"].Status
	if _, _, _, err := apply(t, inv, op(OpEnable, "hello")); err != nil {
		t.Fatal(err)
	}
	if inv.Packages["hello"].Status != before {
		t.Fatal("Apply mutated its input inventory")
	}
}

func withSeq(ev Event, seq uint64) Event { ev.Seq = seq; return ev }

func withPrev(ev Event, prev string) Event { ev.Prev = prev; return ev }
