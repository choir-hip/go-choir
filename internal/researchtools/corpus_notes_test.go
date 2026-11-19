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

// The catching-up note, which is not a failure, has its own predicate; both
// not-searched predicates read the start of the note.
func TestCorpusNotesNameACatchingUpIndexAndReadOnlyTheStart(t *testing.T) {
	note := "the page text of 100 of this computer's 900 captures " + CaptureTextCatchingUpMarker + "; the rest are found by title and labels only"
	if !CaptureTextCatchingUp(note) || CapturesNotSearched(note) {
		t.Fatalf("note %q misread", note)
	}
	// A note that merely mentions the words later is not a failure note.
	if CapturesNotSearched("ranked the strongest; "+CapturesNotSearchedNote) || PublishedNotSearched("x; "+PublishedNotSearchedNote) {
		t.Fatal("a predicate matched in the middle of a note")
	}
	if !CapturesNotSearched("  "+CapturesNotSearchedNote+": x") || !PublishedNotSearched("  "+PublishedNotSearchedNote+": x") {
		t.Fatal("a padded failure note was not recognised")
	}
}
