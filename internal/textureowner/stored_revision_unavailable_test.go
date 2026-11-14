package textureowner

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/store"
)

// breakNamedRevisions points every slim lifecycle command (one that names its
// revision by id) at a revision that does not exist, so the next replay of it
// cannot read the revision it names.
func breakNamedRevisions(t *testing.T, s *store.Store) int {
	return renameNamedRevisions(t, s, "no-such-revision", "")
}

// renameNamedRevisions points every slim command whose command id contains
// only (all when empty) at the revision named to.
func renameNamedRevisions(t *testing.T, s *store.Store, to, only string) int {
	t.Helper()
	rows, err := s.DB().Query(`SELECT o.obj_id, b.body FROM og_objects o JOIN og_bodies b ON b.obj_id = o.obj_id WHERE o.object_kind = 'choir.lifecycle_command'`)
	if err != nil {
		t.Fatal(err)
	}
	type row struct {
		id   int64
		body []byte
	}
	var found []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.id, &r.body); err != nil {
			t.Fatal(err)
		}
		found = append(found, r)
	}
	rows.Close()
	broken := 0
	for _, r := range found {
		var receipt, stored map[string]json.RawMessage
		if json.Unmarshal(r.body, &receipt) != nil || json.Unmarshal(receipt["stored_result"], &stored) != nil {
			continue
		}
		if _, slim := stored["revision_id"]; !slim {
			continue
		}
		if only != "" && !strings.Contains(string(receipt["command_id"]), only) {
			continue
		}
		stored["revision_id"], _ = json.Marshal(to)
		receipt["stored_result"], _ = json.Marshal(stored)
		body, _ := json.Marshal(receipt)
		if _, err := s.DB().Exec(`UPDATE og_bodies SET body = ? WHERE obj_id = ?`, body, r.id); err != nil {
			t.Fatal(err)
		}
		broken++
	}
	return broken
}

// Where the store's command replay is reached over HTTP, the answer is 500 and
// names the broken invariant: a command that exists but cannot return its
// revision is a server fault, never a client error. The owner revise replay
// and the prompt-bar replay are answered before the store replay (the revise
// reads its revision by its derived id; the prompt bar replays by run), so a
// broken command cannot reach them on replay; their commit paths are covered by
// the mapping table below.
func TestCreateReplayWithAnUnreadableNamedRevisionAnswers500(t *testing.T) {
	core, handler := testAPISetup(t)
	post := func() *httptest.ResponseRecorder {
		raw, _ := json.Marshal(map[string]string{"client_request_id": "create-broken", "title": "T", "initial_content": "c"})
		req := httptest.NewRequest(http.MethodPost, "/api/texture/lifecycle-documents", strings.NewReader(string(raw)))
		req.Header.Set("X-Authenticated-User", "user-broken")
		w := httptest.NewRecorder()
		handler.HandleTextureRouter(w, req)
		return w
	}
	if first := post(); first.Code != http.StatusCreated {
		t.Fatalf("first create = %d %s", first.Code, first.Body.String())
	}
	if breakNamedRevisions(t, core.Store()) == 0 {
		t.Fatal("no slim command to break")
	}
	got := post()
	if got.Code != http.StatusInternalServerError || !strings.Contains(got.Body.String(), "stored revision unavailable") {
		t.Fatalf("status = %d, want 500 naming the unavailable stored revision; body %s", got.Code, got.Body.String())
	}
}

// Every mapping of a store error above a path that can receive
// ErrLifecycleStoredRevisionUnavailable answers 500 and names it.
func TestEveryMappingOfAStoreErrorTreatsAnUnavailableStoredRevisionAsServerFault(t *testing.T) {
	unavailable := storeUnavailableForTest()
	refusal := func(err error) (int, string) {
		r, ok := err.(*ownerRevisionRefusal)
		if !ok {
			t.Fatalf("not an owner revision refusal: %#v", err)
		}
		return r.status, r.message
	}
	mappings := []struct {
		name string
		got  func() (int, string)
	}{
		{"fork start (revise_fork.go forkStartFailure)", func() (int, string) { return refusal(forkStartFailure(unavailable)) }},
		{"owner revise and edit commits (refuseCommit)", func() (int, string) { return refusal(refuseCommit(unavailable, "failed to commit")) }},
		{"prompt bar and ask story submit (promptBarSubmitFailure)", func() (int, string) { return promptBarSubmitFailure(unavailable) }},
	}
	for _, m := range mappings {
		status, message := m.got()
		if status != http.StatusInternalServerError || !strings.Contains(message, "stored revision unavailable") {
			t.Errorf("%s = %d %q, want 500 naming it", m.name, status, message)
		}
	}
}

func storeUnavailableForTest() error {
	return fmt.Errorf("%w: revision %q named by command %q: gone", store.ErrLifecycleStoredRevisionUnavailable, "rev", "cmd")
}

// A replayed create whose command names a readable revision that is not the one
// the request derives is not the same create: the handler checks the revision
// the store returns against the derived id and answers 500.
func TestCreateReplayWhoseCommandNamesAnotherRevisionAnswers500(t *testing.T) {
	core, handler := testAPISetup(t)
	post := func(id string) (*httptest.ResponseRecorder, textureLifecycleCreateResponse) {
		raw, _ := json.Marshal(map[string]string{"client_request_id": id, "title": "T", "initial_content": "c"})
		req := httptest.NewRequest(http.MethodPost, "/api/texture/lifecycle-documents", strings.NewReader(string(raw)))
		req.Header.Set("X-Authenticated-User", "user-mismatch")
		w := httptest.NewRecorder()
		handler.HandleTextureRouter(w, req)
		var out textureLifecycleCreateResponse
		_ = json.Unmarshal(w.Body.Bytes(), &out)
		return w, out
	}
	_, a := post("create-a")
	_, b := post("create-b")
	if a.RevisionID == "" || b.RevisionID == "" || a.RevisionID == b.RevisionID {
		t.Fatalf("revisions a=%q b=%q", a.RevisionID, b.RevisionID)
	}
	// Create A's command now names create B's (readable) revision.
	if renameNamedRevisions(t, core.Store(), b.RevisionID, "start:") == 0 {
		t.Fatal("no slim start command")
	}
	got, _ := post("create-a")
	if got.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500; body %s", got.Code, got.Body.String())
	}
}
