package yaegikernel

import (
	"context"
	"testing"
)

// docs/problems/prompt-bar-minesweeper-demo-2026-10-10.md (F2): after a
// Texture cell staged an ApplyTexture whose reduce failed, later cells that
// staged nothing (println("ping ok"), choir.Help) failed with the same
// "persist tray-1" error. Failure modes pinned: a cell that stages nothing
// ships an intent; a staged intent survives into the next cell; the intent
// staged inside an immediately invoked closure is shipped twice.
func TestCellAfterAStagingCellShipsOnlyItsOwnIntents(t *testing.T) {
	broker, issuer, _, _ := testChoirFixture(t)
	scope, err := NewChoirScope(broker, issuer, "computer-choir", "activation-texture", 1, "texture", "")
	if err != nil {
		t.Fatal(err)
	}
	sess, err := NewSession(NewAllowlist("choir"), scope.ChoirExports())
	if err != nil {
		t.Fatal(err)
	}
	defer sess.Close()
	if _, err := sess.Eval(context.Background(), `import "choir"`); err != nil {
		t.Fatalf("import choir: %v", err)
	}
	hooks := scope.BindCell()
	cells := []struct {
		source string
		want   int
	}{
		{`func() {
	doc := "body"
	res, err := choir.ApplyTexture(map[string]interface{}{"op": "apply", "base_revision_id": "rev-1", "content": doc})
	println("apply:", res)
	println("err:", err)
}()`, 1},
		{`println("ping ok")`, 0},
		{`println(choir.Help("ApplyTexture"))`, 0},
		{`res, err := choir.ApplyTexture(map[string]interface{}{"op": "apply", "base_revision_id": "rev-1", "content": "top"})
println(res, err)`, 1},
		{`println("ping again")`, 0},
		{`// a leading comment does not change the first token yaegi sees
func() {
	choir.ApplyTexture(map[string]interface{}{"op": "apply", "base_revision_id": "rev-1", "content": "commented"})
}()`, 1},
		{`println("after the commented closure")`, 0},
	}
	for index, cell := range cells {
		out, err := serveCell(sess, SessionFrame{ID: "cell", Source: cell.source}, nil, &hooks)
		if err != nil || out.Error != "" {
			t.Fatalf("cell %d failed: %v %s", index, err, out.Error)
		}
		if len(out.Intents) != cell.want {
			t.Fatalf("cell %d shipped %d intents %+v, want %d (stdout %q)", index, len(out.Intents), out.Intents, cell.want, out.Stdout)
		}
	}
}
