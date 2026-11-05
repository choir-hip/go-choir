package appdesk

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/deskmanifest"
)

// Failure modes (MA4.4, B4, written first):
//
//	Q1 A restore to a head before a required package was installed boots
//	   without it. -> Boot appends an install event after the restored head.
//	Q2 The re-seed is not idempotent: every boot appends again.
//	   -> a second Boot appends nothing.
//	Q3 A required package older than the image's stays old.
//	   -> an update event is appended; a newer one is left alone.
//	Q4 A required package can be disabled (or uninstalled) by an event.
//	   -> the fold refuses the disable and the package stays.
//	Q5 The re-seed touches open obligations or schedules work.
//	   -> Boot only appends events and registers desks (its Log has no other
//	      power); the crash boot closes obligations in its own pass.
//	Q6 The image's required set is malformed and the boot starts without it.
//	   -> parseRequired refuses a wrong manifest, a bad version, a duplicate.
//	Q7 The log's events are lost in transit. -> encode/decode round-trips.
type memLog struct {
	events []Event
}

func (l *memLog) Events(context.Context) ([]Event, error) {
	return append([]Event(nil), l.events...), nil
}
func (l *memLog) Append(_ context.Context, ev Event) (uint64, error) {
	ev.Seq = uint64(len(l.events)+1) * 10
	l.events = append(l.events, ev)
	return ev.Seq, nil
}

func requiredPkg(pkg, version string) Required {
	return Required{Package: pkg, Version: version, Manifest: goodManifest(pkg)}
}

func TestBootReseedsARequiredPackageAfterARestoredHead(t *testing.T) {
	req := []Required{requiredPkg("reqone", "1.2.0")}
	t.Cleanup(func() { cleanupAll("app:reqone") })
	// The restored head predates the install: the log is empty.
	log := &memLog{}
	set, refusals, err := Boot(context.Background(), log, req)
	if err != nil || len(refusals) != 0 {
		t.Fatalf("boot: %v %+v", err, refusals)
	}
	if len(log.events) != 1 || log.events[0].Action != ActionInstall || log.events[0].Version != "1.2.0" {
		t.Fatalf("re-seed events %+v", log.events)
	}
	if _, ok := deskmanifest.Lookup("app:reqone"); !ok || set.Versions["reqone"] != "1.2.0" {
		t.Fatal("the required desk is not registered after the boot")
	}
	if deskmanifest.CurrentEpoch("app:reqone") != log.events[0].Seq {
		t.Errorf("epoch %d, want the new event's sequence %d", deskmanifest.CurrentEpoch("app:reqone"), log.events[0].Seq)
	}
	// Idempotent.
	if _, _, err := Boot(context.Background(), log, req); err != nil || len(log.events) != 1 {
		t.Fatalf("a second boot appended: %v %d", err, len(log.events))
	}
}

func TestBootUpdatesAnOlderRequiredPackageAndLeavesANewerOne(t *testing.T) {
	t.Cleanup(func() { cleanupAll("app:requp") })
	old := requiredPkg("requp", "1.0.0")
	log := &memLog{}
	if _, _, err := Boot(context.Background(), log, []Required{old}); err != nil {
		t.Fatal(err)
	}
	newer := requiredPkg("requp", "1.1.0")
	newer.Manifest.Verbs = []string{"ReadCapture"}
	if _, _, err := Boot(context.Background(), log, []Required{newer}); err != nil {
		t.Fatal(err)
	}
	if len(log.events) != 2 || log.events[1].Action != ActionUpdate {
		t.Fatalf("events %+v", log.events)
	}
	if d, _ := deskmanifest.Lookup("app:requp"); len(d.Verbs) != 1 {
		t.Errorf("the update did not narrow the desk: %v", d.Verbs)
	}
	// An image older than the log (a rollback of the build) adds nothing and
	// does not downgrade the installed package.
	if _, _, err := Boot(context.Background(), log, []Required{old}); err != nil || len(log.events) != 2 {
		t.Fatalf("an older image changed the log: %v %d", err, len(log.events))
	}
}

