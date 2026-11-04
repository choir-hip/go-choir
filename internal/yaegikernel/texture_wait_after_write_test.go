package yaegikernel

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/texturewrite"
)

// docs/problems/texture-wait-after-write-loses-intents-2026-11-03.md: a cell
// that ran Cast, Write, Wait with `if err != nil { panic(err) }` lost both
// staged intents, because Wait after Write returned an error and the panic
// rolled the cell back. Ways it can fail, written before the fix:
//   - Wait after Write still errors, so the panic fires and out.Error is set;
//   - the Cast or the Write is missing from the shipped intents;
//   - the Wait overwrites the Write's turn (the revision is lost);
//   - Write after Wait stops being refused (a turn still Writes or Waits).
//
// Since docs/problems/texture-write-leaves-owner-request-unattended-2026-11-04.md
// the Wait after a Write records its why with the revision (it names what the
// open request waits on), and a Done after both supersedes the Wait.
func TestTextureCellCastWriteWaitPanicOnErrKeepsCastAndWrite(t *testing.T) {
	scope, _ := textureTurnScope(t)
	sess, err := NewSession(NewAllowlist(DefaultSafeStdlibPackagesList()...), scope.ChoirExports())
	if err != nil {
		t.Fatal(err)
	}
	defer sess.Close()
	hooks := scope.BindCell()
	doc := &DocSnapshot{DocID: "doc-1", RevisionID: "rev-1", Blocks: []texturewrite.Block{{Alias: "b1", ID: "p1", Kind: "paragraph", Text: "body"}}}
	source := `_, err := choir.Cast("management", "build the Reader", map[string]any{"kind": "execution_request"}, map[string]any{"p_success": 0.8, "weight": 0.3, "settlement": "recipient"})
if err != nil { panic(err) }
_, err = choir.Write("@replace b1\nnew body", "rewrite")
if err != nil { panic(err) }
_, err = choir.Wait("waiting on the build")
if err != nil { panic(err) }`
	out, err := serveCell(sess, SessionFrame{ID: "c1", Source: source, Doc: doc}, nil, &hooks)
	if err != nil || out.Error != "" {
		t.Fatalf("cell failed: %v %s", err, out.Error)
	}
	var cast, turn *StagedIntent
	for i := range out.Intents {
		switch out.Intents[i].Kind {
		case IntentCast:
			cast = &out.Intents[i]
		case IntentTextureTurn:
			turn = &out.Intents[i]
		}
	}
	if len(out.Intents) != 2 || cast == nil || turn == nil {
		t.Fatalf("intents = %+v, want the Cast and the Write's turn", out.Intents)
	}
	var got TextureTurn
	if err := json.Unmarshal([]byte(turn.Body), &got); err != nil {
		t.Fatal(err)
	}
	if got.Write == nil || got.Wait != "waiting on the build" {
		t.Fatalf("turn = %+v, want the Write kept and the Wait's why recorded", got)
	}

	// Write, Wait, Done: the revision answers the request, so Done
	// supersedes the Wait and nothing is left waiting.
	hooks = scope.BindCell()
	source = `_, _ = choir.Write("@replace b1\nnew body", "rewrite")
_, _ = choir.Wait("waiting")
_, err := choir.Done("the revision answers it")
if err != nil { panic(err) }`
	out, err = serveCell(sess, SessionFrame{ID: "c-done", Source: source, Doc: doc}, nil, &hooks)
	if err != nil || out.Error != "" {
		t.Fatalf("write, wait, done = %v %s", err, out.Error)
	}
	got = TextureTurn{}
	for _, in := range out.Intents {
		if in.Kind == IntentTextureTurn {
			if err := json.Unmarshal([]byte(in.Body), &got); err != nil {
				t.Fatal(err)
			}
		}
	}
	if got.Write == nil || got.Done != "the revision answers it" || got.Wait != "" {
		t.Fatalf("turn = %+v, want the Write and Done, no Wait", got)
	}

	// Write after Wait is still refused: a turn either Writes or Waits.
	hooks = scope.BindCell()
	source = `_, _ = choir.Wait("waiting")
_, err := choir.Write("@replace b1\nnew body", "rewrite")
println("write after wait:", err)`
	out, err = serveCell(sess, SessionFrame{ID: "c2", Source: source, Doc: doc}, nil, &hooks)
	if err != nil || out.Error != "" || !strings.Contains(out.Stdout+out.Stderr, "already called Wait") {
		t.Fatalf("write after wait = %v %q %q, want the refusal", err, out.Error, out.Stdout+out.Stderr)
	}
}
