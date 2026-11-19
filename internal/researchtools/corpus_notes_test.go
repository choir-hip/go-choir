package researchtools

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
)

// The host reads the verb's notes to tell a half that was not searched from a
// half that found nothing (the research brief). The notes the real tool writes
// must satisfy the exported predicates, so a reworded note breaks this test and
// not the brief silently.
func TestCorpusNotesNameAHalfThatWasNotSearched(t *testing.T) {
	guest, err := store.Open(filepath.Join(t.TempDir(), "runtime.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = guest.Close() })
	tool := newSearchWireCorpusTool(Dependencies{Store: guest}) // no capture store
	ctx := toolregistry.WithExecutionContext(context.Background(), toolregistry.ExecutionContext{OwnerID: "o", ComputerID: "c"})
	out, err := tool.Func(ctx, json.RawMessage(`{"query":"harbor strike"}`))
	if err != nil {
		t.Fatal(err)
	}
	var body struct {
		CapturesNote  string `json:"captures_note"`
		PublishedNote string `json:"published_note"`
	}
	if err := json.Unmarshal([]byte(out), &body); err != nil {
		t.Fatal(err)
	}
	if !CapturesNotSearched(body.CapturesNote) || !PublishedNotSearched(body.PublishedNote) {
		t.Fatalf("notes %q and %q are not recognised as halves not searched", body.CapturesNote, body.PublishedNote)
	}
	for _, note := range []string{"", "searched all 900 captures; ranked the strongest", "published search capped: the archive index is still being built"} {
		if CapturesNotSearched(note) || PublishedNotSearched(note) {
			t.Fatalf("note %q is not a failure", note)
		}
	}
}