func TestARequiredPackageCannotBeDisabled(t *testing.T) {
	req := []Required{requiredPkg("reqkeep", "1.0.0")}
	t.Cleanup(func() { cleanupAll("app:reqkeep") })
	log := &memLog{}
	if _, _, err := Boot(context.Background(), log, req); err != nil {
		t.Fatal(err)
	}
	log.events = append(log.events, Event{Seq: 500, Action: ActionDisable, Package: "reqkeep"})
	set, refusals, err := Boot(context.Background(), log, req)
	if err != nil {
		t.Fatal(err)
	}
	if !hasCode(refusals, CodeRequired) || set.Versions["reqkeep"] == "" {
		t.Fatalf("the disable took: %+v %v", refusals, set.Versions)
	}
	if _, ok := deskmanifest.Lookup("app:reqkeep"); !ok {
		t.Fatal("the required desk was retired by a disable event")
	}
	// A package that is not required still disables.
	log2 := []Event{install(1, goodManifest("optional")), disable(2, "optional")}
	if set, rs := DeriveRequired(log2, req); len(rs) != 0 || len(set.Desks) != 0 {
		t.Errorf("a non-required package could not be disabled: %+v", rs)
	}
}

func TestParseRequiredRefusesAMalformedImageSet(t *testing.T) {
	good := `{"packages":[{"package":"abc","version":"1.0.0","manifest":{"package":"abc","verbs":["Cast"],"worker_max_bytes":1048576,"owned_object_kinds":["choir.app.abc.note"]}}]}`
	if got, err := parseRequired([]byte(good)); err != nil || len(got) != 1 {
		t.Fatalf("good set: %v %v", got, err)
	}
	bad := map[string]string{
		"wrong manifest": `{"packages":[{"package":"abc","version":"1.0.0","manifest":{"package":"abd","verbs":[],"worker_max_bytes":1048576}}]}`,
		"bad version":    `{"packages":[{"package":"abc","version":"1.0","manifest":{"package":"abc","verbs":[],"worker_max_bytes":1048576}}]}`,
		"unknown field":  `{"packages":[],"extra":1}`,
		"duplicate":      `{"packages":[{"package":"abc","version":"1.0.0","manifest":{"package":"abc","verbs":[],"worker_max_bytes":1048576}},{"package":"abc","version":"1.0.0","manifest":{"package":"abc","verbs":[],"worker_max_bytes":1048576}}]}`,
		"invalid desk":   `{"packages":[{"package":"abc","version":"1.0.0","manifest":{"package":"abc","verbs":["Exec"],"worker_max_bytes":1048576}}]}`,
	}
	for name, raw := range bad {
		if _, err := parseRequired([]byte(raw)); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	if _, err := ImageRequired(); err != nil {
		t.Fatalf("the image's own required set does not parse: %v", err)
	}
}

func TestEventCodecRoundTrips(t *testing.T) {
	m := goodManifest("codec")
	in := Event{Action: ActionInstall, Package: "codec", ManifestDigest: Digest(m), Manifest: &m, Version: "2.0.1"}
	body, err := EncodeEvent(in)
	if err != nil {
		t.Fatal(err)
	}
	out, err := DecodeEvent(42, body)
	if err != nil {
		t.Fatal(err)
	}
	if out.Seq != 42 || out.Action != in.Action || out.Version != "2.0.1" || out.Manifest == nil || Digest(*out.Manifest) != in.ManifestDigest {
		t.Fatalf("round trip %+v", out)
	}
	if _, err := DecodeEvent(1, json.RawMessage(`{"action":"install","evil":1}`)); err == nil {
		t.Error("an unknown field was accepted")
	}
}

func TestVersionOrder(t *testing.T) {
	for _, c := range []struct {
		a, b string
		less bool
	}{{"1.0.0", "1.0.1", true}, {"1.9.0", "1.10.0", true}, {"2.0.0", "1.99.99", false}, {"1.0.0", "1.0.0", false}, {"", "0.0.1", true}} {
		if versionLess(c.a, c.b) != c.less {
			t.Errorf("versionLess(%q,%q) != %v", c.a, c.b, c.less)
		}
	}
}
