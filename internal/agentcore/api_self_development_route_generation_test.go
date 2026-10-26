package agentcore

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerversion"
	"github.com/yusefmosiah/go-choir/internal/routeledger"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
	"github.com/yusefmosiah/go-choir/internal/vmctl"
)

// The rollback request binds expected_route_generation to the guest's own
// route slot, and no bearer-readable route exposed it: the compute status
// route answers immutable_identity null when its proxy-side join fails, and an
// operation's recorded route_generation goes stale when a platform update
// moves the slot. The operations listing therefore reports the slot's current
// generation, read through the same resolver the rollback check uses.
// Failure modes pinned: the field is absent although the route resolves; it
// is a recorded operation's generation instead of the slot's; a route that
// cannot be resolved fails the listing (the field is omitted instead).
func TestSelfDevelopmentOperationsReportTheRouteSlotGeneration(t *testing.T) {
	ctx := context.Background()
	runtime, productStore := testRuntime(t)
	computerID := "computer-selfdev-route-generation"
	runtime.cfg.ComputerID = computerID
	operations, err := selfdev.NewStore(productStore, productStore)
	if err != nil {
		t.Fatal(err)
	}
	runtime.selfdevOperations = operations

	now := time.Now().UTC()
	digest := strings.Repeat("1", 64)
	closure, err := computerversion.NewCodeClosure(strings.Repeat("1", 40), []computerversion.CodeArtifact{{Name: "bundle", SHA256: digest, URI: "artifact+sha256://" + digest + "/bundle"}}, now)
	if err != nil {
		t.Fatal(err)
	}
	program, err := computerversion.NewArtifactProgram([]computerversion.ArtifactProgramEntry{{Kind: "bundle", ContentSHA256: digest, ArtifactURI: "artifact+sha256://" + digest + "/bundle"}}, now)
	if err != nil {
		t.Fatal(err)
	}
	version := computerversion.ComputerVersion{CodeRef: closure.Ref, ArtifactProgramRef: program.Ref}
	slotID, _ := routeledger.RouteSlotID("owner", "primary")
	ledger := routeledger.NewMemoryLedger()
	approval, _ := routeledger.NewAuthorizationEvidence(routeledger.AuthorizationEvidenceApproval, slotID, version, json.RawMessage(`{"approval":true}`), now)
	certificate, _ := routeledger.NewAuthorizationEvidence(routeledger.AuthorizationEvidencePromotionCertificate, slotID, version, json.RawMessage(`{"certificate":true}`), now)
	slot, receipt, err := ledger.TransitionWithEvidence(ctx, routeledger.TransitionCommand{
		RouteSlotID: slotID, Kind: routeledger.TransitionBootstrap, New: version,
		ApprovalRef: routeledger.ApprovalRef(approval.Ref), PromotionCertificateRef: routeledger.PromotionCertificateRef(certificate.Ref),
		IdempotencyKey: routeledger.IdempotencyKey("idempotency:bootstrap"),
	}, []routeledger.AuthorizationEvidence{approval, certificate})
	if err != nil {
		t.Fatal(err)
	}
	routeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(vmctl.RouteResolution{Slot: slot, LatestReceipt: receipt, CodeClosure: closure, ArtifactProgram: program})
	}))
	t.Cleanup(routeServer.Close)

	list := func(rt *Runtime) (int, map[string]json.RawMessage) {
		request := httptest.NewRequest(http.MethodGet, "/api/computers/"+computerID+"/self-development/operations", nil)
		request.Header.Set("X-Authenticated-User", "owner")
		request.Header.Set("X-Authenticated-Computer", computerID)
		response := httptest.NewRecorder()
		(&APIHandler{rt: rt}).HandleComputersRouter(response, request)
		var body map[string]json.RawMessage
		_ = json.Unmarshal(response.Body.Bytes(), &body)
		return response.Code, body
	}

	runtime.selfdevRoute, runtime.selfdevRouteOwnerID, runtime.selfdevRouteDesktopID = vmctl.NewClient(routeServer.URL), "owner", "primary"
	code, body := list(runtime)
	if code != http.StatusOK || string(body["route_generation"]) != "1" {
		t.Fatalf("status=%d route_generation=%s, want 200 and the slot's generation 1 (%v)", code, body["route_generation"], body)
	}

	// An unresolvable route leaves the listing intact and the field out.
	routeServer.Close()
	code, body = list(runtime)
	if code != http.StatusOK || body["operations"] == nil {
		t.Fatalf("status=%d body=%v, want the listing to survive an unreachable route", code, body)
	}
	if _, present := body["route_generation"]; present {
		t.Fatalf("route_generation reported although the route did not resolve: %s", body["route_generation"])
	}

	// No route authority configured: the same, the existing listing shape.
	runtime.selfdevRoute = nil
	if code, body = list(runtime); code != http.StatusOK {
		t.Fatalf("status=%d, want 200 with no route authority", code)
	} else if _, present := body["route_generation"]; present {
		t.Fatal("route_generation reported with no route authority")
	}
}
