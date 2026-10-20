package textureowner

// The proxy's story expansion request decides "already asked" by looking for
// the owner revision on the paper's chain, and it computes that revision's id
// itself (internal/proxy/platform_expand.go, expandOwnerRevisionID) from the
// owner, computer, trajectory and client_request_id. If this handler ever
// derives the id differently, the proxy's durable check silently stops
// matching (it still falls back to the paper's digest conflict). This test
// pins the formula where the handler lives.

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestOwnerRevisionIDIsTheHashTheProxyComputes(t *testing.T) {
	core, handler := testAPISetup(t)
	start := startObservationLifecycle(t, core.Store())
	response := postOwnerInstruction(t, handler, "/api/texture/documents/"+start.InitialDocument.DocID+"/revise", start.OwnerID, "expand:pub-1:rev-3", "more research", start.InitialRevision.RevisionID)
	if response.Code != http.StatusAccepted {
		t.Fatalf("revise status=%d body=%s", response.Code, response.Body.String())
	}
	var receipt textureOwnerRevisionResponse
	if err := json.NewDecoder(response.Body).Decode(&receipt); err != nil {
		t.Fatal(err)
	}
	want := uuid.NewSHA1(uuid.NameSpaceOID, []byte(strings.Join([]string{start.OwnerID, start.ComputerID, start.TrajectoryID, "expand:pub-1:rev-3"}, "\x00"))).String()
	if receipt.RevisionID != want {
		t.Fatalf("owner revision id = %s, want %s: update internal/proxy expandOwnerRevisionID with it", receipt.RevisionID, want)
	}
}
