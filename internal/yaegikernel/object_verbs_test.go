package yaegikernel

import (
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/deskmanifest"
)

// MA3.8: the object verbs stage intents that name a kind and an id and never a
// writer; the tray bounds an object body at MaxObjectBody, larger than a
// message body, and refuses an empty or non-JSON one.
func TestObjectVerbsStageIntentsWithoutAWriter(t *testing.T) {
	tr := &Tray{}
	if _, err := tr.ObjectPut("choir.app.x.note", "n1", `{"a":1}`); err != nil {
		t.Fatal(err)
	}
	if _, err := tr.ObjectRetract("choir.app.x.note", "n1"); err != nil {
		t.Fatal(err)
	}
	got := tr.Drain()
	if len(got) != 2 || got[0].Kind != IntentObjectPut || got[0].ObjectKind != "choir.app.x.note" || got[0].ObjectID != "n1" || got[1].Kind != IntentObjectRetract {
		t.Fatalf("intents = %+v", got)
	}
	if _, err := tr.ObjectPut("", "n1", `{}`); err == nil {
		t.Fatal("a put without a kind was staged")
	}
	if _, err := tr.ObjectPut("k", "n1", `not json`); err == nil {
		t.Fatal("a put with a non-JSON body was staged")
	}
	big := `{"x":"` + strings.Repeat("a", MaxIntentBody+1) + `"}`
	if _, err := tr.ObjectPut("k", "n1", big); err != nil {
		t.Fatalf("an object body past the message bound but within MaxObjectBody: %v", err)
	}
	huge := `{"x":"` + strings.Repeat("a", MaxObjectBody) + `"}`
	if _, err := tr.ObjectPut("k", "n2", huge); err == nil {
		t.Fatal("an oversize object body was staged")
	}
}

// Only an app desk holds the object verbs; a built-in desk's module set has
// none of them.
func TestOnlyAppDesksHoldTheObjectVerbs(t *testing.T) {
	for _, name := range deskmanifest.Names() {
		d, _ := deskmanifest.Lookup(name)
		if d.Kind == deskmanifest.KindApp {
			continue
		}
		for _, v := range []string{"Put", "Get", "List", "Retract"} {
			if d.HasVerb(v) {
				t.Errorf("built-in desk %s holds %s", name, v)
			}
		}
	}
	if _, _, ok := deskmanifest.VerbAction("Get"); !ok {
		t.Fatal("Get dispatches no action")
	}
}
