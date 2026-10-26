package yaegikernel

import (
	"encoding/json"
	"strings"
	"testing"
)

// A Texture cell names the owner's revision instead of retyping it
// (docs/problems/texture-retypes-owner-directions-2026-10-25.md, chosen
// fix). Ways it can fail, written before the code:
//   - the frame's owner revision does not reach the worker (the field is
//     dropped by the frame's JSON) or does not reach choir.ReadDoc();
//   - a cell cannot read it through the interpreter;
//   - a Cast that names it stages a different id, or stages the owner's
//     text the cell would have typed;
//   - the surface does not teach it, so Texture keeps retyping;
//   - it survives into the next cell's frame.
func TestTextureCellNamesTheOwnerRevisionFromItsFrame(t *testing.T) {
	var frame SessionFrame
	if err := json.Unmarshal([]byte(`{"id":"c1","doc":{"doc_id":"doc-1","revision_id":"rev-owner","author_kind":"user","owner_revision":"rev-owner"}}`), &frame); err != nil {
		t.Fatal(err)
	}
	if frame.Doc == nil || frame.Doc.OwnerRevision != "rev-owner" {
		t.Fatalf("frame doc = %+v, want owner_revision rev-owner", frame.Doc)
	}

	scope, _ := textureTurnScope(t)
	sess, err := NewSession(NewAllowlist(DefaultSafeStdlibPackagesList()...), scope.ChoirExports())
	if err != nil {
		t.Fatal(err)
	}
	defer sess.Close()
	hooks := scope.BindCell()
	frame.Source = `id := choir.ReadDoc().OwnerRevision
println("owner revision", id)
_, err := choir.Cast("management", "build the Reader", map[string]any{"kind": "execution_request", "owner_revision": id})
println(err)`
	out, err := serveCell(sess, frame, nil, &hooks)
	if err != nil || out.Error != "" {
		t.Fatalf("cell failed: %v %s", err, out.Error)
	}
	if !strings.Contains(out.Stdout+out.Stderr, "owner revision rev-owner") {
		t.Fatalf("cell output = %q %q, want the frame's owner revision", out.Stdout, out.Stderr)
	}
	if len(out.Intents) != 1 || out.Intents[0].Kind != IntentCast {
		t.Fatalf("intents = %+v, want one cast", out.Intents)
	}
	var spec map[string]any
	if err := json.Unmarshal([]byte(out.Intents[0].Statement), &spec); err != nil {
		t.Fatal(err)
	}
	if spec["owner_revision"] != "rev-owner" || spec["owner_directions"] != nil {
		t.Fatalf("staged spec = %v, want owner_revision rev-owner and no owner text", spec)
	}

	// The next cell's frame names no owner revision: none carries over.
	hooks = scope.BindCell()
	out, err = serveCell(sess, SessionFrame{ID: "c2", Doc: &DocSnapshot{DocID: "doc-1", RevisionID: "rev-texture"}, Source: `println("owner revision", choir.ReadDoc().OwnerRevision == "")`}, nil, &hooks)
	if err != nil || out.Error != "" || !strings.Contains(out.Stdout+out.Stderr, "owner revision true") {
		t.Fatalf("second cell = %v %q %q, want no owner revision", err, out.Error, out.Stdout+out.Stderr)
	}

	if doc := (&ChoirScope{desk: "texture"}).Help("ReadDoc"); !strings.Contains(doc, "OwnerRevision") || !strings.Contains(doc, "owner_revision") {
		t.Fatalf("ReadDoc's help does not teach naming the owner revision:\n%s", doc)
	}
}
