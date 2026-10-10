package agentcore

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/selfdev"
)

// A freeze opens its candidate under an id derived from the assignment, and
// nothing listed those candidates: a harness, the choir CLI or a GUI had to
// re-derive the id to find a proposal awaiting approval. Failure modes pinned:
//   - the list omits settled operations (applied, rejected) the owner needs
//     for rollback and history;
//   - the list leaks another computer's operations;
//   - the state filter is ignored or an unknown state is accepted silently;
//   - the collection still answers GET with 405.
func TestSelfDevelopmentOperationsListsEveryStateForThisComputer(t *testing.T) {
	ctx := context.Background()
	runtime, productStore := testRuntime(t)
	computerID := "computer-selfdev-list"
	runtime.cfg.ComputerID = computerID
	operations, err := selfdev.NewStore(productStore, productStore)
	if err != nil {
		t.Fatal(err)
	}
	runtime.selfdevOperations = operations
	now := time.Now().UTC().Truncate(time.Microsecond)
	insert := func(operationID, owner, state string, at time.Time) {
		t.Helper()
		if _, err := productStore.DB().ExecContext(ctx, `INSERT INTO self_development_operations (operation_id,computer_id,idempotency_key,request_commitment,trajectory_id,base_head,prompt_artifact_ref,verifier_refs_json,desired_head,effective_head,state,created_at,updated_at) VALUES (?,?,?,?,?,?,?,'[]',?,?,?, ?,?)`,
			operationID, owner, "key-"+operationID, strings.Repeat("d", 64), "trajectory-"+operationID, strings.Repeat("a", 64),
			"artifact:sha256:"+strings.Repeat("b", 64), strings.Repeat("c", 64), strings.Repeat("c", 64), state, at, at); err != nil {
			t.Fatal(err)
		}
	}
	insert("selfdev-applied", computerID, selfdev.StateApplied, now.Add(-3*time.Minute))
	insert("selfdev-rejected", computerID, selfdev.StateRejected, now.Add(-2*time.Minute))
	insert("selfdev-awaiting", computerID, selfdev.StateAwaitingApproval, now.Add(-time.Minute))
	insert("selfdev-elsewhere", "computer-other", selfdev.StateAwaitingApproval, now)

	handler := &APIHandler{rt: runtime}
	list := func(query string) (int, []selfdev.Operation, string) {
		request := httptest.NewRequest(http.MethodGet, "/api/computers/"+computerID+"/self-development/operations"+query, nil)
		request.Header.Set("X-Authenticated-User", "owner")
		request.Header.Set("X-Authenticated-Computer", computerID)
		response := httptest.NewRecorder()
		handler.HandleComputersRouter(response, request)
		var body struct {
			Operations []selfdev.Operation `json:"operations"`
		}
		_ = json.Unmarshal(response.Body.Bytes(), &body)
		return response.Code, body.Operations, response.Body.String()
	}
	ids := func(ops []selfdev.Operation) []string {
		out := make([]string, 0, len(ops))
		for _, op := range ops {
			out = append(out, op.OperationID)
		}
		return out
	}

	code, all, raw := list("")
	if code != http.StatusOK {
		t.Fatalf("list status=%d body=%s", code, raw)
	}
	if got := strings.Join(ids(all), ","); got != "selfdev-awaiting,selfdev-rejected,selfdev-applied" {
		t.Fatalf("list = %s, want this computer's operations newest first across every state", got)
	}
	code, awaiting, raw := list("?state=" + selfdev.StateAwaitingApproval)
	if code != http.StatusOK || strings.Join(ids(awaiting), ",") != "selfdev-awaiting" {
		t.Fatalf("filtered list status=%d ids=%v body=%s", code, ids(awaiting), raw)
	}
	if code, _, raw := list("?state=pending"); code != http.StatusBadRequest {
		t.Fatalf("unknown state status=%d body=%s, want 400", code, raw)
	}
}
