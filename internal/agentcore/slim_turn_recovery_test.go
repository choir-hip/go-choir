package agentcore

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// nameRevisionInCommand rewrites one lifecycle command's stored result in the
// store's own tables into the slim form: the embedded revision removed and
// revision_id set to namedID (an empty namedID keeps the embedded revision's
// own id). It is the form a writer that names its revision stores.
func nameRevisionInCommand(t *testing.T, s *store.Store, commandID, namedID string) {
	t.Helper()
	rows, err := s.DB().Query(`SELECT o.obj_id, b.body FROM og_objects o JOIN og_bodies b ON b.obj_id = o.obj_id WHERE o.object_kind = 'choir.lifecycle_command'`)
	if err != nil {
		t.Fatal(err)
	}
	type row struct {
		id   int64
		body []byte
	}
	var match *row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.id, &r.body); err != nil {
			t.Fatal(err)
		}
		var head struct {
			CommandID string `json:"command_id"`
		}
		if json.Unmarshal(r.body, &head) == nil && head.CommandID == commandID {
			cp := r
			match = &cp
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	rows.Close()
	if match == nil {
		t.Fatalf("command %s not found", commandID)
	}
	var receipt map[string]json.RawMessage
	var stored map[string]json.RawMessage
	if err := json.Unmarshal(match.body, &receipt); err != nil || json.Unmarshal(receipt["stored_result"], &stored) != nil {
		t.Fatalf("decode command %s: %v", commandID, err)
	}
	if _, already := stored["revision_id"]; already && namedID == "" {
		return // a writer that names its revision stored this command slim already
	}
	if namedID == "" {
		var rev types.Revision
		if err := json.Unmarshal(stored["revision"], &rev); err != nil {
			t.Fatal(err)
		}
		namedID = rev.RevisionID
	}
	delete(stored, "revision")
	stored["revision_id"], _ = json.Marshal(namedID)
	receipt["stored_result"], _ = json.Marshal(stored)
	body, _ := json.Marshal(receipt)
	if bytes.Equal(body, match.body) {
		t.Fatal("rewrite changed nothing")
	}
	if _, err := s.DB().Exec(`UPDATE og_bodies SET body = ? WHERE obj_id = ?`, body, match.id); err != nil {
		t.Fatal(err)
	}
}

func slimTurnFixture(t *testing.T, tag string) (*Runtime, *store.Store, *types.RunRecord, seededSupport) {
	t.Helper()
	rt, s := testRuntime(t)
	owner, computer, docID := "owner-slim-"+tag, rt.cfg.ComputerID, "doc-slim-"+tag
	seeded := seedListedSupportPairs(t, s, owner, computer, docID)
	rec := &types.RunRecord{
		RunID: seeded.RunID, OwnerID: owner, ComputerID: computer, TrajectoryID: "trajectory-" + docID,
		AgentID: agentprofile.Texture + ":" + docID, AgentProfile: agentprofile.Texture, AgentRole: agentprofile.Texture,
		Metadata: map[string]any{"doc_id": docID, "lifecycle_work_item_id": "work-" + docID},
	}
	if err := s.CreateAgentMutation(context.Background(), store.AgentMutation{DocID: docID, RunID: rec.RunID, OwnerID: owner, ComputerID: computer, State: "pending", CreatedAt: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}
	nameRevisionInCommand(t, s, "texture-turn-"+docID, "")
	return rt, s, rec, seeded
}

func runRecoveryPath(rt *Runtime, path string, rec *types.RunRecord) error {
	if path == "run completion" {
		return rt.handleRunCompletion(context.Background(), rec)
	}
	return rt.sleepTextureMutationAfterIdle(context.Background(), rec)
}

// A Texture turn whose command names its revision by id is recovered by the
// run-completion and idle-passivation paths, which read it on a background
// context under the enforced reader gate. The mutation completes with the
// turn's revision; it is not marked stale.
func TestSlimTextureTurnCompletesItsMutationUnderEnforce(t *testing.T) {
	for _, path := range []string{"run completion", "idle passivation"} {
		t.Run(path, func(t *testing.T) {
			rt, s, rec, seeded := slimTurnFixture(t, path[:3])
			defer objectgraph.SetModeForTest(objectgraph.WriterGateEnforce)()
			if err := runRecoveryPath(rt, path, rec); err != nil {
				t.Fatalf("%s: %v", path, err)
			}
			mutation, err := s.GetAgentMutationByRun(context.Background(), rec.OwnerID, rec.ComputerID, rec.RunID)
			if err != nil || mutation == nil || mutation.State != "completed" || mutation.RevisionID != seeded.RevisionID {
				t.Fatalf("%s: mutation = %+v, %v; want completed with revision %s", path, mutation, err, seeded.RevisionID)
			}
		})
	}
}

// A real read failure of the committed turn (the command names a revision
// that is not there) is not "no turn committed": the mutation stays pending
// for the next recovery, and is never marked stale.
func TestFailedTurnReadLeavesTheMutationPendingNotStale(t *testing.T) {
	for _, path := range []string{"run completion", "idle passivation"} {
		t.Run(path, func(t *testing.T) {
			rt, s, rec, _ := slimTurnFixture(t, "bad"+path[:3])
			nameRevisionInCommand(t, s, "texture-turn-doc-slim-bad"+path[:3], "no-such-revision")
			if err := runRecoveryPath(rt, path, rec); err != nil {
				t.Fatalf("%s: %v", path, err)
			}
			mutation, err := s.GetAgentMutationByRun(context.Background(), rec.OwnerID, rec.ComputerID, rec.RunID)
			if err != nil || mutation == nil || mutation.State != "pending" {
				t.Fatalf("%s: mutation = %+v, %v; want pending (not stale)", path, mutation, err)
			}
		})
	}
}

// A command that names a revision the store cannot read is a server invariant
// break, not a client error: the API answers 500 and names it.
func TestLifecycleAPIErrorMapsAnUnavailableStoredRevisionTo500(t *testing.T) {
	err := fmt.Errorf("%w: revision %q named by command %q: gone", store.ErrLifecycleStoredRevisionUnavailable, "rev-x", "cmd-x")
	rec := httptest.NewRecorder()
	writeLifecycleAPIError(rec, err)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500; body %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "stored revision unavailable") {
		t.Fatalf("body %s does not name the broken invariant", rec.Body.String())
	}
	// A plain not-found stays a 404 and an invalid transition stays a client error.
	rec = httptest.NewRecorder()
	writeLifecycleAPIError(rec, store.ErrNotFound)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("not found status = %d", rec.Code)
	}
}
