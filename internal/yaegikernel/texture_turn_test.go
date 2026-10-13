package yaegikernel

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/texturewrite"
)

// Texture's turn verbs (Cut 2 2a). Ways they can fail, written first:
//   - a Write with a bad alias or an unknown frame handle stages anyway and
//     the refusal only arrives after the cell (it must return now, with the
//     script line);
//   - two Writes, or a Write and a Wait, stage two turns the reducer cannot
//     order;
//   - Done, Wait and Reject stage separate intents instead of one turn;
//   - Reject accepts a record the frame never showed;
//   - ApplyTexture is still callable on Texture.
func textureTurnScope(t *testing.T) (*ChoirScope, CellHooks) {
	t.Helper()
	broker, issuer, _, _ := testChoirFixture(t)
	scope, err := NewChoirScope(broker, issuer, "computer-choir", "activation-texture", 1, "texture", "")
	if err != nil {
		t.Fatal(err)
	}
	hooks := scope.BindCell()
	hooks.Begin(SessionFrame{
		Doc: &DocSnapshot{DocID: "doc-1", RevisionID: "rev-1", Blocks: []texturewrite.Block{
			{Alias: "b1", ID: "h1", Kind: "heading", Text: "# Title"},
			{Alias: "b2", ID: "p1", Kind: "paragraph", Text: "Body."},
		}, Sources: []texturewrite.Source{{Handle: "r1.c1", URL: "https://example.org"}}},
		Records: []InboxRecord{{Seq: 4, RecordID: "rec-4", Kind: "report", From: "research:doc-1"}},
		Updates: []PendingUpdate{{UpdateID: "upd-1"}},
	})
	return scope, hooks
}

func TestWriteRefusesSynchronouslyWithTheLine(t *testing.T) {
	scope, hooks := textureTurnScope(t)
	if _, err := scope.Write("@replace b2\nfine\n\n@delete b3", "x"); err == nil || !strings.Contains(err.Error(), "line 4") || !strings.Contains(err.Error(), "b3") {
		t.Fatalf("bad alias = %v", err)
	}
	if _, err := scope.Write("@insert end\nA {{cite r2.c1 \"q\"}}", "x"); err == nil || !strings.Contains(err.Error(), "line 2") || !strings.Contains(err.Error(), "r2.c1") {
		t.Fatalf("unknown frame handle = %v", err)
	}
	if _, err := scope.Write("@replace b2\nfine", ""); err == nil {
		t.Fatal("a Write without why staged")
	}
	if got := hooks.End(); len(got) != 0 {
		t.Fatalf("refused Writes staged %d intents", len(got))
	}
}

func TestTurnVerbsBuildOneTurn(t *testing.T) {
	scope, hooks := textureTurnScope(t)
	if _, err := scope.Write("@insert end\nA {{cite r1.c1 \"q\"}}.", "add a claim"); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if _, err := scope.Write("@delete b1", "again"); err == nil {
		t.Fatal("a second Write staged")
	}
	if _, err := scope.Wait("waiting"); err == nil {
		t.Fatal("Wait staged after Write")
	}
	if _, err := scope.Done("the request is answered"); err != nil {
		t.Fatalf("Done: %v", err)
	}
	if _, err := scope.Reject("r1", "off topic"); err != nil {
		t.Fatalf("Reject record: %v", err)
	}
	if _, err := scope.Reject("upd-1", "superseded"); err != nil {
		t.Fatalf("Reject update: %v", err)
	}
	if _, err := scope.Reject("r9", "x"); err == nil {
		t.Fatal("Reject of a record the frame never showed staged")
	}
	intents := hooks.End()
	if len(intents) != 1 || intents[0].Kind != IntentTextureTurn {
		t.Fatalf("intents = %+v, want one texture_turn", intents)
	}
	var turn TextureTurn
	if err := json.Unmarshal([]byte(intents[0].Body), &turn); err != nil {
		t.Fatal(err)
	}
	if turn.BaseRevisionID != "rev-1" || turn.Write == nil || turn.Write.Why != "add a claim" || turn.Done == "" || len(turn.Rejects) != 2 ||
		turn.Rejects[0].RecordID != "rec-4" || turn.Rejects[1].UpdateID != "upd-1" {
		t.Fatalf("turn = %+v", turn)
	}
}

func TestWaitExcludesDone(t *testing.T) {
	scope, hooks := textureTurnScope(t)
	if _, err := scope.Wait("waiting on research"); err != nil {
		t.Fatalf("Wait: %v", err)
	}
	if _, err := scope.Done("x"); err == nil {
		t.Fatal("Done staged after Wait")
	}
	if _, err := scope.Write("@delete b1", "x"); err == nil {
		t.Fatal("Write staged after Wait")
	}
	if got := hooks.End(); len(got) != 1 {
		t.Fatalf("intents = %+v", got)
	}
}

func TestApplyTextureIsGoneFromTexture(t *testing.T) {
	texture := &ChoirScope{desk: "texture"}
	exports := texture.ChoirExports()["choir/choir"]
	if _, ok := exports["ApplyTexture"]; ok {
		t.Fatal("Texture still exports ApplyTexture")
	}
	for _, name := range []string{"Write", "Done", "Wait", "Reject", "ReadDoc", "Cast", "Ask", "Note"} {
		if _, ok := exports[name]; !ok {
			t.Errorf("Texture does not export %s", name)
		}
	}
	if _, ok := choirFunctionDocs["ApplyTexture"]; ok {
		t.Error("ApplyTexture is still documented")
	}
}
